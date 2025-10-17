package database

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/domain"
	"magpie/internal/security"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRotatingProxyTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	return setupRotatingProxyTestDBWithDSN(t, dsn)
}

func setupRotatingProxyTestDBWithDSN(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	t.Setenv("PROXY_ENCRYPTION_KEY", "rotating-proxy-test-key")
	security.ResetProxyCipherForTests()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		t.Fatalf("set busy timeout: %v", err)
	}

	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Proxy{},
		&domain.UserProxy{},
		&domain.RotatingProxy{},
		&domain.ProxyStatistic{},
		&domain.Protocol{},
		&domain.Judge{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	DB = db

	t.Cleanup(func() {
		DB = nil
	})

	return db
}

func TestGetNextRotatingProxy_RotatesAcrossAliveProxies(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:         "rotator@example.com",
		Password:      "password123",
		HTTPProtocol:  true,
		HTTPSProtocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxies := []domain.Proxy{
		{
			IP:            "10.0.0.1",
			Port:          8080,
			Username:      "user-one",
			Password:      "pass-one",
			Country:       "AA",
			EstimatedType: "residential",
		},
		{
			IP:            "10.0.0.2",
			Port:          8081,
			Username:      "user-two",
			Password:      "pass-two",
			Country:       "AA",
			EstimatedType: "residential",
		},
	}

	for idx := range proxies {
		if err := db.Create(&proxies[idx]).Error; err != nil {
			t.Fatalf("create proxy %d: %v", idx, err)
		}
		if err := db.Create(&domain.UserProxy{
			UserID:  user.ID,
			ProxyID: proxies[idx].ID,
		}).Error; err != nil {
			t.Fatalf("link proxy %d: %v", idx, err)
		}
		stat := domain.ProxyStatistic{
			Alive:        true,
			Attempt:      1,
			ResponseTime: 150,
			ProtocolID:   protocol.ID,
			ProxyID:      proxies[idx].ID,
			JudgeID:      judge.ID,
			CreatedAt:    time.Unix(int64(idx+1), 0),
		}
		if err := db.Create(&stat).Error; err != nil {
			t.Fatalf("create proxy statistic %d: %v", idx, err)
		}
	}

	rotator := domain.RotatingProxy{
		UserID:     user.ID,
		Name:       "test-rotator",
		ProtocolID: protocol.ID,
		ListenPort: 10500,
	}
	if err := db.Create(&rotator).Error; err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}

	first, err := GetNextRotatingProxy(user.ID, rotator.ID)
	if err != nil {
		t.Fatalf("GetNextRotatingProxy first call: %v", err)
	}
	if first.ProxyID != proxies[0].ID {
		t.Fatalf("first proxy id = %d, want %d", first.ProxyID, proxies[0].ID)
	}
	if first.Protocol != protocol.Name {
		t.Fatalf("first protocol = %q, want %q", first.Protocol, protocol.Name)
	}

	var updated domain.RotatingProxy
	if err := db.First(&updated, rotator.ID).Error; err != nil {
		t.Fatalf("reload rotating proxy: %v", err)
	}
	if updated.LastProxyID == nil || *updated.LastProxyID != proxies[0].ID {
		t.Fatalf("last proxy id = %v, want %d", updated.LastProxyID, proxies[0].ID)
	}
	if updated.LastRotationAt == nil {
		t.Fatal("expected last rotation timestamp to be set")
	}

	second, err := GetNextRotatingProxy(user.ID, rotator.ID)
	if err != nil {
		t.Fatalf("GetNextRotatingProxy second call: %v", err)
	}
	if second.ProxyID != proxies[1].ID {
		t.Fatalf("second proxy id = %d, want %d", second.ProxyID, proxies[1].ID)
	}

	if err := db.First(&updated, rotator.ID).Error; err != nil {
		t.Fatalf("reload rotating proxy after second call: %v", err)
	}
	if updated.LastProxyID == nil || *updated.LastProxyID != proxies[1].ID {
		t.Fatalf("last proxy id after second call = %v, want %d", updated.LastProxyID, proxies[1].ID)
	}
}

func TestGetNextRotatingProxy_NoAliveProxies(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:         "noalive@example.com",
		Password:      "password123",
		HTTPProtocol:  true,
		HTTPSProtocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxy := domain.Proxy{
		IP:            "10.0.0.3",
		Port:          8082,
		Username:      "user-three",
		Password:      "pass-three",
		Country:       "AA",
		EstimatedType: "residential",
	}
	if err := db.Create(&proxy).Error; err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	if err := db.Create(&domain.UserProxy{
		UserID:  user.ID,
		ProxyID: proxy.ID,
	}).Error; err != nil {
		t.Fatalf("link proxy: %v", err)
	}

	stat := domain.ProxyStatistic{
		Alive:        false,
		Attempt:      1,
		ResponseTime: 200,
		ProtocolID:   protocol.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&stat).Error; err != nil {
		t.Fatalf("create proxy statistic: %v", err)
	}

	rotator := domain.RotatingProxy{
		UserID:     user.ID,
		Name:       "noalive-rotator",
		ProtocolID: protocol.ID,
		ListenPort: 10600,
	}
	if err := db.Create(&rotator).Error; err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}

	if _, err := GetNextRotatingProxy(user.ID, rotator.ID); err != ErrRotatingProxyNoAliveProxies {
		t.Fatalf("expected ErrRotatingProxyNoAliveProxies, got %v", err)
	}
}

