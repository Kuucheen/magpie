package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"magpie/internal/domain"

	"github.com/redis/go-redis/v9"
)

func TestAddProxyStatistic_BlocksUntilQueueCapacityAvailable(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastBackpressureLog.Load()
	originalDropped := proxyStatisticDroppedCount.Load()
	originalReady := proxyStatisticStreamReady.Load()
	originalCfg := proxyStatisticStreamCfg
	originalClient := proxyStatisticStreamClient
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastBackpressureLog.Store(originalLastLog)
		proxyStatisticDroppedCount.Store(originalDropped)
		proxyStatisticStreamReady.Store(originalReady)
		proxyStatisticStreamCfg = originalCfg
		proxyStatisticStreamClient = originalClient
	})

	t.Setenv(envProxyStatisticsRedisStreamEnabled, "false")
	proxyStatisticStreamReady.Store(false)
	proxyStatisticStreamCfg = proxyStatisticStreamConfig{}
	proxyStatisticStreamClient = nil
	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastBackpressureLog.Store(time.Now().Unix())
	proxyStatisticDroppedCount.Store(0)
	proxyStatisticQueue <- domain.ProxyStatistic{ProxyID: 1}

	done := make(chan struct{})
	go func() {
		AddProxyStatistic(domain.ProxyStatistic{ProxyID: 2})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected AddProxyStatistic to block on full queue")
	case <-time.After(50 * time.Millisecond):
	}

	first := <-proxyStatisticQueue
	if first.ProxyID != 1 {
		t.Fatalf("unexpected first queue entry ProxyID = %d, want %d", first.ProxyID, 1)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AddProxyStatistic did not unblock after queue drain")
	}

	second := <-proxyStatisticQueue
	if second.ProxyID != 2 {
		t.Fatalf("unexpected second queue entry ProxyID = %d, want %d", second.ProxyID, 2)
	}
	if got := proxyStatisticDroppedCount.Load(); got != 0 {
		t.Fatalf("dropped count = %d, want 0", got)
	}
}

func TestAddProxyStatistic_EnqueuesWhenCapacityAvailable(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastBackpressureLog.Load()
	originalDropped := proxyStatisticDroppedCount.Load()
	originalReady := proxyStatisticStreamReady.Load()
	originalCfg := proxyStatisticStreamCfg
	originalClient := proxyStatisticStreamClient
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastBackpressureLog.Store(originalLastLog)
		proxyStatisticDroppedCount.Store(originalDropped)
		proxyStatisticStreamReady.Store(originalReady)
		proxyStatisticStreamCfg = originalCfg
		proxyStatisticStreamClient = originalClient
	})

	t.Setenv(envProxyStatisticsRedisStreamEnabled, "false")
	proxyStatisticStreamReady.Store(false)
	proxyStatisticStreamCfg = proxyStatisticStreamConfig{}
	proxyStatisticStreamClient = nil
	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastBackpressureLog.Store(time.Now().Unix())
	proxyStatisticDroppedCount.Store(0)

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 42})

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
	if got := proxyStatisticDroppedCount.Load(); got != 0 {
		t.Fatalf("dropped count = %d, want 0", got)
	}
}

func TestResolveStatisticsOverloadPolicy_PrefersTenantOverride(t *testing.T) {
	proxyStatisticStreamCfg = proxyStatisticStreamConfig{
		overloadPolicy: statisticsOverloadPolicyDropNew,
		tenantOverloadPolicies: map[uint]string{
			5: statisticsOverloadPolicyBlock,
		},
	}

	if got := resolveStatisticsOverloadPolicy([]uint{1, 2}); got != statisticsOverloadPolicyDropNew {
		t.Fatalf("policy = %q, want %q", got, statisticsOverloadPolicyDropNew)
	}
	if got := resolveStatisticsOverloadPolicy([]uint{5}); got != statisticsOverloadPolicyBlock {
		t.Fatalf("policy = %q, want %q", got, statisticsOverloadPolicyBlock)
	}
}

