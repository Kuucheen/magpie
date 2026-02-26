package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"magpie/internal/domain"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

const (
	envProxyStatisticsRetentionDaysForPartition = "PROXY_STATISTICS_RETENTION_DAYS"
	envProxyStatisticsPartitionPrecreateMonths  = "PROXY_STATISTICS_PARTITION_PRECREATE_MONTHS"
	envProxyStatisticsPartitionPastMonths       = "PROXY_STATISTICS_PARTITION_PAST_MONTHS"
	envProxyStatisticsAutoPartitionMigration    = "PROXY_STATISTICS_AUTO_PARTITION_MIGRATION"
	defaultProxyStatisticsRetentionDaysForPart  = 30
	defaultPartitionPrecreateMonths             = 2
)

func ensureProxyStatisticsPartitionSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}
	if !db.Migrator().HasTable(&domain.ProxyStatistic{}) {
		return nil
	}
	if !isPostgresDialector(db) {
		return nil
	}

	partitioned, err := isProxyStatisticsPartitioned(db)
	if err != nil {
		return err
	}
	if !partitioned {
		if !support.GetEnvBool(envProxyStatisticsAutoPartitionMigration, true) {
			log.Warn("proxy_statistics table is not partitioned; automatic migration disabled")
			return nil
		}

		log.Warn("proxy_statistics table is not partitioned; attempting automatic migration to partitioned table")
		if err := migrateProxyStatisticsToPartitioned(db, time.Now().UTC()); err != nil {
			return err
		}
	}

	return ensureProxyStatisticsMonthlyPartitions(db, time.Now().UTC())
}

func ensureProxyStatisticsMonthlyPartitions(db *gorm.DB, now time.Time) error {
	currentMonth := monthStartUTC(now)
	retentionDays := support.GetEnvInt(envProxyStatisticsRetentionDaysForPartition, defaultProxyStatisticsRetentionDaysForPart)
	if retentionDays <= 0 {
		retentionDays = defaultProxyStatisticsRetentionDaysForPart
	}

	pastMonths := support.GetEnvInt(envProxyStatisticsPartitionPastMonths, retentionDays/30+1)
	if pastMonths < 2 {
		pastMonths = 2
	}

	precreateMonths := support.GetEnvInt(envProxyStatisticsPartitionPrecreateMonths, defaultPartitionPrecreateMonths)
	if precreateMonths < 1 {
		precreateMonths = defaultPartitionPrecreateMonths
	}

	startMonth := currentMonth.AddDate(0, -pastMonths, 0)
	endMonth := currentMonth.AddDate(0, precreateMonths, 0)

	if err := ensureProxyStatisticsDefaultPartition(db); err != nil {
		return err
	}

	for month := startMonth; !month.After(endMonth); month = month.AddDate(0, 1, 0) {
		if err := ensureProxyStatisticsMonthlyPartition(db, month); err != nil {
			return err
		}
	}

	return nil
}

func ensureProxyStatisticsDefaultPartition(db *gorm.DB) error {
	hasDefault, err := hasProxyStatisticsDefaultPartition(db)
	if err != nil {
		return err
	}
	if hasDefault {
		return nil
	}

	return ensureDefaultPartitionForTable(db, "proxy_statistics", "proxy_statistics_default")
}

func ensureProxyStatisticsMonthlyPartition(db *gorm.DB, month time.Time) error {
	return ensureMonthlyPartitionForTable(db, "proxy_statistics", "proxy_statistics", month)
}

func migrateProxyStatisticsToPartitioned(db *gorm.DB, now time.Time) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}
	if !isPostgresDialector(db) {
		return nil
	}

	tx := db.Begin(&sql.TxOptions{})
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback().Error
		}
	}()

	if err := tx.Exec(`LOCK TABLE proxy_statistics IN ACCESS EXCLUSIVE MODE`).Error; err != nil {
		return err
	}

	partitioned, err := isProxyStatisticsPartitioned(tx)
	if err != nil {
		return err
	}
	if partitioned {
		if err := tx.Commit().Error; err != nil {
			return err
		}
		tx = nil
		return nil
	}

	const stagingTable = "proxy_statistics_partitioned_new"
	if err := tx.Exec(`DROP TABLE IF EXISTS ` + quoteIdentifier(stagingTable)).Error; err != nil {
		return err
	}

	createStmt := `
CREATE TABLE ` + quoteIdentifier(stagingTable) + ` (
	LIKE proxy_statistics INCLUDING DEFAULTS INCLUDING IDENTITY INCLUDING GENERATED INCLUDING STORAGE INCLUDING COMMENTS,
	PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at)
`
	if err := tx.Exec(createStmt).Error; err != nil {
		return err
	}

	if err := addProxyStatisticsFKConstraints(tx, stagingTable); err != nil {
		return err
	}

	ranges, err := proxyStatisticsPartitionRange(tx, now)
	if err != nil {
		return err
	}

	if err := ensureDefaultPartitionForTable(tx, stagingTable, "proxy_statistics_default"); err != nil {
		return err
	}
	for month := ranges.start; !month.After(ranges.end); month = month.AddDate(0, 1, 0) {
		// Use final partition names so post-rename startup doesn't try to create
		// another table for an already-covered range.
		if err := ensureMonthlyPartitionForTable(tx, stagingTable, "proxy_statistics", month); err != nil {
			return err
		}
	}

	if err := tx.Exec(`INSERT INTO ` + quoteIdentifier(stagingTable) + ` SELECT * FROM proxy_statistics ORDER BY id`).Error; err != nil {
		return err
	}

	if err := tx.Exec(`DROP TABLE proxy_statistics`).Error; err != nil {
		return err
	}
	if err := tx.Exec(`ALTER TABLE ` + quoteIdentifier(stagingTable) + ` RENAME TO proxy_statistics`).Error; err != nil {
		return err
	}
	if err := reseedProxyStatisticsIDSequence(tx, "proxy_statistics"); err != nil {
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	tx = nil
	log.Info("proxy_statistics migration to partitioned table completed")
	return nil
}

