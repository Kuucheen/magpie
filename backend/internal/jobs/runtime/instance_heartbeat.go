package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"

	"magpie/internal/support"
)

const (
	InstanceHeartbeatKeyPrefix = "magpie:instance:"
	InstanceHeartbeatIndexKey  = "magpie:instances:active"
	DefaultHeartbeatInterval   = 15 * time.Second
	DefaultHeartbeatTTL        = 30 * time.Second
	heartbeatScanBatchSize     = 200
)

type ActiveInstance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Region    string `json:"region"`
	PortStart int    `json:"port_start"`
	PortEnd   int    `json:"port_end"`
}

var instanceID = generateInstanceID()

func generateInstanceID() string {
	if configured := strings.TrimSpace(support.GetInstanceID()); configured != "" {
		return configured
	}
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
}

func currentInstancePayload() ActiveInstance {
	start, end := support.GetRotatingProxyPortRange()
	return ActiveInstance{
		ID:        instanceID,
		Name:      support.GetInstanceName(),
		Region:    support.GetInstanceRegion(),
		PortStart: start,
		PortEnd:   end,
	}
}

func CurrentInstance() ActiveInstance {
	return currentInstancePayload()
}

func StartInstanceHeartbeat(ctx context.Context, client *redis.Client, keyPrefix string, interval, ttl time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	heartbeatKey := heartbeatKeyForInstance(instanceID, keyPrefix)
	heartbeatValue, _ := json.Marshal(currentInstancePayload())

	sendHeartbeat := func() {
		expiresAt := time.Now().Add(ttl).UnixMilli()
		pipe := client.Pipeline()
		pipe.SetEx(ctx, heartbeatKey, heartbeatValue, ttl)
		pipe.ZAdd(ctx, InstanceHeartbeatIndexKey, redis.Z{
			Score:  float64(expiresAt),
			Member: instanceID,
		})
		if _, err := pipe.Exec(ctx); err != nil {
			log.Error("Failed to update instance heartbeat", "key", heartbeatKey, "error", err)
		}
	}

	sendHeartbeat()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			pipe := client.Pipeline()
			pipe.Del(cleanupCtx, heartbeatKey)
			pipe.ZRem(cleanupCtx, InstanceHeartbeatIndexKey, instanceID)
			if _, err := pipe.Exec(cleanupCtx); err != nil {
				log.Warn("Failed to cleanup instance heartbeat", "instance_id", instanceID, "error", err)
			}
			cancel()
			return
		case <-ticker.C:
			sendHeartbeat()
		}
	}
}

func LaunchInstanceHeartbeat(parent context.Context, client *redis.Client) context.CancelFunc {
	ctx, cancel := context.WithCancel(parent)
	go StartInstanceHeartbeat(ctx, client, InstanceHeartbeatKeyPrefix, DefaultHeartbeatInterval, DefaultHeartbeatTTL)
	return cancel
}

func CountActiveInstances(ctx context.Context, client *redis.Client) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := pruneExpiredInstanceIndex(ctx, client); err != nil {
		return 0, err
	}

	count, err := client.ZCard(ctx, InstanceHeartbeatIndexKey).Result()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		count, err = rebuildInstanceIndexFromScan(ctx, client)
		if err != nil {
			return 0, err
		}
	}

	return int(count), nil
}

