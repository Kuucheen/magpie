package checker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"magpie/internal/database"
	"magpie/internal/domain"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

const (
	failureBatchWindow       = 50 * time.Millisecond
	failureBatchMaxItems     = 8192
	failureSQLChunkSize      = 200
	failureQueryTimeout      = 5 * time.Second
	failureProcessingTimeout = 10 * time.Second
)

type failureEvent struct {
	UserID            uint
	Success           bool
	AutoRemove        bool
	FailureThreshold  uint8
	HasEligibleChecks bool
}

type failureResponse struct {
	removedUsers map[uint]struct{}
	orphaned     []domain.Proxy
	err          error
}

type failureRequest struct {
	ctx     context.Context
	proxyID uint64
	users   []failureEvent
	resp    chan failureResponse
}

type failureTracker struct {
	requests chan *failureRequest
}

type failureKey struct {
	userID  uint
	proxyID uint64
}

type failureIncrementEntry struct {
	request    *failureRequest
	userID     uint
	proxyID    uint64
	autoRemove bool
	threshold  uint8
}

type failurePair struct {
	userID  uint
	proxyID uint64
}

var trackerInstance = newFailureTracker()

func init() {
	go trackerInstance.run()
}

func processFailureEvents(ctx context.Context, proxyID uint64, users []failureEvent) (map[uint]struct{}, []domain.Proxy, error) {
	return trackerInstance.Submit(ctx, proxyID, users)
}

func newFailureTracker() *failureTracker {
	return &failureTracker{
		requests: make(chan *failureRequest, 1024),
	}
}

