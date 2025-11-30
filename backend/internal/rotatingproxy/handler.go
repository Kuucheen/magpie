package rotatingproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"magpie/internal/api/dto"
	"magpie/internal/database"
	"magpie/internal/domain"
)

const (
	connectEstablishedResponse = "HTTP/1.1 200 Connection Established\r\nProxy-Agent: Magpie Rotator\r\n\r\n"
)

var (
	getNextRotatingProxyFunc   = database.GetNextRotatingProxy
	dialUpstreamFunc           = dialUpstream
	performUpstreamConnectFunc = performUpstreamConnect
	connectThroughUpstreamFunc = connectThroughUpstream
)

type proxyHandler struct {
	rotator domain.RotatingProxy
}

type socksProxyHandler struct {
	rotator domain.RotatingProxy
}

func newSocksProxyHandler(rotator domain.RotatingProxy) *socksProxyHandler {
	return &socksProxyHandler{rotator: rotator}
}

func (h *socksProxyHandler) handle(conn net.Conn) {
	switch strings.ToLower(strings.TrimSpace(h.rotator.Protocol.Name)) {
	case "socks4":
		h.handleSocks4(conn)
	default:
		h.handleSocks5(conn)
	}
}

func (h *socksProxyHandler) handleSocks5(conn net.Conn) {
	defer conn.Close()

	target, err := h.performSocks5Handshake(conn)
	if err != nil {
		return
	}

	next, err := getNextRotatingProxyFunc(h.rotator.UserID, h.rotator.ID)
	if err != nil {
		_ = writeSocks5Reply(conn, 0x01)
		return
	}

	if !supportedUpstream(next.Protocol) {
		_ = writeSocks5Reply(conn, 0x07)
		return
	}

	upstreamConn, err := connectThroughUpstreamFunc(target, next)
	if err != nil {
		_ = writeSocks5Reply(conn, 0x05)
		return
	}

	if err := writeSocks5Success(conn, upstreamConn.LocalAddr()); err != nil {
		_ = upstreamConn.Close()
		return
	}

	pipeConnections(conn, upstreamConn)
}

func (h *socksProxyHandler) performSocks5Handshake(conn net.Conn) (string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	if header[0] != 0x05 {
		_ = writeSocks5Reply(conn, 0x01)
		return "", errors.New("unsupported socks version")
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", err
	}

	selected := byte(0xff)
	if h.rotator.AuthRequired {
		for _, method := range methods {
			if method == 0x02 {
				selected = 0x02
				break
			}
		}
	} else {
		for _, method := range methods {
			if method == 0x00 {
				selected = 0x00
				break
			}
		}
		if selected == 0xff {
			for _, method := range methods {
				if method == 0x02 {
					selected = 0x02
					break
				}
			}
		}
	}

	if selected == 0xff {
		_, _ = conn.Write([]byte{0x05, 0xff})
		return "", errors.New("no acceptable authentication methods")
	}

	if _, err := conn.Write([]byte{0x05, selected}); err != nil {
		return "", err
	}

	if selected == 0x02 {
		if err := h.verifySocks5Credentials(conn); err != nil {
			return "", err
		}
	}

	target, err := readSocks5Target(conn)
	if err != nil {
		return "", err
	}

	return target, nil
}

