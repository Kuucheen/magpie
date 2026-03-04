package server

import (
	"context"
	"errors"
	"testing"

	"magpie/internal/domain"
)

func TestEnqueueScrapeSitesOrRollback_EnqueueSuccess(t *testing.T) {
	originalEnqueue := enqueueScrapeSites
	originalDelete := deleteScrapeSiteRelations
	originalRemove := removeScrapeSitesFromQueue
	originalDeleteOrphans := deleteOrphanScrapeSites
	t.Cleanup(func() {
		enqueueScrapeSites = originalEnqueue
		deleteScrapeSiteRelations = originalDelete
		removeScrapeSitesFromQueue = originalRemove
		deleteOrphanScrapeSites = originalDeleteOrphans
	})

	enqueueCalled := 0
	rollbackCalled := 0
	enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
		enqueueCalled++
		return nil
	}
	deleteScrapeSiteRelations = func(userID uint, scrapeSite []int) (int64, []domain.ScrapeSite, error) {
		rollbackCalled++
		return 0, nil, nil
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error { return nil }
	deleteOrphanScrapeSites = func(_ context.Context) (int64, error) { return 0, nil }

	err := enqueueScrapeSitesOrRollback(12, []domain.ScrapeSite{{ID: 10}, {ID: 11}})
	if err != nil {
		t.Fatalf("enqueueScrapeSitesOrRollback returned error: %v", err)
	}
	if enqueueCalled != 1 {
		t.Fatalf("enqueue called %d times, want 1", enqueueCalled)
	}
	if rollbackCalled != 0 {
		t.Fatalf("rollback called %d times, want 0", rollbackCalled)
	}
}

func TestEnqueueScrapeSitesOrRollback_RollsBackOnEnqueueFailure(t *testing.T) {
	originalEnqueue := enqueueScrapeSites
	originalDelete := deleteScrapeSiteRelations
	originalRemove := removeScrapeSitesFromQueue
	originalDeleteOrphans := deleteOrphanScrapeSites
	t.Cleanup(func() {
		enqueueScrapeSites = originalEnqueue
		deleteScrapeSiteRelations = originalDelete
		removeScrapeSitesFromQueue = originalRemove
		deleteOrphanScrapeSites = originalDeleteOrphans
	})

	expectedErr := errors.New("queue unavailable")
	enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
		return expectedErr
	}

	var rollbackUserID uint
	var rollbackIDs []int
	queueCleanupCalled := 0
	orphanCleanupCalled := 0
	deleteScrapeSiteRelations = func(userID uint, scrapeSite []int) (int64, []domain.ScrapeSite, error) {
		rollbackUserID = userID
		rollbackIDs = append([]int(nil), scrapeSite...)
		return int64(len(scrapeSite)), []domain.ScrapeSite{{ID: 7, URL: "https://a.example"}}, nil
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error {
		queueCleanupCalled++
		return nil
	}
	deleteOrphanScrapeSites = func(_ context.Context) (int64, error) {
		orphanCleanupCalled++
		return 1, nil
	}

	err := enqueueScrapeSitesOrRollback(42, []domain.ScrapeSite{{ID: 7}, {ID: 8}})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected enqueue error, got: %v", err)
	}
	if rollbackUserID != 42 {
		t.Fatalf("rollback user id = %d, want 42", rollbackUserID)
	}
	if len(rollbackIDs) != 2 || rollbackIDs[0] != 7 || rollbackIDs[1] != 8 {
		t.Fatalf("rollback ids = %#v, want [7 8]", rollbackIDs)
	}
	if queueCleanupCalled != 1 {
		t.Fatalf("queue cleanup called %d times, want 1", queueCleanupCalled)
	}
	if orphanCleanupCalled != 1 {
		t.Fatalf("orphan cleanup called %d times, want 1", orphanCleanupCalled)
	}
}

func TestEnqueueScrapeSitesOrRollback_ReportsRollbackFailure(t *testing.T) {
	originalEnqueue := enqueueScrapeSites
	originalDelete := deleteScrapeSiteRelations
	originalRemove := removeScrapeSitesFromQueue
	originalDeleteOrphans := deleteOrphanScrapeSites
	t.Cleanup(func() {
		enqueueScrapeSites = originalEnqueue
		deleteScrapeSiteRelations = originalDelete
		removeScrapeSitesFromQueue = originalRemove
		deleteOrphanScrapeSites = originalDeleteOrphans
	})

	enqueueErr := errors.New("enqueue failed")
	rollbackErr := errors.New("rollback failed")
	enqueueScrapeSites = func(sites []domain.ScrapeSite) error {
		return enqueueErr
	}
	deleteScrapeSiteRelations = func(userID uint, scrapeSite []int) (int64, []domain.ScrapeSite, error) {
		return 0, nil, rollbackErr
	}
	removeScrapeSitesFromQueue = func(sites []domain.ScrapeSite) error { return nil }
	deleteOrphanScrapeSites = func(_ context.Context) (int64, error) { return 0, nil }

	err := enqueueScrapeSitesOrRollback(7, []domain.ScrapeSite{{ID: 1}})
	if err == nil {
		t.Fatal("expected combined error, got nil")
	}
	if !errors.Is(err, enqueueErr) {
		t.Fatalf("expected combined error to include enqueue error, got: %v", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected combined error to include rollback error, got: %v", err)
	}
}
