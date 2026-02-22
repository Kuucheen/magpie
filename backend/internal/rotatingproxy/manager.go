package rotatingproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/support"
)

type Manager struct {
	mu      sync.RWMutex
	servers map[uint64]*proxyServer
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[uint64]*proxyServer),
	}
}

var GlobalManager = NewManager()

func (m *Manager) StartAll() {
	m.Reconcile()
}

func (m *Manager) Reconcile() {
	start, end := support.GetRotatingProxyPortRange()
	rotators, err := database.GetAllRotatingProxies()
	if err != nil {
		log.Error("rotating proxy manager: failed to load rotators", "error", err)
		return
	}

	desired := make(map[uint64]domain.RotatingProxy, len(rotators))
	for _, rotator := range rotators {
		if rotator.ListenPort == 0 || int(rotator.ListenPort) < start || int(rotator.ListenPort) > end {
			log.Warn("rotating proxy manager: skipping rotator without valid port", "rotator_id", rotator.ID, "listen_port", rotator.ListenPort)
			continue
		}
		desired[rotator.ID] = rotator
	}

	m.mu.Lock()
	for id, server := range m.servers {
		if _, ok := desired[id]; ok {
			continue
		}
		server.Stop()
		delete(m.servers, id)
		log.Info("rotating proxy server stopped", "rotator_id", id)
	}
	m.mu.Unlock()

	for _, rotator := range desired {
		m.mu.RLock()
		_, alreadyRunning := m.servers[rotator.ID]
		m.mu.RUnlock()
		if alreadyRunning {
			continue
		}
		if err := m.startServer(rotator); err != nil {
			log.Error("rotating proxy manager: failed to start server", "rotator_id", rotator.ID, "port", rotator.ListenPort, "error", err)
		}
	}
}

func (m *Manager) StartSyncLoop(ctx context.Context, interval time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Reconcile()
		}
	}
}

func (m *Manager) startServer(rotator domain.RotatingProxy) error {
	start, end := support.GetRotatingProxyPortRange()
	if rotator.ListenPort == 0 || int(rotator.ListenPort) < start || int(rotator.ListenPort) > end {
		return fmt.Errorf("invalid listen port %d for rotator %d", rotator.ListenPort, rotator.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.servers[rotator.ID]; ok {
		existing.Stop()
		delete(m.servers, rotator.ID)
	}

	server := newProxyServer(rotator)
	if err := server.Start(); err != nil {
		return err
	}

	m.servers[rotator.ID] = server
	log.Info("rotating proxy server started", "rotator_id", rotator.ID, "port", rotator.ListenPort)
	return nil
}

func (m *Manager) Add(rotatorID uint64) error {
	rotator, err := database.GetRotatingProxyByID(rotatorID)
	if err != nil {
		return err
	}
	return m.startServer(*rotator)
}

func (m *Manager) Remove(rotatorID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, ok := m.servers[rotatorID]
	if !ok {
		return
	}
	server.Stop()
	delete(m.servers, rotatorID)
	log.Info("rotating proxy server stopped", "rotator_id", rotatorID)
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, server := range m.servers {
		server.Stop()
		delete(m.servers, id)
	}
}

type proxyServer struct {
	rotator     domain.RotatingProxy
	listener    net.Listener
	httpServer  *http.Server
	http3Server *http3.Server
	closeOnce   sync.Once
}

func newProxyServer(rotator domain.RotatingProxy) *proxyServer {
	return &proxyServer{rotator: rotator}
}

func (ps *proxyServer) Start() error {
	transport := listenTransportProtocolName(ps.rotator)
	switch transport {
	case support.TransportQUIC, support.TransportHTTP3:
		if isSocksProtocol(listenProtocolName(ps.rotator)) {
			return fmt.Errorf("socks rotators require tcp transport")
		}
		return ps.startHTTP3Server(transport)
	default:
		if isSocksProtocol(listenProtocolName(ps.rotator)) {
			return ps.startSocksServer()
		}
		return ps.startHTTPServer()
	}
}

func (ps *proxyServer) startHTTPServer() error {
	address := fmt.Sprintf(":%d", ps.rotator.ListenPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	handler := &proxyHandler{rotator: ps.rotator}
	server := &http.Server{
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
	}

	ps.listener = listener
	ps.httpServer = server

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Error("rotating proxy server: serve error", "rotator_id", ps.rotator.ID, "error", err)
		}
	}()

	return nil
}

func (ps *proxyServer) startSocksServer() error {
	address := fmt.Sprintf(":%d", ps.rotator.ListenPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	handler := newSocksProxyHandler(ps.rotator)
	ps.listener = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Error("rotating proxy server: accept error", "rotator_id", ps.rotator.ID, "error", err)
				continue
			}
			go handler.handle(conn)
		}
	}()

	return nil
}

func (ps *proxyServer) startHTTP3Server(transport string) error {
	address := fmt.Sprintf(":%d", ps.rotator.ListenPort)
	tlsConfig, err := rotatorTLSConfig()
	if err != nil {
		return err
	}

	enableDatagrams := transport == support.TransportQUIC
	handler := &proxyHandler{rotator: ps.rotator}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			http.Error(w, "CONNECT is not supported for HTTP/3 rotators", http.StatusMethodNotAllowed)
			return
		}
		handler.ServeHTTP(w, r)
	})

	server := &http3.Server{
		Addr:            address,
		Handler:         httpHandler,
		TLSConfig:       tlsConfig,
		QUICConfig:      &quic.Config{EnableDatagrams: enableDatagrams},
		EnableDatagrams: enableDatagrams,
		IdleTimeout:     30 * time.Second,
		MaxHeaderBytes:  http.DefaultMaxHeaderBytes,
	}

	ps.http3Server = server

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("rotating proxy server: http3 serve error", "rotator_id", ps.rotator.ID, "error", err)
		}
	}()

	return nil
}

func (ps *proxyServer) Stop() {
	ps.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if ps.httpServer != nil {
			if err := ps.httpServer.Shutdown(ctx); err != nil {
				log.Error("rotating proxy server shutdown", "rotator_id", ps.rotator.ID, "error", err)
			}
		}
		if ps.http3Server != nil {
			if err := ps.http3Server.Close(); err != nil {
				log.Error("rotating proxy server http3 close", "rotator_id", ps.rotator.ID, "error", err)
			}
		}
		if ps.listener != nil {
			_ = ps.listener.Close()
		}
	})
}

func isSocksProtocol(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "socks4", "socks5":
		return true
	default:
		return false
	}
}

func listenProtocolName(rotator domain.RotatingProxy) string {
	if name := strings.TrimSpace(rotator.ListenProtocol.Name); name != "" {
		return name
	}
	return strings.TrimSpace(rotator.Protocol.Name)
}

func listenTransportProtocolName(rotator domain.RotatingProxy) string {
	if name := strings.TrimSpace(rotator.ListenTransportProtocol); name != "" {
		return support.NormalizeTransportProtocol(name)
	}
	if name := strings.TrimSpace(rotator.TransportProtocol); name != "" {
		return support.NormalizeTransportProtocol(name)
	}
	return support.TransportTCP
}
