package checker

import (
	"errors"
	"testing"

	"magpie/internal/domain"
	"magpie/internal/support"
)

func TestCheckProxyWithRetries_PerformsInitialAttemptWhenRetriesZero(t *testing.T) {
	proxy := domain.Proxy{
		IP:   "127.0.0.1",
		Port: 8080,
	}
	judge := &domain.Judge{
		FullString: "://invalid-url",
	}

	html, err, _, attempt := CheckProxyWithRetries(proxy, judge, "http", support.TransportTCP, 100, 0)
	if err == nil {
		t.Fatalf("expected at least one attempt and an error for invalid judge URL, got nil error and html=%q", html)
	}
	if attempt != 0 {
		t.Fatalf("expected first attempt index 0 for retries=0, got %d", attempt)
	}
}

func TestProcessJudgeAssignments_EnqueuesOneStatisticPerNetworkCheck(t *testing.T) {
	originalCheck := checkProxyWithRetries
	originalEnqueue := enqueueProxyStatistic
	t.Cleanup(func() {
		checkProxyWithRetries = originalCheck
		enqueueProxyStatistic = originalEnqueue
	})

	checkProxyWithRetries = func(domain.Proxy, *domain.Judge, string, string, uint16, uint8) (string, error, int64, uint8) {
		return "ok", nil, 88, 1
	}

	var captured []domain.ProxyStatistic
	var capturedUserIDs [][]uint
	enqueueProxyStatistic = func(stat domain.ProxyStatistic, userIDs []uint) {
		captured = append(captured, stat)
		capturedUserIDs = append(capturedUserIDs, append([]uint(nil), userIDs...))
	}

	proxy := domain.Proxy{ID: 99}
	judge := &domain.Judge{ID: 7}
	assignments := map[string]*requestAssignment{
		"7_http_tcp": {
			judge:             judge,
			proxyProtocol:     "http",
			transportProtocol: support.TransportTCP,
			protocolID:        1,
			checks: []userCheck{
				{userID: 1, regex: "ok"},
				{userID: 2, regex: "ok"},
				{userID: 3, regex: "missing"},
			},
		},
	}
	userSuccess := map[uint]bool{
		1: false,
		2: false,
		3: false,
	}

	processJudgeAssignments(proxy, assignments, userSuccess, 1000, 2, false)

	if len(captured) != 1 {
		t.Fatalf("expected 1 proxy statistic, got %d", len(captured))
	}
	stat := captured[0]
	if stat.ProxyID != proxy.ID {
		t.Fatalf("unexpected proxy id = %d, want %d", stat.ProxyID, proxy.ID)
	}
	if stat.JudgeID != judge.ID {
		t.Fatalf("unexpected judge id = %d, want %d", stat.JudgeID, judge.ID)
	}
	if stat.ProtocolID != 1 {
		t.Fatalf("unexpected protocol id = %d, want 1", stat.ProtocolID)
	}
	if !stat.Alive {
		t.Fatal("expected aggregated statistic to be alive when any regex matches")
	}
	if stat.LevelID == nil {
		t.Fatal("expected level id to be set for alive aggregated statistic")
	}
	if !userSuccess[1] || !userSuccess[2] || userSuccess[3] {
		t.Fatalf("unexpected user success map: %#v", userSuccess)
	}
	if len(capturedUserIDs) != 1 {
		t.Fatalf("expected 1 captured user id set, got %d", len(capturedUserIDs))
	}
	if len(capturedUserIDs[0]) != 3 {
		t.Fatalf("captured user ids = %#v, want 3 unique IDs", capturedUserIDs[0])
	}
}

func TestProcessJudgeAssignments_RecordsFailedCheckOnce(t *testing.T) {
	originalCheck := checkProxyWithRetries
	originalEnqueue := enqueueProxyStatistic
	t.Cleanup(func() {
		checkProxyWithRetries = originalCheck
		enqueueProxyStatistic = originalEnqueue
	})

	checkProxyWithRetries = func(domain.Proxy, *domain.Judge, string, string, uint16, uint8) (string, error, int64, uint8) {
		return "", errors.New("failed"), 250, 3
	}

	var captured []domain.ProxyStatistic
	enqueueProxyStatistic = func(stat domain.ProxyStatistic, _ []uint) {
		captured = append(captured, stat)
	}

	proxy := domain.Proxy{ID: 44}
	judge := &domain.Judge{ID: 9}
	assignments := map[string]*requestAssignment{
		"9_https_tcp": {
			judge:             judge,
			proxyProtocol:     "https",
			transportProtocol: support.TransportTCP,
			protocolID:        2,
			checks: []userCheck{
				{userID: 10, regex: "ok"},
				{userID: 11, regex: "ok"},
			},
		},
	}
	userSuccess := map[uint]bool{
		10: false,
		11: false,
	}

	processJudgeAssignments(proxy, assignments, userSuccess, 1000, 1, false)

	if len(captured) != 1 {
		t.Fatalf("expected 1 proxy statistic, got %d", len(captured))
	}
	stat := captured[0]
	if stat.Alive {
		t.Fatal("expected failed check statistic to be dead")
	}
	if stat.LevelID != nil {
		t.Fatal("expected level id to be nil for dead statistic")
	}
	if userSuccess[10] || userSuccess[11] {
		t.Fatalf("expected users to remain unsuccessful on failed check, got %#v", userSuccess)
	}
}
