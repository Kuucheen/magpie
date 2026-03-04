//go:build integration

package support

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisSentinelFailover_RecoversWritesAfterPrimaryDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("docker compose plugin not available")
	}

	resetRedisClientTestState()
	t.Cleanup(resetRedisClientTestState)

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	composeFile := filepath.Join(filepath.Dir(currentFile), "testdata", "redis-sentinel-compose.yml")
	project := fmt.Sprintf("magpie-redis-ha-it-%d", time.Now().UnixNano())

	runCompose(t, composeFile, project, "up", "-d")
	t.Cleanup(func() {
		runComposeNoFail(composeFile, project, "down", "-v", "--remove-orphans")
	})
	t.Cleanup(func() {
		runDockerNoFail("unpause", fmt.Sprintf("%s-redis-primary-1", project))
	})

	t.Setenv(envRedisMode, redisModeSentinel)
	t.Setenv(envRedisMasterName, "mymaster")
	t.Setenv(envRedisSentinelAddrs, "127.0.0.1:36379,127.0.0.1:36380,127.0.0.1:36381")
	t.Setenv(envRedisConnectRetryBackoffMS, "500")

	_ = CloseRedisClient()
	t.Cleanup(func() {
		_ = CloseRedisClient()
	})

	client := waitForRedisClient(t, 45*time.Second)

	ctx := context.Background()
	if err := client.Set(ctx, "redis:sentinel:failover", "before-failover", 0).Err(); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	runDocker(t, "pause", fmt.Sprintf("%s-redis-primary-1", project))

	deadline := time.Now().Add(120 * time.Second)
	var lastErr error
	recovered := false
	for time.Now().Before(deadline) {
		value := fmt.Sprintf("after-failover-%d", time.Now().UnixNano())
		err := client.Set(ctx, "redis:sentinel:failover", value, 0).Err()
		if err == nil {
			recovered = true
			break
		}

		lastErr = err
		_ = CloseRedisClient()
		if reconnectClient, reconnectErr := GetRedisClient(); reconnectErr == nil {
			client = reconnectClient
		}
		time.Sleep(2 * time.Second)
	}
	if !recovered {
		t.Fatalf("writes did not recover after sentinel failover: %v", lastErr)
	}
}

func waitForRedisClient(t *testing.T, timeout time.Duration) *redis.Client {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := GetRedisClient()
		if err == nil {
			return client
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}

	t.Fatalf("failed to get redis client within %s: %v", timeout, lastErr)
	return nil
}

func runCompose(t *testing.T, composeFile string, project string, args ...string) {
	t.Helper()

	baseArgs := []string{"compose", "-f", composeFile, "-p", project}
	cmd := exec.Command("docker", append(baseArgs, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %v failed: %v\n%s", append(baseArgs, args...), err, string(output))
	}
}

func runComposeNoFail(composeFile string, project string, args ...string) {
	baseArgs := []string{"compose", "-f", composeFile, "-p", project}
	cmd := exec.Command("docker", append(baseArgs, args...)...)
	_, _ = cmd.CombinedOutput()
}

func runDocker(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %v failed: %v\n%s", args, err, string(output))
	}
}

func runDockerNoFail(args ...string) {
	cmd := exec.Command("docker", args...)
	_, _ = cmd.CombinedOutput()
}
