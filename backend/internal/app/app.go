package app

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

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

	configureLogLevel()

	backendPortFlag := flag.Int("backend-port", defaultBackendPort, "Port for API server")
	productionFlag := flag.Bool("production", false, "Run in production mode")
	flag.Parse()

	config.SetProductionMode(*productionFlag || config.RuntimeEnvironmentIndicatesProduction())

	if err := auth.RequireJWTSecretConfigured(); err != nil {
		return err
	}
	if err := auth.RequireJWTTTLConfigured(); err != nil {
		return err
	}
	if err := security.RequireProxyEncryptionKeyConfigured(); err != nil {
		return err
	}

	backendPort := resolvePort("BACKEND_PORT", "backend-port", *backendPortFlag)
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redisClient, err := support.GetRedisClient()
	if err != nil {
		log.Warn("Redis unavailable at startup; continuing in degraded mode", "error", err)
	}

	heartbeatCancel := runtime.LaunchInstanceHeartbeat(rootCtx, redisClient)
	defer heartbeatCancel()

	if err := bootstrap.Setup(rootCtx); err != nil {
		return err
	}

	defer func() {
		if err := proxyqueue.PublicProxyQueue.Close(); err != nil {
			log.Warn("error closing proxy queue", "error", err)
		}
		if err := sitequeue.PublicScrapeSiteQueue.Close(); err != nil {
			log.Warn("error closing scrape-site queue", "error", err)
		}
	}()

	return server.OpenRoutes(rootCtx, backendPort)
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

func configureLogLevel() {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))

	switch raw {
	case "", "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	default:
		log.SetLevel(log.InfoLevel)
		log.Warn("invalid LOG_LEVEL value, defaulting to info", "value", raw)
	}
}