func addProxyStatisticsFKConstraints(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}

	stmts := []string{
		fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (protocol_id) REFERENCES protocols(id) ON UPDATE CASCADE ON DELETE SET NULL`, quoteIdentifier(table), quoteIdentifier(table+"_protocol_fk")),
		fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (level_id) REFERENCES anonymity_levels(id) ON UPDATE CASCADE ON DELETE SET NULL`, quoteIdentifier(table), quoteIdentifier(table+"_level_fk")),
		fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON UPDATE CASCADE ON DELETE CASCADE`, quoteIdentifier(table), quoteIdentifier(table+"_proxy_fk")),
		fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (judge_id) REFERENCES judges(id) ON UPDATE CASCADE ON DELETE CASCADE`, quoteIdentifier(table), quoteIdentifier(table+"_judge_fk")),
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

type partitionRange struct {
	start time.Time
	end   time.Time
}

func proxyStatisticsPartitionRange(db *gorm.DB, now time.Time) (partitionRange, error) {
	currentMonth := monthStartUTC(now)
	retentionDays := support.GetEnvInt(envProxyStatisticsRetentionDaysForPartition, defaultProxyStatisticsRetentionDaysForPart)
	if retentionDays <= 0 {
		retentionDays = defaultProxyStatisticsRetentionDaysForPart
	}
	pastMonths := support.GetEnvInt(envProxyStatisticsPartitionPastMonths, retentionDays/30+1)
	if pastMonths < 2 {
		pastMonths = 2
	}
	precreateMonths := support.GetEnvInt(envProxyStatisticsPartitionPrecreateMonths, defaultPartitionPrecreateMonths)
	if precreateMonths < 1 {
		precreateMonths = defaultPartitionPrecreateMonths
	}

	start := currentMonth.AddDate(0, -pastMonths, 0)
	end := currentMonth.AddDate(0, precreateMonths, 0)

	var minCreated sql.NullTime
	if err := db.Raw(`SELECT MIN(created_at) FROM proxy_statistics`).Scan(&minCreated).Error; err != nil {
		return partitionRange{}, err
	}
	if minCreated.Valid {
		minMonth := monthStartUTC(minCreated.Time)
		if minMonth.Before(start) {
			start = minMonth
		}
	}

	return partitionRange{start: start, end: end}, nil
}

func ensureDefaultPartitionForTable(db *gorm.DB, parent string, name string) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s DEFAULT`, quoteIdentifier(name), quoteIdentifier(parent))
	return db.Exec(stmt).Error
}

func ensureMonthlyPartitionForTable(db *gorm.DB, parent string, partitionPrefix string, month time.Time) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}

	start := monthStartUTC(month)
	end := start.AddDate(0, 1, 0)

	exists, err := hasPartitionForRange(db, parent, start, end)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	name := partitionPrefix + "_" + strconv.Itoa(start.Year()) + "_" + fmt.Sprintf("%02d", int(start.Month()))
	stmt := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		quoteIdentifier(name),
		quoteIdentifier(parent),
		start.Format("2006-01-02 15:04:05-07"),
		end.Format("2006-01-02 15:04:05-07"),
	)
	return db.Exec(stmt).Error
}

func hasPartitionForRange(db *gorm.DB, parent string, start time.Time, end time.Time) (bool, error) {
	var exists bool
	query := `
