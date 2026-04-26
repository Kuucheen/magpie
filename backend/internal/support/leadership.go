package support

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultLeadershipTTL   = 45 * time.Second
	leadershipRetryDelay   = time.Second
	renewalTimeout         = 5 * time.Second
	minRenewalInterval     = time.Second
	defaultRenewalFraction = 3
)

var ErrLeaderLockNotAcquired = errors.New("support: leader lock not acquired")

var (
	leaderCounter atomic.Uint64

	renewScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
else
	return 0
end`)

	releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end`)
)

// RunWithLeader acquires a Redis-based leadership lock and invokes run while the
// lock is held. The run function is provided a context that is cancelled when
// leadership is lost or the parent context is done. The lock is renewed
// periodically and released when run returns. If the parent context is
// cancelled, the function returns.
func RunWithLeader(ctx context.Context, key string, ttl time.Duration, run func(context.Context)) error {
	if run == nil {
		return errors.New("support: leader run function cannot be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if ttl <= 0 {
		ttl = DefaultLeadershipTTL
	}

	client, err := GetRedisClient()
	if err != nil {
		return fmt.Errorf("support: leader lock redis client: %w", err)
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		session, err := acquireLeaderSession(ctx, client, key, ttl)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			log.Warn("leader lock: failed to acquire", "key", key, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(leadershipRetryDelay):
				continue
			}
		}

		log.Debug("leader lock: acquired", "key", key)
		run(session.ctx)
		session.Close()
		log.Debug("leader lock: released", "key", key)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(leadershipRetryDelay):
		}
	}
}

// RunLeaderTaskOnce tries to acquire a Redis-based leadership lock exactly once.
// If lock acquisition succeeds, run is invoked while the lock is renewed in the
// background. If another instance already holds the lock, ErrLeaderLockNotAcquired
// is returned.
func RunLeaderTaskOnce(ctx context.Context, key string, ttl time.Duration, run func(context.Context) error) error {
	if run == nil {
		return errors.New("support: leader run function cannot be nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if ttl <= 0 {
		ttl = DefaultLeadershipTTL
	}

	client, err := GetRedisClient()
	if err != nil {
		return fmt.Errorf("support: leader lock redis client: %w", err)
	}

	value := generateLeaderID()
	ok, err := acquireLeaderLock(ctx, client, key, value, ttl)
	if err != nil {
		return fmt.Errorf("support: leader lock set nx: %w", err)
	}
	if !ok {
		return ErrLeaderLockNotAcquired
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	session := &leaderSession{
		client:    client,
		key:       key,
		value:     value,
		ttl:       ttl,
		ctx:       sessionCtx,
		cancel:    cancel,
		stopRenew: make(chan struct{}),
	}
	go session.renewLoop()

	defer session.Close()
	return run(sessionCtx)
}

type leaderSession struct {
	client    *redis.Client
	key       string
	value     string
	ttl       time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	stopRenew chan struct{}
	closeOnce sync.Once
}

func acquireLeaderSession(ctx context.Context, client *redis.Client, key string, ttl time.Duration) (*leaderSession, error) {
	value := generateLeaderID()

	for {
		ok, err := acquireLeaderLock(ctx, client, key, value, ttl)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			log.Warn("leader lock: set nx failed", "key", key, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(leadershipRetryDelay):
				continue
			}
		}

		if ok {
			sessionCtx, cancel := context.WithCancel(ctx)
			session := &leaderSession{
				client:    client,
				key:       key,
				value:     value,
				ttl:       ttl,
				ctx:       sessionCtx,
				cancel:    cancel,
				stopRenew: make(chan struct{}),
			}
			go session.renewLoop()
			return session, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(leadershipRetryDelay):
		}
	}
}

func acquireLeaderLock(ctx context.Context, client *redis.Client, key, value string, ttl time.Duration) (bool, error) {
	_, err := client.SetArgs(ctx, key, value, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return false, err
}

func (ls *leaderSession) Close() {
	ls.closeOnce.Do(func() {
		ls.cancel()
		close(ls.stopRenew)
		if err := ls.releaseLock(); err != nil {
			log.Warn("leader lock: release failed", "key", ls.key, "error", err)
		}
	})
}

func (ls *leaderSession) renewLoop() {
	interval := ls.ttl / defaultRenewalFraction
	if interval <= 0 {
		interval = minRenewalInterval
	}
	if interval < minRenewalInterval {
		interval = minRenewalInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ls.stopRenew:
			return
		case <-ls.ctx.Done():
			return
		case <-ticker.C:
			if err := ls.renewLock(); err != nil {
				log.Warn("leader lock: renewal failed", "key", ls.key, "error", err)
				ls.cancel()
				return
			}
		}
	}
}

func (ls *leaderSession) renewLock() error {
	ctx, cancel := context.WithTimeout(context.Background(), renewalTimeout)
	defer cancel()

	ttlMs := ls.ttl.Milliseconds()
	if ttlMs <= 0 {
		ttlMs = DefaultLeadershipTTL.Milliseconds()
	}

	res, err := renewScript.Run(ctx, ls.client, []string{ls.key}, ls.value, ttlMs).Result()
	if err != nil {
		return err
	}

	if updated, ok := res.(int64); ok && updated == 0 {
		return errors.New("lock lost")
	}

	return nil
}

func (ls *leaderSession) releaseLock() error {
	ctx, cancel := context.WithTimeout(context.Background(), renewalTimeout)
	defer cancel()

	_, err := releaseScript.Run(ctx, ls.client, []string{ls.key}, ls.value).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	return nil
}

func generateLeaderID() string {
	host, _ := os.Hostname()
	counter := leaderCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d-%d", host, os.Getpid(), time.Now().UnixNano(), counter)
}
