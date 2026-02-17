package proxyqueue

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"magpie/internal/config"
	"magpie/internal/domain"
	"magpie/internal/jobs/runtime"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"
)

const (
	proxyKeyPrefix  = "proxy:"
	queueKey        = "proxy_queue"
	emptyQueueSleep = 1 * time.Second
)

//go:embed pop.lua
var luaPopScript string

type RedisProxyQueue struct {
	client    *redis.Client
	ctx       context.Context
	popScript *redis.Script
}

type queuedProxyUser struct {
	ID                         uint
	HTTPProtocol               bool
	HTTPSProtocol              bool
	SOCKS4Protocol             bool
	SOCKS5Protocol             bool
	Timeout                    uint16
	Retries                    uint8
	UseHttpsForSocks           bool
	TransportProtocol          string
	AutoRemoveFailingProxies   bool
	AutoRemoveFailureThreshold uint8
}

type queuedProxy struct {
	ID       uint64
	IP       string
	Port     uint16
	Username string
	Password string
	Hash     []byte
	Users    []queuedProxyUser
}

var PublicProxyQueue RedisProxyQueue

func init() {
	client, err := support.GetRedisClient()
	if err != nil {
		log.Fatal("Could not connect to redis for proxy queue", "error", err)
	}
	PublicProxyQueue = *NewRedisProxyQueue(client)

	go func() {
		updates := config.CheckIntervalUpdates()
		for interval := range updates {
			if err := PublicProxyQueue.Reschedule(interval); err != nil {
				log.Error("Failed to reschedule proxy queue after interval update", "error", err)
			}
		}
	}()
}

func NewRedisProxyQueue(client *redis.Client) *RedisProxyQueue {
	return &RedisProxyQueue{
		client:    client,
		ctx:       context.Background(),
		popScript: redis.NewScript(luaPopScript),
	}
}

func (rpq *RedisProxyQueue) AddToQueue(proxies []domain.Proxy) error {
	if len(proxies) == 0 {
		return nil
	}

	pipe := rpq.client.Pipeline()
	interval := config.GetTimeBetweenChecks()
	now := time.Now()
	proxyLenDuration := time.Duration(len(proxies))
	batchSize := 500 // Adjust based on your Redis server capabilities

	for i, proxy := range proxies {
		offset := (interval * time.Duration(i)) / proxyLenDuration
		nextCheck := now.Add(offset)
		hashKey := string(proxy.Hash)
		proxyKey := proxyKeyPrefix + hashKey

		proxyJSON, err := marshalQueuedProxy(proxy)
		if err != nil {
			return fmt.Errorf("failed to marshal proxy: %w", err)
		}

		pipe.Set(rpq.ctx, proxyKey, proxyJSON, 0)
		pipe.ZAddArgs(rpq.ctx, queueKey, redis.ZAddArgs{
			NX: true,
			Members: []redis.Z{{
				Score:  float64(nextCheck.Unix()),
				Member: hashKey,
			}},
		})

		// Execute in batches to prevent oversized pipelines
		if i%batchSize == 0 && i > 0 {
			if _, err := pipe.Exec(rpq.ctx); err != nil {
				return fmt.Errorf("batch pipeline failed: %w", err)
			}
			pipe = rpq.client.Pipeline()
		}
	}

	if _, err := pipe.Exec(rpq.ctx); err != nil {
		return fmt.Errorf("final pipeline exec failed: %w", err)
	}

	return nil
}

func (rpq *RedisProxyQueue) RemoveFromQueue(proxies []domain.Proxy) error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}
	if len(proxies) == 0 {
		return nil
	}

	const batchSize = 500
	pipe := rpq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rpq.ctx); err != nil {
			return fmt.Errorf("remove pipeline exec failed: %w", err)
		}
		pipe = rpq.client.Pipeline()
		opCount = 0
		return nil
	}

	for _, proxy := range proxies {
		if len(proxy.Hash) == 0 {
			continue
		}

		hashKey := string(proxy.Hash)
		proxyKey := proxyKeyPrefix + hashKey

		pipe.Del(rpq.ctx, proxyKey)
		opCount++
		pipe.ZRem(rpq.ctx, queueKey, hashKey)
		opCount++

		if opCount >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

func (rpq *RedisProxyQueue) GetNextProxy() (domain.Proxy, time.Time, error) {
	return rpq.GetNextProxyContext(rpq.ctx)
}

func (rpq *RedisProxyQueue) GetNextProxyContext(ctx context.Context) (domain.Proxy, time.Time, error) {
	if ctx == nil {
		ctx = rpq.ctx
	}

	for {
		select {
		case <-ctx.Done():
			return domain.Proxy{}, time.Time{}, ctx.Err()
		default:
		}

		currentTime := time.Now().Unix()
		result, err := rpq.popScript.Run(ctx, rpq.client, []string{queueKey, proxyKeyPrefix}, currentTime).Result()

		if errors.Is(err, redis.Nil) {
			select {
			case <-ctx.Done():
				return domain.Proxy{}, time.Time{}, ctx.Err()
			case <-time.After(emptyQueueSleep):
			}
			continue
		} else if err != nil {
			return domain.Proxy{}, time.Time{}, fmt.Errorf("lua script failed: %w", err)
		}

		resSlice := result.([]interface{})
		proxyJSON := resSlice[1].(string)
		score := resSlice[2].(int64)

		var payload queuedProxy
		if err := json.Unmarshal([]byte(proxyJSON), &payload); err != nil {
			return domain.Proxy{}, time.Time{}, fmt.Errorf("failed to unmarshal proxy: %w", err)
		}
		proxy := payload.toDomainProxy()

		return proxy, time.Unix(score, 0), nil
	}
}

func (rpq *RedisProxyQueue) RequeueProxy(proxy domain.Proxy, lastCheckTime time.Time) error {
	interval := config.GetTimeBetweenChecks()
	base := lastCheckTime
	// Clamp to now so overdue proxies don't keep hogging the queue.
	if now := time.Now(); now.After(base) {
		base = now
	}
	nextCheck := base.Add(interval)
	hashKey := string(proxy.Hash)
	proxyKey := proxyKeyPrefix + hashKey

	proxyJSON, err := marshalQueuedProxy(proxy)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy: %w", err)
	}

	pipe := rpq.client.Pipeline()
	pipe.Set(rpq.ctx, proxyKey, proxyJSON, 0)
	pipe.ZAdd(rpq.ctx, queueKey, redis.Z{
		Score:  float64(nextCheck.Unix()),
		Member: hashKey,
	})

	_, err = pipe.Exec(rpq.ctx)
	return err
}

