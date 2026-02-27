package runtime

import (
	"testing"
	"time"

	"magpie/internal/domain"
)

func TestAddProxyStatistic_EvictsOldestWhenQueueIsFull(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastSaturationLog.Load()
	originalDropped := proxyStatisticDroppedCount.Load()
	originalEvicted := proxyStatisticEvictedCount.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastSaturationLog.Store(originalLastLog)
		proxyStatisticDroppedCount.Store(originalDropped)
		proxyStatisticEvictedCount.Store(originalEvicted)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastSaturationLog.Store(time.Now().Unix())
	proxyStatisticDroppedCount.Store(0)
	proxyStatisticEvictedCount.Store(0)
	proxyStatisticQueue <- domain.ProxyStatistic{ProxyID: 1}

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 2})

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
	stat := <-proxyStatisticQueue
	if stat.ProxyID != 2 {
		t.Fatalf("unexpected queue entry ProxyID = %d, want %d", stat.ProxyID, 2)
	}
	if got := proxyStatisticEvictedCount.Load(); got != 1 {
		t.Fatalf("evicted count = %d, want 1", got)
	}
	if got := proxyStatisticDroppedCount.Load(); got != 0 {
		t.Fatalf("dropped count = %d, want 0", got)
	}
}

func TestAddProxyStatistic_EnqueuesWhenCapacityAvailable(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastSaturationLog.Load()
	originalDropped := proxyStatisticDroppedCount.Load()
	originalEvicted := proxyStatisticEvictedCount.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastSaturationLog.Store(originalLastLog)
		proxyStatisticDroppedCount.Store(originalDropped)
		proxyStatisticEvictedCount.Store(originalEvicted)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastSaturationLog.Store(time.Now().Unix())
	proxyStatisticDroppedCount.Store(0)
	proxyStatisticEvictedCount.Store(0)

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 42})

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
	if got := proxyStatisticDroppedCount.Load(); got != 0 {
		t.Fatalf("dropped count = %d, want 0", got)
	}
	if got := proxyStatisticEvictedCount.Load(); got != 0 {
		t.Fatalf("evicted count = %d, want 0", got)
	}
}

func TestAddProxyStatistic_DropsCounterAccumulates(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastSaturationLog.Load()
	originalDropped := proxyStatisticDroppedCount.Load()
	originalEvicted := proxyStatisticEvictedCount.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastSaturationLog.Store(originalLastLog)
		proxyStatisticDroppedCount.Store(originalDropped)
		proxyStatisticEvictedCount.Store(originalEvicted)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic)
	proxyStatisticLastSaturationLog.Store(time.Now().Unix())
	proxyStatisticDroppedCount.Store(0)
	proxyStatisticEvictedCount.Store(0)

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 2})
	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 3})

	if got := proxyStatisticDroppedCount.Load(); got != 2 {
		t.Fatalf("dropped count = %d, want 2", got)
	}
	if got := proxyStatisticEvictedCount.Load(); got != 0 {
		t.Fatalf("evicted count = %d, want 0", got)
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
