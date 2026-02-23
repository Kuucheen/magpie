package runtime

import (
	"testing"
	"time"

	"magpie/internal/domain"
)

func TestAddProxyStatistic_DoesNotBlockWhenQueueIsFull(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalDropCount := proxyStatisticDropCount.Load()
	originalLastLog := proxyStatisticLastDropLog.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticDropCount.Store(originalDropCount)
		proxyStatisticLastDropLog.Store(originalLastLog)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticDropCount.Store(0)
	proxyStatisticLastDropLog.Store(time.Now().Unix())
	proxyStatisticQueue <- domain.ProxyStatistic{ProxyID: 1}

	done := make(chan struct{})
	go func() {
		AddProxyStatistic(domain.ProxyStatistic{ProxyID: 2})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("AddProxyStatistic blocked while queue was full")
	}

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	if dropped := proxyStatisticDropCount.Load(); dropped != 1 {
		t.Fatalf("dropped count = %d, want 1", dropped)
	}
}

func TestAddProxyStatistic_EnqueuesWhenCapacityAvailable(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalDropCount := proxyStatisticDropCount.Load()
	originalLastLog := proxyStatisticLastDropLog.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticDropCount.Store(originalDropCount)
		proxyStatisticLastDropLog.Store(originalLastLog)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticDropCount.Store(0)
	proxyStatisticLastDropLog.Store(time.Now().Unix())

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 42})

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	if dropped := proxyStatisticDropCount.Load(); dropped != 0 {
		t.Fatalf("dropped count = %d, want 0", dropped)
	}
}
