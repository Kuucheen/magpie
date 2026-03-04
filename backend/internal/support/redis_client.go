package support

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisMu     sync.Mutex
	redisClient *redis.Client
	redisNow    = time.Now

	redisRetryAfter time.Time
	redisLastErr    error
)

const (
	defaultRedisConnectRetryBackoff = 5 * time.Second
	envRedisConnectRetryBackoffMS   = "REDIS_CONNECT_RETRY_BACKOFF_MS"
	envRedisMode                    = "REDIS_MODE"
	envRedisMasterName              = "REDIS_MASTER_NAME"
	envRedisSentinelAddrs           = "REDIS_SENTINEL_ADDRS"
	envRedisPassword                = "REDIS_PASSWORD"
	envRedisSentinelPassword        = "REDIS_SENTINEL_PASSWORD"
	redisModeSingle                 = "single"
	redisModeSentinel               = "sentinel"
)

func GetRedisClient() (*redis.Client, error) {
	redisMu.Lock()
	defer redisMu.Unlock()

	if redisClient != nil {
		return redisClient, nil
	}
	now := redisNow()
	if !redisRetryAfter.IsZero() && now.Before(redisRetryAfter) {
		wait := redisRetryAfter.Sub(now).Round(time.Millisecond)
		if wait < time.Millisecond {
			wait = time.Millisecond
		}
		if redisLastErr != nil {
			return nil, fmt.Errorf("redis reconnect deferred for %s: %w", wait, redisLastErr)
		}
		return nil, fmt.Errorf("redis reconnect deferred for %s", wait)
	}

	client, err := buildRedisClientFromEnv()
	if err != nil {
		redisLastErr = err
		redisRetryAfter = now.Add(redisConnectRetryBackoff())
		return nil, redisLastErr
	}

	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		redisLastErr = fmt.Errorf("failed to connect to Redis: %w", err)
		redisRetryAfter = now.Add(redisConnectRetryBackoff())
		return nil, redisLastErr
	}

	redisClient = client
	redisLastErr = nil
	redisRetryAfter = time.Time{}
	return redisClient, nil
}

type RedisClientStatus struct {
	Mode       string
	Connected  bool
	LastError  string
	RetryAfter time.Duration
}

func GetRedisClientStatus() RedisClientStatus {
	redisMu.Lock()
	defer redisMu.Unlock()

	status := RedisClientStatus{
		Mode:      redisModeFromEnv(),
		Connected: redisClient != nil,
	}
	if redisLastErr != nil {
		status.LastError = redisLastErr.Error()
	}

	if !redisRetryAfter.IsZero() {
		wait := redisRetryAfter.Sub(redisNow()).Round(time.Millisecond)
		if wait > 0 {
			status.RetryAfter = wait
		}
	}

	return status
}

func CloseRedisClient() error {
	redisMu.Lock()
	defer redisMu.Unlock()

	redisLastErr = nil
	redisRetryAfter = time.Time{}

	if redisClient == nil {
		return nil
	}

	err := redisClient.Close()
	redisClient = nil
	return err
}

func redisConnectRetryBackoff() time.Duration {
	ms := GetEnvInt(envRedisConnectRetryBackoffMS, int(defaultRedisConnectRetryBackoff/time.Millisecond))
	if ms <= 0 {
		ms = int(defaultRedisConnectRetryBackoff / time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond
}

func buildRedisClientFromEnv() (*redis.Client, error) {
	mode := redisModeFromEnv()
	switch mode {
	case redisModeSingle:
		redisURL := GetEnv("redisUrl", "redis://localhost:8946")

		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redisUrl")
		}
		if opt.Password == "" {
			opt.Password = strings.TrimSpace(GetEnv(envRedisPassword, ""))
		}
		return redis.NewClient(opt), nil
	case redisModeSentinel:
		masterName := strings.TrimSpace(GetEnv(envRedisMasterName, ""))
		if masterName == "" {
			return nil, fmt.Errorf("missing %s for sentinel mode", envRedisMasterName)
		}

		sentinelAddrs := parseRedisSentinelAddrs(GetEnv(envRedisSentinelAddrs, ""))
		if len(sentinelAddrs) == 0 {
			return nil, fmt.Errorf("missing %s for sentinel mode", envRedisSentinelAddrs)
		}

		opts := &redis.FailoverOptions{
			MasterName:       masterName,
			SentinelAddrs:    sentinelAddrs,
			Password:         strings.TrimSpace(GetEnv(envRedisPassword, "")),
			SentinelPassword: strings.TrimSpace(GetEnv(envRedisSentinelPassword, "")),
		}
		return redis.NewFailoverClient(opts), nil
	default:
		return nil, fmt.Errorf("invalid %s %q, expected %q or %q", envRedisMode, mode, redisModeSingle, redisModeSentinel)
	}
}

func redisModeFromEnv() string {
	mode := strings.ToLower(strings.TrimSpace(GetEnv(envRedisMode, redisModeSingle)))
	if mode == "" {
		return redisModeSingle
	}
	return mode
}

func parseRedisSentinelAddrs(raw string) []string {
	parts := strings.Split(raw, ",")
	addrs := make([]string, 0, len(parts))
	for _, part := range parts {
		addr := strings.TrimSpace(part)
		if addr == "" {
			continue
		}
		addrs = append(addrs, addr)
	}
	return addrs
}
