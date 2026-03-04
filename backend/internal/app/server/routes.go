package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/charmbracelet/log"

	"magpie/internal/auth"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func enableCORS(next http.Handler) http.Handler {
	cors := resolveCORSConfig()
	allowedMethods := "GET, POST, OPTIONS, PUT, DELETE"
	allowedHeaders := "Content-Type, Authorization, X-Request-ID, X-Admin-Bootstrap-Token"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !cors.isAllowed(origin) && !isSameHostOrigin(origin, r) {
				log.Warn("Blocked CORS origin", "origin", origin, "request_host", r.Host)
				writeError(w, "CORS origin is not allowed", http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.Header().Add("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func OpenRoutes(ctx context.Context, port int) error {
	if ctx == nil {
		ctx = context.Background()
	}

	router := http.NewServeMux()
	router.Handle("GET /healthz", http.HandlerFunc(healthz))
	router.Handle("GET /readyz", http.HandlerFunc(readyz))
	router.Handle("GET /metrics", metricsHandler())

	gqlHandler, err := getGraphQLHandler()
	if err != nil {
		return fmt.Errorf("failed to initialize graphql handler: %w", err)
	}

	apiMux := http.NewServeMux()
	apiMux.Handle("/graphql", applyRequestBodyLimit(gqlHandler, resolveJSONMaxBodyBytes()))
	apiMux.Handle("POST /register", withRegisterRateLimit(http.HandlerFunc(registerUser)))
	apiMux.Handle("POST /login", withLoginRateLimit(http.HandlerFunc(loginUser)))
	apiMux.Handle("POST /logout", auth.RequireAuth(http.HandlerFunc(logoutUser)))
	apiMux.Handle("POST /refreshToken", auth.RequireAuth(http.HandlerFunc(refreshToken)))
	apiMux.Handle("GET /checkLogin", auth.RequireAuth(http.HandlerFunc(checkLogin)))
	apiMux.Handle("POST /changePassword", auth.RequireAuth(http.HandlerFunc(changePassword)))
	apiMux.Handle("POST /deleteAccount", auth.RequireAuth(http.HandlerFunc(deleteAccount)))
	apiMux.Handle("POST /saveSettings", auth.IsAdmin(http.HandlerFunc(saveSettings)))
	apiMux.Handle("GET /releases", http.HandlerFunc(getReleases))
	apiMux.Handle("GET /getDashboardInfo", auth.RequireAuth(http.HandlerFunc(getDashboardInfo)))

	apiMux.Handle("GET /getProxyCount", auth.RequireAuth(http.HandlerFunc(getProxyCount)))
	apiMux.Handle("GET /getProxyPage/{page}", auth.RequireAuth(http.HandlerFunc(getProxyPage)))
	apiMux.Handle("GET /proxyFilters", auth.RequireAuth(http.HandlerFunc(getProxyFilters)))
	apiMux.Handle("GET /proxies/{id}/statistics", auth.RequireAuth(http.HandlerFunc(getProxyStatistics)))
	apiMux.Handle("GET /proxies/{id}/statistics/{statisticId}", auth.RequireAuth(http.HandlerFunc(getProxyStatisticResponseBody)))
	apiMux.Handle("GET /proxies/{id}", auth.RequireAuth(http.HandlerFunc(getProxyDetail)))
	apiMux.Handle("POST /addProxies", auth.RequireAuth(http.HandlerFunc(addProxies)))
	apiMux.Handle("DELETE /proxies", auth.RequireAuth(http.HandlerFunc(deleteProxies)))

	apiMux.Handle("GET /rotatingProxies", auth.RequireAuth(http.HandlerFunc(listRotatingProxies)))
	apiMux.Handle("GET /rotatingProxies/instances", auth.RequireAuth(http.HandlerFunc(listRotatingProxyInstances)))
	apiMux.Handle("POST /rotatingProxies", auth.RequireAuth(http.HandlerFunc(createRotatingProxy)))
	apiMux.Handle("DELETE /rotatingProxies/{id}", auth.RequireAuth(http.HandlerFunc(deleteRotatingProxy)))
	apiMux.Handle("POST /rotatingProxies/{id}/next", auth.RequireAuth(http.HandlerFunc(getNextRotatingProxy)))

	apiMux.Handle("GET /getScrapingSourcesCount", auth.RequireAuth(http.HandlerFunc(getScrapeSourcesCount)))
	apiMux.Handle("GET /getScrapingSourcesPage/{page}", auth.RequireAuth(http.HandlerFunc(getScrapeSourcePage)))
	apiMux.Handle("POST /scrapingSources", auth.RequireAuth(http.HandlerFunc(saveScrapingSources)))
	apiMux.Handle("DELETE /scrapingSources", auth.RequireAuth(http.HandlerFunc(deleteScrapingSources)))
	apiMux.Handle("GET /scrapingSources/check", auth.RequireAuth(http.HandlerFunc(checkScrapeSourceRobots)))
	apiMux.Handle("GET /scrapingSources/respectRobots", auth.RequireAuth(http.HandlerFunc(getRobotsRespectSetting)))
	apiMux.Handle("GET /scrapingSources/{id}/proxies", auth.RequireAuth(http.HandlerFunc(getScrapeSourceProxies)))
	apiMux.Handle("GET /scrapingSources/{id}", auth.RequireAuth(http.HandlerFunc(getScrapeSourceDetail)))

	apiMux.Handle("GET /user/settings", auth.RequireAuth(http.HandlerFunc(getUserSettings)))
	apiMux.Handle("POST /user/settings", auth.RequireAuth(http.HandlerFunc(saveUserSettings)))
	apiMux.Handle("GET /user/role", auth.RequireAuth(http.HandlerFunc(getUserRole)))
	apiMux.Handle("POST /user/export", auth.RequireAuth(http.HandlerFunc(exportProxies)))
	apiMux.Handle("GET /global/settings", auth.IsAdmin(http.HandlerFunc(getGlobalSettings)))

	router.Handle("/api", http.StripPrefix("/api", apiMux))
	router.Handle("/api/", http.StripPrefix("/api", apiMux))

	log.Debug("Routes opened")
	timeouts := resolveServerTimeouts()

	handler := withRequestID(withAccessLog(withPanicRecovery(enableCORS(router))))

	server := http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadTimeout:       timeouts.readTimeout,
		ReadHeaderTimeout: timeouts.readHeaderTimeout,
		WriteTimeout:      timeouts.writeTimeout,
		IdleTimeout:       timeouts.idleTimeout,
	}

	log.Infof("Starting magpie backend on port :%d", port)
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- fmt.Errorf("api server failed: %w", err)
			return
		}
		serverErrCh <- nil
	}()

	select {
	case err := <-serverErrCh:
		return err
	case <-ctx.Done():
		log.Info("Shutting down API server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), resolveServerShutdownTimeout())
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("api server shutdown failed: %w", err)
		}
		return <-serverErrCh
	}
}