func marshalQueuedProxy(proxy domain.Proxy) ([]byte, error) {
	return json.Marshal(newQueuedProxy(proxy))
}

func newQueuedProxy(proxy domain.Proxy) queuedProxy {
	users := make([]queuedProxyUser, 0, len(proxy.Users))
	for _, user := range proxy.Users {
		users = append(users, queuedProxyUser{
			ID:                         user.ID,
			HTTPProtocol:               user.HTTPProtocol,
			HTTPSProtocol:              user.HTTPSProtocol,
			SOCKS4Protocol:             user.SOCKS4Protocol,
			SOCKS5Protocol:             user.SOCKS5Protocol,
			Timeout:                    user.Timeout,
			Retries:                    user.Retries,
			UseHttpsForSocks:           user.UseHttpsForSocks,
			TransportProtocol:          user.TransportProtocol,
			AutoRemoveFailingProxies:   user.AutoRemoveFailingProxies,
			AutoRemoveFailureThreshold: user.AutoRemoveFailureThreshold,
		})
	}

	return queuedProxy{
		ID:       proxy.ID,
		IP:       proxy.GetIp(),
		Port:     proxy.Port,
		Username: proxy.Username,
		Password: proxy.Password,
		Hash:     proxy.Hash,
		Users:    users,
	}
}

func (qp queuedProxy) toDomainProxy() domain.Proxy {
	users := make([]domain.User, 0, len(qp.Users))
	for _, user := range qp.Users {
		users = append(users, domain.User{
			ID:                         user.ID,
			HTTPProtocol:               user.HTTPProtocol,
			HTTPSProtocol:              user.HTTPSProtocol,
			SOCKS4Protocol:             user.SOCKS4Protocol,
			SOCKS5Protocol:             user.SOCKS5Protocol,
			Timeout:                    user.Timeout,
			Retries:                    user.Retries,
			UseHttpsForSocks:           user.UseHttpsForSocks,
			TransportProtocol:          user.TransportProtocol,
			AutoRemoveFailingProxies:   user.AutoRemoveFailingProxies,
			AutoRemoveFailureThreshold: user.AutoRemoveFailureThreshold,
		})
	}

	return domain.Proxy{
		ID:       qp.ID,
		IP:       qp.IP,
		Port:     qp.Port,
		Username: qp.Username,
		Password: qp.Password,
		Hash:     qp.Hash,
		Users:    users,
	}
}

func (rpq *RedisProxyQueue) GetProxyCount() (int64, error) {
	return rpq.client.ZCard(rpq.ctx, queueKey).Result()
}

func (rpq *RedisProxyQueue) GetActiveInstances() (int, error) {
	return runtime.CountActiveInstances(rpq.ctx, rpq.client)
}

func (rpq *RedisProxyQueue) Close() error {
	return support.CloseRedisClient()
}

func (rpq *RedisProxyQueue) Reschedule(interval time.Duration) error {
	if rpq == nil {
		return errors.New("redis proxy queue is nil")
	}

	if interval <= 0 {
		interval = time.Second
	}

	total, err := rpq.client.ZCard(rpq.ctx, queueKey).Result()
	if err != nil {
		return fmt.Errorf("reschedule: failed to count queue entries: %w", err)
	}

	if total == 0 {
		return nil
	}

	now := time.Now()
	totalDuration := time.Duration(total)
	const fetchBatch int64 = 1000
	const updateBatch = 500

	pipe := rpq.client.Pipeline()
	opCount := 0

	flush := func() error {
		if opCount == 0 {
			return nil
		}
		if _, err := pipe.Exec(rpq.ctx); err != nil {
			return fmt.Errorf("reschedule: pipeline exec failed: %w", err)
		}
		pipe = rpq.client.Pipeline()
		opCount = 0
		return nil
	}

	for start := int64(0); start < total; start += fetchBatch {
		end := start + fetchBatch - 1
		if end >= total {
			end = total - 1
		}

		members, err := rpq.client.ZRange(rpq.ctx, queueKey, start, end).Result()
		if err != nil {
			return fmt.Errorf("reschedule: failed to fetch members: %w", err)
		}

		for idx, member := range members {
			globalIndex := start + int64(idx)
			offset := (interval * time.Duration(globalIndex)) / totalDuration
			nextCheck := now.Add(offset).Unix()

			pipe.ZAdd(rpq.ctx, queueKey, redis.Z{
				Score:  float64(nextCheck),
				Member: member,
			})
			opCount++

			if opCount != 0 && opCount%updateBatch == 0 {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}

	if err := flush(); err != nil {
		return err
	}

	log.Debug("proxy queue rescheduled", "entries", total, "interval", interval)
	return nil
}