SELECT EXISTS (
	SELECT 1
	FROM pg_inherits i
	JOIN pg_class p ON p.oid = i.inhparent
	JOIN pg_class c ON c.oid = i.inhrelid
	JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE p.relname = ?
	  AND n.nspname = current_schema()
	  AND pg_get_expr(c.relpartbound, c.oid) <> 'DEFAULT'
	  AND (regexp_match(pg_get_expr(c.relpartbound, c.oid), $$FROM \('([^']+)'\)$$))[1]::timestamptz = ?
	  AND (regexp_match(pg_get_expr(c.relpartbound, c.oid), $$TO \('([^']+)'\)$$))[1]::timestamptz = ?
)`
	if err := db.Raw(query, parent, start.UTC(), end.UTC()).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

func reseedProxyStatisticsIDSequence(db *gorm.DB, table string) error {
	if db == nil {
		return fmt.Errorf("nil database connection")
	}

	var sequence sql.NullString
	if err := db.Raw(`SELECT pg_get_serial_sequence(?, 'id')`, table).Scan(&sequence).Error; err != nil {
		return err
	}
	if !sequence.Valid || strings.TrimSpace(sequence.String) == "" {
		return nil
	}

	var maxID sql.NullInt64
	if err := db.Raw(`SELECT MAX(id) FROM ` + quoteIdentifier(table)).Scan(&maxID).Error; err != nil {
		return err
	}
	if maxID.Valid && maxID.Int64 > 0 {
		return db.Exec(`SELECT setval(?::regclass, ?, true)`, sequence.String, maxID.Int64).Error
	}

	return db.Exec(`SELECT setval(?::regclass, 1, false)`, sequence.String).Error
}

func isProxyStatisticsPartitioned(db *gorm.DB) (bool, error) {
	var partitioned bool
	query := `
SELECT c.relkind = 'p' AS partitioned
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema()
  AND c.relname = 'proxy_statistics'
LIMIT 1`

	if err := db.Raw(query).Scan(&partitioned).Error; err != nil {
		return false, err
	}
	return partitioned, nil
}

func hasProxyStatisticsDefaultPartition(db *gorm.DB) (bool, error) {
	var hasDefault bool
	query := `
SELECT EXISTS (
	SELECT 1
	FROM pg_inherits i
	JOIN pg_class child ON child.oid = i.inhrelid
	JOIN pg_class parent ON parent.oid = i.inhparent
	WHERE parent.relname = 'proxy_statistics'
	  AND pg_get_expr(child.relpartbound, child.oid) = 'DEFAULT'
)`

	if err := db.Raw(query).Scan(&hasDefault).Error; err != nil {
		return false, err
	}
	return hasDefault, nil
}

type proxyStatisticsPartitionInfo struct {
	Name       string
	UpperBound time.Time
}

func DropProxyStatisticsPartitionsOlderThan(ctx context.Context, olderThan time.Time, maxDrops int) (int, error) {
	if DB == nil || olderThan.IsZero() {
		return 0, nil
	}
	if maxDrops <= 0 {
		maxDrops = 1
	}

	db := DB
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	if !isPostgresDialector(db) {
		return 0, nil
	}

	partitioned, err := isProxyStatisticsPartitioned(db)
	if err != nil || !partitioned {
		return 0, err
	}

	partitions, err := listProxyStatisticsPartitions(db)
	if err != nil {
		return 0, err
	}

	dropped := 0
	for _, partition := range partitions {
		if partition.UpperBound.After(olderThan) {
			continue
		}
		if partition.Name == "proxy_statistics_default" {
			continue
		}

		hasLatest, err := partitionHasLatestStatisticReferences(db, partition.Name)
		if err != nil {
			return dropped, err
		}
		if hasLatest {
			continue
		}

		if err := db.Exec(`DROP TABLE IF EXISTS ` + quoteIdentifier(partition.Name)).Error; err != nil {
			return dropped, err
		}
		dropped++
		if dropped >= maxDrops {
			break
		}
	}

	return dropped, nil
}

func listProxyStatisticsPartitions(db *gorm.DB) ([]proxyStatisticsPartitionInfo, error) {
	var rows []struct {
		PartitionName string
		UpperBoundRaw sql.NullString
	}

	query := `
SELECT
	child.relname AS partition_name,
	(regexp_match(pg_get_expr(child.relpartbound, child.oid), $$TO \('([^']+)'\)$$))[1] AS upper_bound
FROM pg_inherits i
JOIN pg_class parent ON parent.oid = i.inhparent
JOIN pg_class child ON child.oid = i.inhrelid
JOIN pg_namespace ns ON ns.oid = child.relnamespace
WHERE parent.relname = 'proxy_statistics'
  AND ns.nspname = current_schema()
  AND pg_get_expr(child.relpartbound, child.oid) <> 'DEFAULT'
ORDER BY upper_bound ASC`

	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]proxyStatisticsPartitionInfo, 0, len(rows))
	for _, row := range rows {
		if !row.UpperBoundRaw.Valid {
			continue
		}
		parsed, err := time.Parse("2006-01-02 15:04:05-07", row.UpperBoundRaw.String)
		if err != nil {
			return nil, err
		}
		out = append(out, proxyStatisticsPartitionInfo{
			Name:       row.PartitionName,
			UpperBound: parsed.UTC(),
		})
	}

	return out, nil
}

func partitionHasLatestStatisticReferences(db *gorm.DB, partition string) (bool, error) {
	query := fmt.Sprintf(`
SELECT EXISTS (
	SELECT 1
	FROM proxy_latest_statistics pls
	JOIN %s part ON part.id = pls.statistic_id
	LIMIT 1
)`, quoteIdentifier(partition))

	var exists bool
	if err := db.Raw(query).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

func monthStartUTC(ts time.Time) time.Time {
	return time.Date(ts.UTC().Year(), ts.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
}

func isPostgresDialector(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(db.Dialector.Name()))
	return strings.Contains(name, "postgres")
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
