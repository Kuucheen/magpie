package database

import (
	"context"
	"fmt"
	"sync/atomic"

	"magpie/internal/domain"

	"github.com/charmbracelet/log"
)

var proxyStatisticFieldCount atomic.Int32

func CalculateProxyStatisticBatchSize(statCount int) int {
	if statCount <= 0 {
		return 0
	}

	numFields := proxyStatisticColumnCount()
	if numFields <= 0 {
		return minBatchSize
	}

	maxPossibleBatchSize := maxParamsPerBatch / numFields
	if maxPossibleBatchSize < 1 {
		maxPossibleBatchSize = 1
	}

	batchSize := maxPossibleBatchSize
	if batchSize < minBatchSize {
		batchSize = minBatchSize
	}
	if batchSize > statCount {
		batchSize = statCount
	}

	return batchSize
}

func proxyStatisticColumnCount() int {
	if count := proxyStatisticFieldCount.Load(); count > 0 {
		return int(count)
	}

	if DB == nil {
		log.Error("Failed to determine proxy statistics batch size: database not initialised")
		return 0
	}

	numFields, err := getNumDatabaseFields(domain.ProxyStatistic{}, DB)
	if err != nil || numFields == 0 {
		log.Error("Failed to determine proxy statistics batch size", "error", err)
		return 0
	}

	proxyStatisticFieldCount.Store(int32(numFields))
	return numFields
}

func InsertProxyStatistics(ctx context.Context, statistics []domain.ProxyStatistic, batchSize int) error {
	if len(statistics) == 0 {
		return nil
	}

	if DB == nil {
		return fmt.Errorf("proxy statistics: database connection was not initialised")
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Errorf("Transaction rolled back due to panic: %v", r)
		}
	}()

	if err := tx.CreateInBatches(statistics, batchSize).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := updateProxyStatusCaches(tx, statistics); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