func ListActiveInstances(ctx context.Context, client *redis.Client) ([]ActiveInstance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := pruneExpiredInstanceIndex(ctx, client); err != nil {
		return nil, err
	}

	instanceIDs, err := client.ZRange(ctx, InstanceHeartbeatIndexKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(instanceIDs) == 0 {
		if _, rebuildErr := rebuildInstanceIndexFromScan(ctx, client); rebuildErr != nil {
			return nil, rebuildErr
		}
		instanceIDs, err = client.ZRange(ctx, InstanceHeartbeatIndexKey, 0, -1).Result()
		if err != nil {
			return nil, err
		}
	}
	if len(instanceIDs) == 0 {
		return []ActiveInstance{}, nil
	}

	keys := make([]string, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		keys = append(keys, heartbeatKeyForInstance(id, InstanceHeartbeatKeyPrefix))
	}
	if len(keys) == 0 {
		return []ActiveInstance{}, nil
	}

	values, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make([]ActiveInstance, 0, len(keys))
	staleIDs := make([]string, 0)
	for idx, key := range keys {
		instanceID := strings.TrimPrefix(key, InstanceHeartbeatKeyPrefix)
		instance := ActiveInstance{ID: instanceID}
		if instance.ID == "" {
			continue
		}

		if idx < len(values) {
			if raw, ok := values[idx].(string); ok && strings.TrimSpace(raw) != "" {
				var payload ActiveInstance
				if err := json.Unmarshal([]byte(raw), &payload); err == nil {
					if strings.TrimSpace(payload.ID) != "" {
						instance.ID = strings.TrimSpace(payload.ID)
					}
					instance.Name = strings.TrimSpace(payload.Name)
					instance.Region = strings.TrimSpace(payload.Region)
					instance.PortStart = payload.PortStart
					instance.PortEnd = payload.PortEnd
				}
			} else {
				staleIDs = append(staleIDs, instance.ID)
			}
		}

		if instance.Name == "" {
			instance.Name = instance.ID
		}
		if instance.Region == "" {
			instance.Region = "Unknown"
		}
		if instance.PortStart <= 0 || instance.PortEnd <= 0 || instance.PortEnd < instance.PortStart {
			start, end := support.GetRotatingProxyPortRange()
			instance.PortStart = start
			instance.PortEnd = end
		}

		result = append(result, instance)
	}
	if len(staleIDs) > 0 {
		staleMembers := make([]interface{}, 0, len(staleIDs))
		for _, id := range staleIDs {
			staleMembers = append(staleMembers, id)
		}
		_ = client.ZRem(ctx, InstanceHeartbeatIndexKey, staleMembers...).Err()
	}

	return result, nil
}

func heartbeatKeyForInstance(id string, keyPrefix string) string {
	return keyPrefix + id
}

func pruneExpiredInstanceIndex(ctx context.Context, client *redis.Client) error {
	now := time.Now().UnixMilli()
	return client.ZRemRangeByScore(ctx, InstanceHeartbeatIndexKey, "-inf", strconv.FormatInt(now, 10)).Err()
}

func rebuildInstanceIndexFromScan(ctx context.Context, client *redis.Client) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		cursor uint64
		total  int64
	)

	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, InstanceHeartbeatKeyPrefix+"*", heartbeatScanBatchSize).Result()
		if err != nil {
			return 0, err
		}

		if len(keys) > 0 {
			ttlCmds := make([]*redis.DurationCmd, len(keys))
			pipe := client.Pipeline()
			for i, key := range keys {
				ttlCmds[i] = pipe.TTL(ctx, key)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				return 0, err
			}

			addPipe := client.Pipeline()
			batchAdds := int64(0)
			for i, key := range keys {
				id := strings.TrimPrefix(key, InstanceHeartbeatKeyPrefix)
				if id == "" {
					continue
				}
				ttl := ttlCmds[i].Val()
				if ttl <= 0 {
					continue
				}
				expiresAt := time.Now().Add(ttl).UnixMilli()
				addPipe.ZAdd(ctx, InstanceHeartbeatIndexKey, redis.Z{
					Score:  float64(expiresAt),
					Member: id,
				})
				batchAdds++
			}
			if batchAdds > 0 {
				if _, err := addPipe.Exec(ctx); err != nil {
					return 0, err
				}
				total += batchAdds
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if err := pruneExpiredInstanceIndex(ctx, client); err != nil {
		return 0, err
	}
	return client.ZCard(ctx, InstanceHeartbeatIndexKey).Result()
}
