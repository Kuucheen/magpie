package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"

	"magpie/internal/support"
)

const (
	InstanceHeartbeatKeyPrefix = "magpie:instance:"
	DefaultHeartbeatInterval   = 15 * time.Second
	DefaultHeartbeatTTL        = 30 * time.Second
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
	heartbeatKey := keyPrefix + instanceID
	heartbeatValue, _ := json.Marshal(currentInstancePayload())

	sendHeartbeat := func() {
		if err := client.SetEx(ctx, heartbeatKey, heartbeatValue, ttl).Err(); err != nil {
			log.Error("Failed to update instance heartbeat", "key", heartbeatKey, "error", err)
		}
	}

	sendHeartbeat()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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
	keys, err := client.Keys(ctx, InstanceHeartbeatKeyPrefix+"*").Result()
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

func ListActiveInstances(ctx context.Context, client *redis.Client) ([]ActiveInstance, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	keys, err := client.Keys(ctx, InstanceHeartbeatKeyPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []ActiveInstance{}, nil
	}

	values, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make([]ActiveInstance, 0, len(keys))
	for idx, key := range keys {
		instance := ActiveInstance{
			ID: strings.TrimPrefix(key, InstanceHeartbeatKeyPrefix),
		}
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

	return result, nil
}
