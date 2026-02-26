package runtime

import (
	"testing"
	"time"

	"magpie/internal/domain"
)

func TestAddProxyStatistic_BlocksUntilQueueHasCapacity(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastBackpressureLog.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastBackpressureLog.Store(originalLastLog)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastBackpressureLog.Store(time.Now().Unix())
	proxyStatisticQueue <- domain.ProxyStatistic{ProxyID: 1}

	done := make(chan struct{})
	go func() {
		AddProxyStatistic(domain.ProxyStatistic{ProxyID: 2})
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("AddProxyStatistic returned while queue was full")
	case <-time.After(100 * time.Millisecond):
	}

	<-proxyStatisticQueue

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("AddProxyStatistic did not unblock after capacity became available")
	}

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}

	stat := <-proxyStatisticQueue
	if stat.ProxyID != 2 {
		t.Fatalf("queued ProxyID = %d, want 2", stat.ProxyID)
	}
}

func TestAddProxyStatistic_EnqueuesWhenCapacityAvailable(t *testing.T) {
	originalQueue := proxyStatisticQueue
	originalLastLog := proxyStatisticLastBackpressureLog.Load()
	t.Cleanup(func() {
		proxyStatisticQueue = originalQueue
		proxyStatisticLastBackpressureLog.Store(originalLastLog)
	})

	proxyStatisticQueue = make(chan domain.ProxyStatistic, 1)
	proxyStatisticLastBackpressureLog.Store(time.Now().Unix())

	AddProxyStatistic(domain.ProxyStatistic{ProxyID: 42})

	if got := len(proxyStatisticQueue); got != 1 {
		t.Fatalf("queue length = %d, want 1", got)
	}
}
