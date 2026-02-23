package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"magpie/internal/app/bootstrap"
	"magpie/internal/app/server"
	"magpie/internal/auth"
	"magpie/internal/config"
	proxyqueue "magpie/internal/jobs/queue/proxy"
	sitequeue "magpie/internal/jobs/queue/sites"
	"magpie/internal/jobs/runtime"
	"magpie/internal/security"
	"magpie/internal/support"
)

const (
	defaultBackendPort = 5656
)

func Run() error {
	if err := godotenv.Load(); err != nil {
		log.Warn("No .env file found. Falling back to system environment variables.")
	}

	log.SetLevel(log.DebugLevel)
	if err := auth.RequireJWTSecretConfigured(); err != nil {
		return err
	}
	if err := security.RequireProxyEncryptionKeyConfigured(); err != nil {
		return err
	}

	backendPortFlag := flag.Int("backend-port", defaultBackendPort, "Port for API server")
	productionFlag := flag.Bool("production", false, "Run in production mode")
	flag.Parse()

	config.SetProductionMode(*productionFlag)

	backendPort := resolvePort("BACKEND_PORT", "backend-port", *backendPortFlag)

	redisClient, err := support.GetRedisClient()
	if err != nil {
		return fmt.Errorf("failed to get redis client: %w", err)
	}

	heartbeatCancel := runtime.LaunchInstanceHeartbeat(context.Background(), redisClient)
	defer heartbeatCancel()

	bootstrap.Setup()

	defer func() {
		if err := proxyqueue.PublicProxyQueue.Close(); err != nil {
			log.Warn("error closing proxy queue", "error", err)
		}
		if err := sitequeue.PublicScrapeSiteQueue.Close(); err != nil {
			log.Warn("error closing scrape-site queue", "error", err)
		}
	}()

	return server.OpenRoutes(backendPort)
}

func resolvePort(primaryEnv, legacyEnv string, fallback int) int {
	if port := readPort(primaryEnv); port != 0 {
		return port
	}
	if port := readPort(legacyEnv); port != 0 {
		return port
	}
	return fallback
}

func readPort(envKey string) int {
	raw := os.Getenv(envKey)
	if raw == "" {
		return 0
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port == 0 {
		log.Warn("invalid port override", "env", envKey, "value", raw)
		return 0
	}
	return port
}