func (h *socksProxyHandler) verifySocks5Credentials(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != 0x01 {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return errors.New("invalid socks5 authentication version")
	}

	usernameLen := int(header[1])
	username := make([]byte, usernameLen)
	if _, err := io.ReadFull(conn, username); err != nil {
		return err
	}

	passLen := make([]byte, 1)
	if _, err := io.ReadFull(conn, passLen); err != nil {
		return err
	}

	password := make([]byte, int(passLen[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		return err
	}

	if h.rotator.AuthRequired && (string(username) != h.rotator.AuthUsername || string(password) != h.rotator.AuthPassword) {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return errors.New("invalid socks5 credentials")
	}

	_, err := conn.Write([]byte{0x01, 0x00})
	return err
}

func (h *socksProxyHandler) handleSocks4(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return
	}

	if header[0] != 0x04 || header[1] != 0x01 {
		_ = writeSocks4Response(conn, 0x5B, header[2:4], header[4:8])
		return
	}

	dstPort := header[2:4]
	dstIP := header[4:8]

	userID, err := reader.ReadString('\x00')
	if err != nil {
		_ = writeSocks4Response(conn, 0x5B, dstPort, dstIP)
		return
	}
	userID = strings.TrimSuffix(userID, "\x00")

	targetHost := net.IP(dstIP).String()
	if dstIP[0] == 0 && dstIP[1] == 0 && dstIP[2] == 0 && dstIP[3] != 0 {
		domain, err := reader.ReadString('\x00')
		if err != nil {
			_ = writeSocks4Response(conn, 0x5B, dstPort, dstIP)
			return
		}
		targetHost = strings.TrimSuffix(domain, "\x00")
	}

	if h.rotator.AuthRequired {
		expected := h.rotator.AuthUsername
		if h.rotator.AuthPassword != "" {
			expected = fmt.Sprintf("%s:%s", h.rotator.AuthUsername, h.rotator.AuthPassword)
		}
		if userID != expected {
			_ = writeSocks4Response(conn, 0x5B, dstPort, dstIP)
			return
		}
	}

	port := binary.BigEndian.Uint16(dstPort)
	target := net.JoinHostPort(targetHost, strconv.Itoa(int(port)))

	next, err := getNextRotatingProxyFunc(h.rotator.UserID, h.rotator.ID)
	if err != nil || !supportedUpstream(next.Protocol) {
		_ = writeSocks4Response(conn, 0x5B, dstPort, dstIP)
		return
	}

	upstreamConn, err := connectThroughUpstreamFunc(target, next)
	if err != nil {
		_ = writeSocks4Response(conn, 0x5B, dstPort, dstIP)
		return
	}

	if err := writeSocks4Response(conn, 0x5A, dstPort, dstIP); err != nil {
		_ = upstreamConn.Close()
		return
	}

	pipeConnections(conn, upstreamConn)
}
func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateClient(w, r) {
		return
	}

	switch strings.ToUpper(r.Method) {
	case http.MethodConnect:
		h.handleConnect(w, r)
	default:
		h.handleHTTP(w, r)
	}
}

func (h *proxyHandler) authenticateClient(w http.ResponseWriter, r *http.Request) bool {
	if !h.rotator.AuthRequired {
		return true
	}

	header := strings.TrimSpace(r.Header.Get("Proxy-Authorization"))
	if header == "" {
		writeProxyAuthRequired(w)
		return false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		writeProxyAuthRequired(w)
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		writeProxyAuthRequired(w)
		return false
	}

	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		writeProxyAuthRequired(w)
		return false
	}

	if creds[0] != h.rotator.AuthUsername || creds[1] != h.rotator.AuthPassword {
		writeProxyAuthRequired(w)
		return false
	}

	return true
}