func (ft *failureTracker) Submit(ctx context.Context, proxyID uint64, users []failureEvent) (map[uint]struct{}, []domain.Proxy, error) {
	if len(users) == 0 {
		return nil, nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req := &failureRequest{
		ctx:     ctx,
		proxyID: proxyID,
		users:   users,
		resp:    make(chan failureResponse, 1),
	}

	select {
	case ft.requests <- req:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	select {
	case res := <-req.resp:
		return res.removedUsers, res.orphaned, res.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (ft *failureTracker) run() {
	batch := make([]*failureRequest, 0, failureBatchMaxItems)
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		if len(batch) == 0 {
			return
		}
		items := make([]*failureRequest, len(batch))
		copy(items, batch)
		batch = batch[:0]
		ft.processBatch(items)
	}

	for {
		select {
		case req := <-ft.requests:
			batch = append(batch, req)
			if len(batch) >= failureBatchMaxItems {
				if timer != nil {
					if !timer.Stop() {
						<-timer.C
					}
					timer = nil
					timerC = nil
				}
				flush()
				continue
			}
			if timer == nil {
				timer = time.NewTimer(failureBatchWindow)
				timerC = timer.C
			}
		case <-timerC:
			flush()
			timer = nil
			timerC = nil
		}
	}
}

func (ft *failureTracker) processBatch(batch []*failureRequest) {
	active := make([]*failureRequest, 0, len(batch))
	for _, req := range batch {
		if req == nil {
			continue
		}
		if req.ctx == nil {
			req.ctx = context.Background()
		}
		select {
		case <-req.ctx.Done():
			req.respond(failureResponse{err: req.ctx.Err()})
		default:
			active = append(active, req)
		}
	}

	if len(active) == 0 {
		return
	}

	resetCandidates := make(map[uint][]uint64)
	var increments []failureIncrementEntry

	for _, req := range active {
		for _, user := range req.users {
			if !user.HasEligibleChecks {
				continue
			}
			if user.Success {
				resetCandidates[user.UserID] = append(resetCandidates[user.UserID], req.proxyID)
				continue
			}
			increments = append(increments, failureIncrementEntry{
				request:    req,
				userID:     user.UserID,
				proxyID:    req.proxyID,
				autoRemove: user.AutoRemove,
				threshold:  user.FailureThreshold,
			})
		}
	}

	if err := applyFailureResets(resetCandidates); err != nil {
		log.Error("reset proxy failure streak", "error", err)
		ft.failBatch(active, err)
		return
	}

	counts, err := applyFailureIncrements(increments)
	if err != nil {
		log.Error("increment proxy failure streak", "error", err)
		ft.failBatch(active, err)
		return
	}

	counts, err = ensureFailureCounts(counts, increments)
	if err != nil {
		log.Error("fetch proxy failure streaks", "error", err)
		ft.failBatch(active, err)
		return
	}

	responses := make(map[*failureRequest]*failureResponse, len(active))
	for _, req := range active {
		responses[req] = &failureResponse{}
	}

	for _, entry := range increments {
		if !entry.autoRemove || entry.threshold == 0 {
			continue
		}
		count := counts[failureKey{userID: entry.userID, proxyID: entry.proxyID}]
		if count == 0 || count < uint16(entry.threshold) {
			continue
		}
		_, orphaned, removeErr := database.DeleteProxyRelation(entry.userID, []int{int(entry.proxyID)})
		if removeErr != nil {
			log.Error("auto-remove proxy", "proxy_id", entry.proxyID, "user_id", entry.userID, "error", removeErr)
			ft.failBatch(active, removeErr)
			return
		}
		log.Info("auto-removing proxy after repeated failures", "proxy_id", entry.proxyID, "user_id", entry.userID, "failures", count)
		resp := responses[entry.request]
		resp.markRemoved(entry.userID)
		if len(orphaned) > 0 {
			resp.orphaned = append(resp.orphaned, orphaned...)
		}
	}

	for _, req := range active {
		req.respond(*responses[req])
	}
}

func (ft *failureTracker) failBatch(batch []*failureRequest, err error) {
	for _, req := range batch {
		if req == nil {
			continue
		}
		req.respond(failureResponse{err: err})
	}
}

func (req *failureRequest) respond(resp failureResponse) {
	select {
	case req.resp <- resp:
	case <-req.ctx.Done():
	default:
	}
}

func (resp *failureResponse) markRemoved(userID uint) {
	if resp.removedUsers == nil {
		resp.removedUsers = make(map[uint]struct{})
	}
	resp.removedUsers[userID] = struct{}{}
}

func applyFailureResets(candidates map[uint][]uint64) error {
	if len(candidates) == 0 {
		return nil
	}
	if database.DB == nil {
		return fmt.Errorf("database not initialised")
	}

	pairs := make([]failurePair, 0)
	for userID, proxies := range candidates {
		for _, proxyID := range proxies {
			pairs = append(pairs, failurePair{userID: userID, proxyID: proxyID})
		}
	}

	for start := 0; start < len(pairs); start += failureSQLChunkSize {
		end := start + failureSQLChunkSize
		if end > len(pairs) {
			end = len(pairs)
		}
		if err := execResetChunk(pairs[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func execResetChunk(pairs []failurePair) error {
	if len(pairs) == 0 {
		return nil
	}

	db := database.DB
	if db == nil {
		return fmt.Errorf("database not initialised")
	}

	if !isPostgresDialect(db) {
		for _, pair := range pairs {
			if err := db.Model(&domain.UserProxy{}).
				Where("user_id = ? AND proxy_id = ?", pair.userID, pair.proxyID).
				Update("consecutive_failures", 0).Error; err != nil {
				return err
			}
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), failureQueryTimeout)
	defer cancel()

	var builder strings.Builder
	args := make([]any, 0, len(pairs)*2)
	builder.WriteString("WITH targets(user_id, proxy_id) AS (VALUES ")
	argPos := 1
	for i, p := range pairs {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(fmt.Sprintf("($%d::bigint,$%d::bigint)", argPos, argPos+1))
		args = append(args, p.userID, p.proxyID)
		argPos += 2
	}
	builder.WriteString(")\nUPDATE user_proxies up SET consecutive_failures = 0 FROM targets WHERE up.user_id = targets.user_id AND up.proxy_id = targets.proxy_id")

	return db.WithContext(ctx).Exec(builder.String(), args...).Error
}

func applyFailureIncrements(entries []failureIncrementEntry) (map[failureKey]uint16, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	if database.DB == nil {
		return nil, fmt.Errorf("database not initialised")
	}

	counts := make(map[failureKey]uint16, len(entries))
	for start := 0; start < len(entries); start += failureSQLChunkSize {
		end := start + failureSQLChunkSize
		if end > len(entries) {
			end = len(entries)
		}
		chunk := entries[start:end]
		chunkCounts, err := execIncrementChunk(chunk)
		if err != nil {
			return nil, err
		}
		for key, val := range chunkCounts {
			counts[key] = val
		}
	}
	return counts, nil
}

func execIncrementChunk(entries []failureIncrementEntry) (map[failureKey]uint16, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	db := database.DB
	if db == nil {
		return nil, fmt.Errorf("database not initialised")
	}
	if !isPostgresDialect(db) {
		return execIncrementChunkFallback(db, entries)
	}

	ctx, cancel := context.WithTimeout(context.Background(), failureQueryTimeout)
	defer cancel()

	var builder strings.Builder
	args := make([]any, 0, len(entries)*3)
	builder.WriteString("WITH updates(user_id, proxy_id, inc) AS (VALUES ")
	argPos := 1
	for i, entry := range entries {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(fmt.Sprintf("($%d::bigint,$%d::bigint,$%d::integer)", argPos, argPos+1, argPos+2))
		args = append(args, entry.userID, entry.proxyID, 1)
		argPos += 3
	}
	builder.WriteString(")\nUPDATE user_proxies up SET consecutive_failures = LEAST(up.consecutive_failures + updates.inc, 65535) FROM updates WHERE up.user_id = updates.user_id AND up.proxy_id = updates.proxy_id RETURNING up.user_id, up.proxy_id, up.consecutive_failures")

	var rows []struct {
		UserID              uint
		ProxyID             uint64
		ConsecutiveFailures uint16 `gorm:"column:consecutive_failures"`
	}

	if err := db.WithContext(ctx).Raw(builder.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[failureKey]uint16, len(rows))
	for _, row := range rows {
		result[failureKey{userID: row.UserID, proxyID: row.ProxyID}] = row.ConsecutiveFailures
	}
	return result, nil
}

func execIncrementChunkFallback(db *gorm.DB, entries []failureIncrementEntry) (map[failureKey]uint16, error) {
	result := make(map[failureKey]uint16, len(entries))
	for _, entry := range entries {
		var pair domain.UserProxy
		err := db.
			Select("consecutive_failures").
			Where("user_id = ? AND proxy_id = ?", entry.userID, entry.proxyID).
			First(&pair).Error
		if err != nil {
			return nil, err
		}
		newVal := pair.ConsecutiveFailures + 1
		if newVal > 65535 {
			newVal = 65535
		}
		if err := db.Model(&domain.UserProxy{}).
			Where("user_id = ? AND proxy_id = ?", entry.userID, entry.proxyID).
			Update("consecutive_failures", newVal).Error; err != nil {
			return nil, err
		}
		result[failureKey{userID: entry.userID, proxyID: entry.proxyID}] = newVal
	}
	return result, nil
}

func ensureFailureCounts(counts map[failureKey]uint16, entries []failureIncrementEntry) (map[failureKey]uint16, error) {
	if len(entries) == 0 {
		return counts, nil
	}

	missingSet := make(map[failureKey]struct{})
	for _, entry := range entries {
		if !entry.autoRemove || entry.threshold == 0 {
			continue
		}
		key := failureKey{userID: entry.userID, proxyID: entry.proxyID}
		if counts[key] != 0 {
			continue
		}
		missingSet[key] = struct{}{}
	}

	if len(missingSet) == 0 {
		return counts, nil
	}

	keys := make([]failureKey, 0, len(missingSet))
	for key := range missingSet {
		keys = append(keys, key)
	}

	fetched, err := fetchFailureCounts(keys)
	if err != nil {
		return nil, err
	}
	for key, val := range fetched {
		counts[key] = val
	}

	return counts, nil
}

func fetchFailureCounts(keys []failureKey) (map[failureKey]uint16, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	db := database.DB
	if db == nil {
		return nil, fmt.Errorf("database not initialised")
	}
	if !isPostgresDialect(db) {
		return fetchFailureCountsFallback(db, keys)
	}

	ctx, cancel := context.WithTimeout(context.Background(), failureQueryTimeout)
	defer cancel()

	var builder strings.Builder
	args := make([]any, 0, len(keys)*2)
	builder.WriteString("SELECT user_id, proxy_id, consecutive_failures FROM user_proxies WHERE (user_id, proxy_id) IN (")
	for i, key := range keys {
		if i > 0 {
			builder.WriteString(",")
		}
		argPos := i*2 + 1
		builder.WriteString(fmt.Sprintf("($%d::bigint,$%d::bigint)", argPos, argPos+1))
		args = append(args, key.userID, key.proxyID)
	}
	builder.WriteString(")")

	var rows []struct {
		UserID              uint
		ProxyID             uint64
		ConsecutiveFailures uint16 `gorm:"column:consecutive_failures"`
	}

	if err := db.WithContext(ctx).Raw(builder.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[failureKey]uint16, len(rows))
	for _, row := range rows {
		result[failureKey{userID: row.UserID, proxyID: row.ProxyID}] = row.ConsecutiveFailures
	}

	return result, nil
}

func fetchFailureCountsFallback(db *gorm.DB, keys []failureKey) (map[failureKey]uint16, error) {
	result := make(map[failureKey]uint16, len(keys))
	for _, key := range keys {
		var pair domain.UserProxy
		err := db.
			Select("consecutive_failures").
			Where("user_id = ? AND proxy_id = ?", key.userID, key.proxyID).
			First(&pair).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[key] = pair.ConsecutiveFailures
	}
	return result, nil
}

func isPostgresDialect(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	name := strings.ToLower(db.Dialector.Name())
	return name == "postgres" || name == "postgresql"
}