func TestGetNextRotatingProxy_ConcurrentStress(t *testing.T) {
	tempDir := t.TempDir()
	dsn := fmt.Sprintf(
		"file:%s?mode=rwc&_journal=WAL&_fk=1&_busy_timeout=5000&_synchronous=NORMAL",
		filepath.Join(tempDir, "stress.db"),
	)
	db := setupRotatingProxyTestDBWithDSN(t, dsn)

	user := domain.User{
		Email:         "stress@example.com",
		Password:      "password123",
		HTTPProtocol:  true,
		HTTPSProtocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	const proxyCount = 50
	proxies := make([]domain.Proxy, proxyCount)
	for i := 0; i < proxyCount; i++ {
		proxies[i] = domain.Proxy{
			IP:            fmt.Sprintf("10.0.1.%d", i+1),
			Port:          uint16(9000 + i),
			Username:      fmt.Sprintf("user-%d", i),
			Password:      fmt.Sprintf("pass-%d", i),
			Country:       "AA",
			EstimatedType: "residential",
		}
		if err := db.Create(&proxies[i]).Error; err != nil {
			t.Fatalf("create proxy %d: %v", i, err)
		}
		if err := db.Create(&domain.UserProxy{
			UserID:  user.ID,
			ProxyID: proxies[i].ID,
		}).Error; err != nil {
			t.Fatalf("link proxy %d: %v", i, err)
		}
		stat := domain.ProxyStatistic{
			Alive:        true,
			Attempt:      1,
			ResponseTime: 120,
			ProtocolID:   protocol.ID,
			ProxyID:      proxies[i].ID,
			JudgeID:      judge.ID,
			CreatedAt:    time.Unix(int64(i+1), 0),
		}
		if err := db.Create(&stat).Error; err != nil {
			t.Fatalf("create proxy statistic %d: %v", i, err)
		}
	}

	rotator := domain.RotatingProxy{
		UserID:     user.ID,
		Name:       "stress-rotator",
		ProtocolID: protocol.ID,
		ListenPort: 10700,
	}
	if err := db.Create(&rotator).Error; err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}

	const goroutines = 4
	const iterations = 200

	counts := make(map[uint64]int)
	var countsMu sync.Mutex

	var stop atomic.Bool
	var firstErr atomic.Value // stores error

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations && !stop.Load(); {
				const maxAttempts = 20
				var (
					next *dto.RotatingProxyNext
					err  error
				)
				for attempt := 0; attempt < maxAttempts; attempt++ {
					next, err = GetNextRotatingProxy(user.ID, rotator.ID)
					if err == nil || !isSQLiteLocked(err) {
						break
					}
					time.Sleep(time.Millisecond * time.Duration(attempt+1))
				}
				if err != nil {
					if isSQLiteLocked(err) {
						continue
					}
					if !stop.Swap(true) {
						firstErr.Store(err)
					}
					return
				}

				countsMu.Lock()
				counts[next.ProxyID]++
				countsMu.Unlock()
				j++
			}
		}()
	}

	wg.Wait()

	if errVal := firstErr.Load(); errVal != nil {
		t.Fatalf("GetNextRotatingProxy error during stress test: %v", errVal.(error))
	}

	expectedTotal := goroutines * iterations
	var total int
	minCount := iterations * goroutines
	maxCount := 0
	for i := 0; i < proxyCount; i++ {
		count := counts[proxies[i].ID]
		if count == 0 {
			t.Fatalf("proxy %d was never selected", i)
		}
		total += count
		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
	}

	if total != expectedTotal {
		t.Fatalf("total rotations = %d, want %d", total, expectedTotal)
	}

	if diff := maxCount - minCount; diff > 2 {
		t.Fatalf("rotation distribution too uneven, max=%d min=%d", maxCount, minCount)
	}

	var updated domain.RotatingProxy
	if err := db.First(&updated, rotator.ID).Error; err != nil {
		t.Fatalf("reload rotating proxy: %v", err)
	}
	if updated.LastProxyID == nil {
		t.Fatal("expected last proxy id to be persisted after stress test")
	}
	if updated.LastRotationAt == nil {
		t.Fatal("expected last rotation timestamp to be set after stress test")
	}
}

func isSQLiteLocked(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked")
}
