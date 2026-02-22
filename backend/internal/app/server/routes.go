package server

import (
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow any origin
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Pass the request to the next handler
		next.ServeHTTP(w, r)
	})
}

func OpenRoutes(port int) error {

	router := http.NewServeMux()

	gqlHandler, err := getGraphQLHandler()
	if err != nil {
		return fmt.Errorf("failed to initialize graphql handler: %w", err)
	}

	apiMux := http.NewServeMux()
	apiMux.Handle("/graphql", gqlHandler)
	apiMux.HandleFunc("POST /register", registerUser)
	apiMux.HandleFunc("POST /login", loginUser)
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

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: enableCORS(router),
	}

	log.Infof("Starting magpie backend on port :%d", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server failed: %w", err)
	}
	return nil
}