func TestResolveStatisticsIngestWorkers_Default(t *testing.T) {
	t.Setenv(envProxyStatisticsIngestWorkers, "")

	if got := resolveStatisticsIngestWorkers(); got != defaultStatisticsIngestWorkers {
		t.Fatalf("workers = %d, want %d", got, defaultStatisticsIngestWorkers)
	}
}

func TestResolveStatisticsIngestWorkers_ClampsToRange(t *testing.T) {
	t.Setenv(envProxyStatisticsIngestWorkers, "0")
	if got := resolveStatisticsIngestWorkers(); got != 1 {
		t.Fatalf("workers = %d, want 1", got)
	}

	t.Setenv(envProxyStatisticsIngestWorkers, "999")
	if got := resolveStatisticsIngestWorkers(); got != maxStatisticsIngestWorkers {
		t.Fatalf("workers = %d, want %d", got, maxStatisticsIngestWorkers)
	}
}

func TestResolveProxyStatisticStreamConfig_DefaultsAndValidation(t *testing.T) {
	t.Setenv(envProxyStatisticsRedisStreamEnabled, "")
	t.Setenv(envProxyStatisticsRedisStreamKey, "")
	t.Setenv(envProxyStatisticsRedisStreamGroup, "")
	t.Setenv(envProxyStatisticsRedisStreamMaxLen, "0")
	t.Setenv(envProxyStatisticsRedisStreamOverloadPolicy, "invalid")
	t.Setenv(envProxyStatisticsTenantOverloadPolicies, "1=drop_new,2=block,bad=drop_new")

	cfg := resolveProxyStatisticStreamConfig()
	if !cfg.enabled {
		t.Fatal("expected stream ingest enabled by default")
	}
	if cfg.streamKey != defaultProxyStatisticsRedisStreamKey {
		t.Fatalf("stream key = %q, want %q", cfg.streamKey, defaultProxyStatisticsRedisStreamKey)
	}
	if cfg.groupName != defaultProxyStatisticsRedisStreamGroup {
		t.Fatalf("group = %q, want %q", cfg.groupName, defaultProxyStatisticsRedisStreamGroup)
	}
	if cfg.maxLen != defaultProxyStatisticsRedisStreamMaxLen {
		t.Fatalf("max len = %d, want %d", cfg.maxLen, defaultProxyStatisticsRedisStreamMaxLen)
	}
	if cfg.overloadPolicy != defaultProxyStatisticsStreamOverloadPolicy {
		t.Fatalf("overload policy = %q, want %q", cfg.overloadPolicy, defaultProxyStatisticsStreamOverloadPolicy)
	}
	if len(cfg.tenantOverloadPolicies) != 2 {
		t.Fatalf("tenant policies = %#v, want 2 entries", cfg.tenantOverloadPolicies)
	}
	if cfg.tenantOverloadPolicies[1] != statisticsOverloadPolicyDropNew {
		t.Fatalf("tenant 1 policy = %q, want %q", cfg.tenantOverloadPolicies[1], statisticsOverloadPolicyDropNew)
	}
	if cfg.tenantOverloadPolicies[2] != statisticsOverloadPolicyBlock {
		t.Fatalf("tenant 2 policy = %q, want %q", cfg.tenantOverloadPolicies[2], statisticsOverloadPolicyBlock)
	}
}

func TestDecodeProxyStatisticStreamMessage(t *testing.T) {
	input := domain.ProxyStatistic{
		ProxyID:      77,
		ProtocolID:   1,
		JudgeID:      3,
		Alive:        true,
		ResponseTime: 123,
		Attempt:      1,
		CreatedAt:    time.Now().UTC(),
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	msg := redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"stat": string(payload),
		},
	}

	decoded, err := decodeProxyStatisticStreamMessage(msg)
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if decoded.ProxyID != input.ProxyID || decoded.ProtocolID != input.ProtocolID || decoded.JudgeID != input.JudgeID {
		t.Fatalf("decoded stat mismatch: %#v vs %#v", decoded, input)
	}
}
