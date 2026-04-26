package geolite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"

	"magpie/internal/database"
	"magpie/internal/support"
)

const (
	geoLiteRedisKeyPrefix = "magpie:geolite:file:"
	geoLiteRedisChannel   = "magpie:geolite:updates"
	geoLiteRedisOpTimeout = 30 * time.Second
)

type geoLiteUpdatePayload struct {
	Files     []string `json:"files"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

type geoLiteRedisState struct {
	mu     sync.RWMutex
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

var globalGeoLiteRedis geoLiteRedisState

// EnableRedisDistribution wires GeoLite database replication to Redis so that
// only one node needs to download the MaxMind archives.
func EnableRedisDistribution(ctx context.Context, client *redis.Client) {
	if client == nil {
		log.Warn("GeoLite redis distribution disabled: redis client is nil")
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	syncCtx, cancel := context.WithCancel(ctx)

	globalGeoLiteRedis.mu.Lock()
	if globalGeoLiteRedis.client != nil {
		globalGeoLiteRedis.mu.Unlock()
		cancel()
		return
	}

	globalGeoLiteRedis.client = client
	globalGeoLiteRedis.ctx = syncCtx
	globalGeoLiteRedis.cancel = cancel
	globalGeoLiteRedis.mu.Unlock()

	go func() {
		if updated, err := fetchGeoLiteFromRedis(syncCtx, client, nil); err != nil {
			log.Error("geolite redis sync: initial load failed", "error", err)
		} else if updated {
			log.Info("geolite redis sync: loaded databases from redis")
		}
	}()

	go subscribeToGeoLiteUpdates(syncCtx, client)
}

// PublishGeoLiteDatabases uploads the current GeoLite databases to Redis and
// notifies other instances to pull them. filenames is optional; when empty all
// known editions are published.
func PublishGeoLiteDatabases(ctx context.Context, filenames []string) error {
	if len(filenames) == 0 {
		filenames = defaultGeoLiteFilenames()
	}

	client, baseCtx := geoLiteRedisClient()
	if client == nil {
		var err error
		client, err = support.GetRedisClient()
		if err != nil {
			return fmt.Errorf("geolite redis sync: redis client unavailable: %w", err)
		}
		baseCtx = context.Background()
	}

	opCtx := mergedContext(ctx, baseCtx)
	for _, name := range filenames {
		path := database.GeoLiteFilePath(name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("geolite redis sync: read %s: %w", name, err)
		}
		if err := storeGeoLiteFile(opCtx, client, name, data); err != nil {
			return fmt.Errorf("geolite redis sync: store %s: %w", name, err)
		}
	}

	payload := geoLiteUpdatePayload{
		Files:     filenames,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("geolite redis sync: serialize payload: %w", err)
	}

	if err := publishGeoLiteNotification(opCtx, client, data); err != nil {
		return err
	}

	return nil
}

func subscribeToGeoLiteUpdates(ctx context.Context, client *redis.Client) {
	pubsub := client.Subscribe(ctx, geoLiteRedisChannel)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, redis.ErrClosed) || ctx.Err() != nil {
				return
			}
			log.Error("geolite redis sync: subscription error", "error", err)
			time.Sleep(time.Second)
			continue
		}

		var payload geoLiteUpdatePayload
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			log.Error("geolite redis sync: invalid payload", "error", err)
			continue
		}
		files := payload.Files
		if len(files) == 0 {
			files = defaultGeoLiteFilenames()
		}

		if updated, err := fetchGeoLiteFromRedis(ctx, client, files); err != nil {
			log.Error("geolite redis sync: failed to apply update", "error", err)
		} else if updated {
			log.Info("geolite redis sync: applied update", "files", files)
		}
	}
}

func fetchGeoLiteFromRedis(ctx context.Context, client *redis.Client, filenames []string) (bool, error) {
	if client == nil {
		return false, errors.New("geolite redis sync: redis client is nil")
	}
	if len(filenames) == 0 {
		filenames = defaultGeoLiteFilenames()
	}

	var updated bool
	for _, name := range filenames {
		data, err := fetchGeoLiteFile(ctx, client, name)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return false, err
		}
		if len(data) == 0 {
			continue
		}
		destPath := database.GeoLiteFilePath(name)
		if err := writeToFile(destPath, bytes.NewReader(data)); err != nil {
			return false, fmt.Errorf("geolite redis sync: write %s: %w", name, err)
		}
		updated = true
	}

	if updated {
		if err := database.ReloadGeoLiteFromDisk(); err != nil {
			return false, fmt.Errorf("geolite redis sync: reload databases: %w", err)
		}
	}

	return updated, nil
}

func storeGeoLiteFile(ctx context.Context, client *redis.Client, filename string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	opCtx, cancel := redisTimeoutCtx(ctx)
	defer cancel()
	return client.Set(opCtx, geoLiteRedisKey(filename), data, 0).Err()
}

func publishGeoLiteNotification(ctx context.Context, client *redis.Client, payload []byte) error {
	opCtx, cancel := redisTimeoutCtx(ctx)
	defer cancel()
	return client.Publish(opCtx, geoLiteRedisChannel, payload).Err()
}

func fetchGeoLiteFile(ctx context.Context, client *redis.Client, filename string) ([]byte, error) {
	opCtx, cancel := redisTimeoutCtx(ctx)
	defer cancel()
	return client.Get(opCtx, geoLiteRedisKey(filename)).Bytes()
}

func geoLiteRedisKey(filename string) string {
	return geoLiteRedisKeyPrefix + filename
}

func defaultGeoLiteFilenames() []string {
	files := make([]string, 0, len(downloadTargets))
	for _, target := range downloadTargets {
		files = append(files, target.filename)
	}
	return files
}

func geoLiteRedisClient() (*redis.Client, context.Context) {
	globalGeoLiteRedis.mu.RLock()
	defer globalGeoLiteRedis.mu.RUnlock()
	return globalGeoLiteRedis.client, globalGeoLiteRedis.ctx
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
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline && time.Until(deadline) <= geoLiteRedisOpTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, geoLiteRedisOpTimeout)
}
