package blacklist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
)

const (
	redisBlacklistChannel = "magpie:blacklist:updates"
	redisBlacklistTimeout = 5 * time.Second
)

type redisSyncState struct {
	mu     sync.RWMutex
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

type blacklistSyncEvent struct {
	Origin    string `json:"origin"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

var (
	globalRedisSync     redisSyncState
	blacklistSyncNodeID = generateBlacklistSyncNodeID()
)

// EnableRedisSynchronization wires blacklist cache reloads to Redis pub/sub so
// followers refresh their in-memory cache after a leader refresh.
func EnableRedisSynchronization(ctx context.Context, client *redis.Client) {
	if client == nil {
		log.Warn("Blacklist sync disabled: redis client is nil")
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	globalRedisSync.mu.Lock()
	if globalRedisSync.client != nil {
		globalRedisSync.mu.Unlock()
		return
	}

	syncCtx, cancel := context.WithCancel(ctx)
	globalRedisSync.client = client
	globalRedisSync.ctx = syncCtx
	globalRedisSync.cancel = cancel
	globalRedisSync.mu.Unlock()

	go subscribeToBlacklistUpdates(syncCtx, client)
}

func subscribeToBlacklistUpdates(ctx context.Context, client *redis.Client) {
	pubsub := client.Subscribe(ctx, redisBlacklistChannel)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, redis.ErrClosed) || ctx.Err() != nil {
				return
			}
			log.Error("Blacklist sync: subscription error", "error", err)
			time.Sleep(time.Second)
			continue
		}

		var event blacklistSyncEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Error("Blacklist sync: invalid payload", "error", err)
			continue
		}

		if event.Origin == blacklistSyncNodeID {
			continue
		}

		if err := LoadCache(ctx); err != nil {
			log.Error("Blacklist sync: cache reload failed", "error", err)
			continue
		}
		log.Debug("Blacklist sync: cache reloaded", "reason", event.Reason)
	}
}

func broadcastRefreshUpdate(ctx context.Context, reason string) error {
	client, baseCtx := blacklistRedisClient()
	if client == nil {
		return nil
	}

	event := blacklistSyncEvent{
		Origin:    blacklistSyncNodeID,
		Reason:    reason,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	merged := mergedContext(ctx, baseCtx)
	opCtx, cancel := redisTimeoutCtx(merged)
	defer cancel()

	return client.Publish(opCtx, redisBlacklistChannel, payload).Err()
}

func blacklistRedisClient() (*redis.Client, context.Context) {
	globalRedisSync.mu.RLock()
	defer globalRedisSync.mu.RUnlock()
	return globalRedisSync.client, globalRedisSync.ctx
}

func generateBlacklistSyncNodeID() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%d", host, os.Getpid(), time.Now().UnixNano())
}

func mergedContext(ctx context.Context, fallback context.Context) context.Context {
	switch {
	case ctx != nil && ctx.Err() == nil:
		return ctx
	case fallback != nil && fallback.Err() == nil:
		return fallback
	default:
		return context.Background()
	}
}

func redisTimeoutCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline && time.Until(deadline) <= redisBlacklistTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, redisBlacklistTimeout)
}