func writeProxyAuthRequired(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="Magpie Rotator"`)
	w.WriteHeader(http.StatusProxyAuthRequired)
	_, _ = w.Write([]byte("Proxy authentication required"))
}

func (h *proxyHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	next, err := getNextRotatingProxyFunc(h.rotator.UserID, h.rotator.ID)
	if err != nil {
		http.Error(w, "failed to acquire upstream proxy", http.StatusBadGateway)
		return
	}

	if !supportedUpstream(next.Protocol) {
		http.Error(w, "upstream protocol not supported by rotator", http.StatusBadGateway)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	targetURL := r.URL
	if !targetURL.IsAbs() {
		scheme := "http"
		if strings.HasPrefix(strings.ToLower(r.Proto), "https") {
			scheme = "https"
		}
		targetURL = &url.URL{
			Scheme:   scheme,
			Host:     r.Host,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
	}

	newReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "failed to build upstream request", http.StatusInternalServerError)
		return
	}

	newReq.Header = r.Header.Clone()
	newReq.Header.Del("Proxy-Authorization")

	transport := buildHTTPTransport(next)
	resp, err := transport.RoundTrip(newReq)
	if err != nil {
		http.Error(w, "upstream proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Warn("rotating proxy: failed to copy response body", "rotator_id", h.rotator.ID, "error", err)
	}
}

func (h *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "failed to hijack connection", http.StatusInternalServerError)
		return
	}

	defer func() {
		if err := clientConn.Close(); err != nil {
			log.Debug("rotating proxy: client connection close", "error", err)
		}
	}()

	next, err := getNextRotatingProxyFunc(h.rotator.UserID, h.rotator.ID)
	if err != nil {
		writeHijackedResponse(buf, http.StatusBadGateway, "Failed to acquire upstream proxy")
		return
	}

	if !supportedUpstream(next.Protocol) {
		writeHijackedResponse(buf, http.StatusBadGateway, "Upstream protocol not supported by rotator")
		return
	}

	upConn, err := dialUpstreamFunc(next)
	if err != nil {
		writeHijackedResponse(buf, http.StatusBadGateway, "Failed to connect to upstream proxy")
		return
	}

	if err := performUpstreamConnectFunc(upConn, r.Host, next); err != nil {
		_ = upConn.Close()
		writeHijackedResponse(buf, http.StatusBadGateway, "Upstream CONNECT failed")
		return
	}

	if _, err := clientConn.Write([]byte(connectEstablishedResponse)); err != nil {
		_ = upConn.Close()
		return
	}

	pipeConnections(clientConn, upConn)
}

func writeHijackedResponse(buf *bufio.ReadWriter, status int, message string) {
	fmt.Fprintf(buf, "HTTP/1.1 %d %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
		status,
		http.StatusText(status),
		len(message),
		message,
	)
	_ = buf.Flush()
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func supportedUpstream(protocol string) bool {
	switch strings.ToLower(protocol) {
	case "http", "https", "socks4", "socks5":
		return true
	default:
		return false
	}
}

func buildHTTPTransport(next *dto.RotatingProxyNext) *http.Transport {
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", next.IP, next.Port),
	}
	if next.HasAuth {
		proxyURL.User = url.UserPassword(next.Username, next.Password)
	}

	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
		MaxIdleConns:      0,
		IdleConnTimeout:   0,
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialProxyWithFallback(ctx, network, addr, next)
	}

	return transport
}

func dialUpstream(next *dto.RotatingProxyNext) (net.Conn, error) {
	address := fmt.Sprintf("%s:%d", next.IP, next.Port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	if shouldAttemptTLS(next.Protocol) {
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
		deadline := time.Now().Add(5 * time.Second)
		_ = conn.SetDeadline(deadline)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			fallback, fallbackErr := dialer.Dial("tcp", address)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			return fallback, nil
		}
		_ = tlsConn.SetDeadline(time.Time{})
		return tlsConn, nil
	}

	return conn, nil
}

func performUpstreamConnect(conn net.Conn, targetHost string, next *dto.RotatingProxyNext) error {
	request := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: Keep-Alive\r\n", targetHost, targetHost)
	if next.HasAuth {
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", next.Username, next.Password)))
		request += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}
	request += "\r\n"

	if _, err := conn.Write([]byte(request)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return errors.New("upstream returned non-200 response")
	}

	return nil
}

func pipeConnections(left, right net.Conn) {
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(left, right)
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(right, left)
		errCh <- err
	}()

	<-errCh
	left.Close()
	right.Close()
}

func dialProxyWithFallback(ctx context.Context, network, addr string, next *dto.RotatingProxyNext) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	if shouldAttemptTLS(next.Protocol) {
		tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
		deadline := time.Now().Add(5 * time.Second)
		_ = conn.SetDeadline(deadline)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return dialer.DialContext(ctx, network, addr)
		}
		_ = tlsConn.SetDeadline(time.Time{})
		return tlsConn, nil
	}

	return conn, nil
}

func shouldAttemptTLS(protocol string) bool {
	return strings.EqualFold(protocol, "https")
}

func readSocks5Target(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}

	if header[0] != 0x05 {
		_ = writeSocks5Reply(conn, 0x01)
		return "", errors.New("invalid socks version")
	}

	if header[1] != 0x01 {
		_ = writeSocks5Reply(conn, 0x07)
		return "", errors.New("unsupported socks5 command")
	}

	target, err := readSocksAddress(conn, header[3])
	if err != nil {
		_ = writeSocks5Reply(conn, 0x08)
		return "", err
	}

	return target, nil
}

func readSocksAddress(conn net.Conn, atyp byte) (string, error) {
	var host string

	switch atyp {
	case 0x01:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domain := make([]byte, int(lenBuf[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", err
		}
		host = string(domain)
	case 0x04:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	default:
		return "", errors.New("unsupported address type")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func writeSocks5Reply(conn net.Conn, rep byte) error {
	return writeSocks5BoundReply(conn, rep, net.IPv4zero, 0)
}

func writeSocks5Success(conn net.Conn, addr net.Addr) error {
	if tcp, ok := addr.(*net.TCPAddr); ok && tcp != nil {
		ip := tcp.IP
		if ip == nil {
			ip = net.IPv4zero
		}
		return writeSocks5BoundReply(conn, 0x00, ip, uint16(tcp.Port))
	}
	return writeSocks5BoundReply(conn, 0x00, net.IPv4zero, 0)
}

func writeSocks5BoundReply(conn net.Conn, rep byte, ip net.IP, port uint16) error {
	atyp := byte(0x01)
	addrBytes := ip.To4()

	if addrBytes == nil {
		if v6 := ip.To16(); v6 != nil {
			atyp = 0x04
			addrBytes = v6
		} else {
			addrBytes = []byte{0, 0, 0, 0}
		}
	}

	resp := []byte{0x05, rep, 0x00, atyp}
	resp = append(resp, addrBytes...)
	resp = append(resp, byte(port>>8), byte(port))

	_, err := conn.Write(resp)
	return err
}

func writeSocks4Response(conn net.Conn, status byte, port []byte, ip []byte) error {
	resp := []byte{0x00, status}

	if len(port) != 2 {
		port = []byte{0x00, 0x00}
	}
	if len(ip) != 4 {
		ip = []byte{0x00, 0x00, 0x00, 0x00}
	}

	resp = append(resp, port...)
	resp = append(resp, ip...)

	_, err := conn.Write(resp)
	return err
}

func connectThroughUpstream(target string, next *dto.RotatingProxyNext) (net.Conn, error) {
	upConn, err := dialUpstreamFunc(next)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(next.Protocol) {
	case "http", "https":
		if err := performUpstreamConnectFunc(upConn, target, next); err != nil {
			_ = upConn.Close()
			return nil, err
		}
		return upConn, nil
	case "socks5":
		if err := performSocks5UpstreamConnect(upConn, target, next); err != nil {
			_ = upConn.Close()
			return nil, err
		}
		return upConn, nil
	case "socks4":
		if err := performSocks4UpstreamConnect(upConn, target, next); err != nil {
			_ = upConn.Close()
			return nil, err
		}
		return upConn, nil
	default:
		_ = upConn.Close()
		return nil, fmt.Errorf("unsupported upstream protocol %s", next.Protocol)
	}
}

func performSocks5UpstreamConnect(conn net.Conn, target string, next *dto.RotatingProxyNext) error {
	greeting := []byte{0x05, 0x01, 0x00}
	if next.HasAuth {
		greeting[2] = 0x02
	}
	if _, err := conn.Write(greeting); err != nil {
		return err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x05 {
		return errors.New("invalid socks5 response from upstream")
	}
	if resp[1] == 0xff {
		return errors.New("upstream socks5 proxy offered no acceptable authentication methods")
	}

	if next.HasAuth && resp[1] != 0x02 {
		return errors.New("upstream socks5 proxy does not accept username/password authentication")
	}
	if next.HasAuth && resp[1] == 0x02 {
		if err := sendSocks5Credentials(conn, next.Username, next.Password); err != nil {
			return err
		}
	}

	host, port, err := splitTargetAddress(target)
	if err != nil {
		return err
	}

	atyp, addrBytes, portBytes, err := encodeSocksAddress(host, port)
	if err != nil {
		return err
	}

	req := []byte{0x05, 0x01, 0x00, atyp}
	req = append(req, addrBytes...)
	req = append(req, portBytes...)

	if _, err := conn.Write(req); err != nil {
		return err
	}

	reply := make([]byte, 4)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return err
	}
	if reply[0] != 0x05 {
		return errors.New("invalid socks5 connect reply")
	}
	if reply[1] != 0x00 {
		return fmt.Errorf("socks5 connect failed with code %d", reply[1])
	}

	return discardSocks5Address(conn, reply[3])
}

func performSocks4UpstreamConnect(conn net.Conn, target string, next *dto.RotatingProxyNext) error {
	host, port, err := splitTargetAddress(target)
	if err != nil {
		return err
	}

	ip := net.ParseIP(host)
	ipBytes := ip.To4()
	var domain string
	if ipBytes == nil {
		ipBytes = []byte{0x00, 0x00, 0x00, 0x01}
		domain = host
	}

	req := []byte{0x04, 0x01}
	portBytes := []byte{byte(port >> 8), byte(port)}
	req = append(req, portBytes...)
	req = append(req, ipBytes...)

	if next.HasAuth {
		userField := next.Username
		if next.Password != "" {
			userField = fmt.Sprintf("%s:%s", next.Username, next.Password)
		}
		req = append(req, []byte(userField)...)
	}
	req = append(req, 0x00)

	if domain != "" {
		req = append(req, []byte(domain)...)
		req = append(req, 0x00)
	}

	if _, err := conn.Write(req); err != nil {
		return err
	}

	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if len(resp) < 2 || resp[1] != 0x5A {
		return errors.New("socks4 connect failed")
	}

	return nil
}

func sendSocks5Credentials(conn net.Conn, username, password string) error {
	if len(username) > 255 || len(password) > 255 {
		return errors.New("socks5 credentials too long")
	}

	payload := []byte{0x01, byte(len(username))}
	payload = append(payload, []byte(username)...)
	payload = append(payload, byte(len(password)))
	payload = append(payload, []byte(password)...)

	if _, err := conn.Write(payload); err != nil {
		return err
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if len(resp) < 2 || resp[1] != 0x00 {
		return errors.New("socks5 authentication failed")
	}

	return nil
}

func encodeSocksAddress(host string, port uint16) (byte, []byte, []byte, error) {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return 0x01, v4, []byte{byte(port >> 8), byte(port)}, nil
		}
		if v6 := ip.To16(); v6 != nil {
			return 0x04, v6, []byte{byte(port >> 8), byte(port)}, nil
		}
	}

	if host == "" {
		return 0, nil, nil, errors.New("empty host")
	}

	if len(host) > 255 {
		return 0, nil, nil, errors.New("hostname too long")
	}

	addr := append([]byte{byte(len(host))}, []byte(host)...)
	return 0x03, addr, []byte{byte(port >> 8), byte(port)}, nil
}

func discardSocks5Address(conn net.Conn, atyp byte) error {
	var addrLen int

	switch atyp {
	case 0x01:
		addrLen = 4
	case 0x04:
		addrLen = 16
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return err
		}
		addrLen = int(lenBuf[0])
	default:
		return errors.New("unsupported address type in socks5 reply")
	}

	buf := make([]byte, addrLen+2)
	_, err := io.ReadFull(conn, buf)
	return err
}

func splitTargetAddress(target string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}

	return host, uint16(port), nil
}
