package support

import (
	"context"
	"fmt"
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

	redisURL := GetEnv("redisUrl", "redis://localhost:8946")

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		redisLastErr = fmt.Errorf("failed to parse Redis URL %q: %w", redisURL, err)
		redisRetryAfter = now.Add(redisConnectRetryBackoff())
		return nil, redisLastErr
	}

	client := redis.NewClient(opt)
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
