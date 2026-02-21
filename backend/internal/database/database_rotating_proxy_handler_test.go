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
		&domain.ProxyReputation{},
		&domain.RotatingProxy{},
		&domain.ProxyStatistic{},
		&domain.ProxyLatestStatistic{},
		&domain.ProxyOverallStatus{},
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

func TestCreateRotatingProxy_AllowsDistinctListenProtocol(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:          "combo@example.com",
		Password:       "password123",
		HTTPProtocol:   true,
		SOCKS5Protocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	httpProto := domain.Protocol{Name: "http"}
	socksProto := domain.Protocol{Name: "socks5"}
	if err := db.Create(&httpProto).Error; err != nil {
		t.Fatalf("create http protocol: %v", err)
	}
	if err := db.Create(&socksProto).Error; err != nil {
		t.Fatalf("create socks5 protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge-combo.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxy := domain.Proxy{
		IP:            "10.20.0.1",
		Port:          9050,
		Country:       "US",
		EstimatedType: "datacenter",
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
		Alive:        true,
		Attempt:      1,
		ResponseTime: 90,
		ProtocolID:   socksProto.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&stat).Error; err != nil {
		t.Fatalf("create proxy stat: %v", err)
	}
	if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
		t.Fatalf("update proxy status cache: %v", err)
	}

	payload := dto.RotatingProxyCreateRequest{
		Name:           "http-on-socks",
		Protocol:       "socks5",
		ListenProtocol: "http",
		AuthRequired:   false,
	}

	created, err := CreateRotatingProxy(user.ID, payload)
	if err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}
	if created.Protocol != socksProto.Name {
		t.Fatalf("proxy protocol = %q, want %q", created.Protocol, socksProto.Name)
	}
	if created.ListenProtocol != httpProto.Name {
		t.Fatalf("listen protocol = %q, want %q", created.ListenProtocol, httpProto.Name)
	}
	if created.AliveProxyCount != 1 {
		t.Fatalf("alive proxy count = %d, want 1", created.AliveProxyCount)
	}
	if created.ListenPort == 0 {
		t.Fatalf("expected listen port to be assigned")
	}

	stored, err := GetRotatingProxyByID(created.ID)
	if err != nil {
		t.Fatalf("reload rotating proxy: %v", err)
	}
	if stored.ProtocolID != socksProto.ID {
		t.Fatalf("stored protocol id = %d, want %d", stored.ProtocolID, socksProto.ID)
	}
	if stored.ListenProtocolID != httpProto.ID {
		t.Fatalf("stored listen protocol id = %d, want %d", stored.ListenProtocolID, httpProto.ID)
	}
	if stored.ListenProtocol.Name != httpProto.Name {
		t.Fatalf("stored listen protocol name = %q, want %q", stored.ListenProtocol.Name, httpProto.Name)
	}
}

func TestCreateRotatingProxy_ListensOnProtocolWithoutUserFlag(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:          "listen-any@example.com",
		Password:       "password123",
		HTTPProtocol:   true,
		SOCKS5Protocol: false,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	httpProto := domain.Protocol{Name: "http"}
	socksProto := domain.Protocol{Name: "socks5"}
	if err := db.Create(&httpProto).Error; err != nil {
		t.Fatalf("create http protocol: %v", err)
	}
	if err := db.Create(&socksProto).Error; err != nil {
		t.Fatalf("create socks5 protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge-listen.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxy := domain.Proxy{
		IP:            "10.30.0.1",
		Port:          8080,
		Country:       "US",
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
		Alive:        true,
		Attempt:      1,
		ResponseTime: 120,
		ProtocolID:   httpProto.ID,
		ProxyID:      proxy.ID,
		JudgeID:      judge.ID,
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&stat).Error; err != nil {
		t.Fatalf("create proxy statistic: %v", err)
	}
	if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
		t.Fatalf("update proxy status cache: %v", err)
	}

	payload := dto.RotatingProxyCreateRequest{
		Name:           "http-client-socks-listen",
		Protocol:       "http",
		ListenProtocol: "socks5",
	}

	created, err := CreateRotatingProxy(user.ID, payload)
	if err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}
	if created.ListenProtocol != socksProto.Name {
		t.Fatalf("listen protocol = %q, want %q", created.ListenProtocol, socksProto.Name)
	}
	if created.Protocol != httpProto.Name {
		t.Fatalf("protocol = %q, want %q", created.Protocol, httpProto.Name)
	}
}

func TestCreateRotatingProxy_UptimeFilterValidation(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:        "uptime-validation@example.com",
		Password:     "password123",
		HTTPProtocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	cases := []struct {
		name    string
		tp      string
		pct     *float64
		wantErr error
	}{
		{name: "missing percentage", tp: "min", pct: nil, wantErr: ErrRotatingProxyUptimeValueMissing},
		{name: "missing type", tp: "", pct: float64Ptr(80), wantErr: ErrRotatingProxyUptimeTypeMissing},
		{name: "invalid type", tp: "avg", pct: float64Ptr(80), wantErr: ErrRotatingProxyUptimeTypeInvalid},
		{name: "above max", tp: "max", pct: float64Ptr(120), wantErr: ErrRotatingProxyUptimeOutOfRange},
		{name: "below min", tp: "min", pct: float64Ptr(-1), wantErr: ErrRotatingProxyUptimeOutOfRange},
	}

	for idx, tc := range cases {
		payload := dto.RotatingProxyCreateRequest{
			Name:             fmt.Sprintf("invalid-uptime-%d", idx),
			Protocol:         "http",
			UptimeFilterType: tc.tp,
			UptimePercentage: tc.pct,
		}
		_, err := CreateRotatingProxy(user.ID, payload)
		if err != tc.wantErr {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.wantErr, err)
		}
	}

	created, err := CreateRotatingProxy(user.ID, dto.RotatingProxyCreateRequest{
		Name:             "valid-uptime-filter",
		Protocol:         "http",
		UptimeFilterType: "min",
		UptimePercentage: float64Ptr(80.04),
	})
	if err != nil {
		t.Fatalf("create rotating proxy with valid uptime filter: %v", err)
	}
	if created.UptimeFilterType != "min" {
		t.Fatalf("uptime filter type = %q, want min", created.UptimeFilterType)
	}
	if created.UptimePercentage == nil || *created.UptimePercentage != 80.0 {
		t.Fatalf("uptime percentage = %v, want 80.0", created.UptimePercentage)
	}
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
		if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
			t.Fatalf("update proxy status cache %d: %v", idx, err)
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

func TestGetNextRotatingProxy_ReputationFilterApplied(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:        "reputation@example.com",
		Password:     "password123",
		HTTPProtocol: true,
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
		{IP: "10.0.0.10", Port: 9000, Country: "AA", EstimatedType: "residential"},
		{IP: "10.0.0.11", Port: 9001, Country: "AA", EstimatedType: "residential"},
		{IP: "10.0.0.12", Port: 9002, Country: "AA", EstimatedType: "residential"},
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
			ResponseTime: 150,
			Attempt:      1,
			ProtocolID:   protocol.ID,
			ProxyID:      proxies[idx].ID,
			JudgeID:      judge.ID,
			CreatedAt:    time.Unix(int64(idx+1), 0),
		}
		if err := db.Create(&stat).Error; err != nil {
			t.Fatalf("create statistic %d: %v", idx, err)
		}
		if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
			t.Fatalf("update proxy status cache %d: %v", idx, err)
		}
	}

	reputations := []domain.ProxyReputation{
		{ProxyID: proxies[0].ID, Kind: domain.ProxyReputationKindOverall, Score: 95, Label: "good", CalculatedAt: time.Now(), UpdatedAt: time.Now()},
		{ProxyID: proxies[1].ID, Kind: domain.ProxyReputationKindOverall, Score: 75, Label: "neutral", CalculatedAt: time.Now(), UpdatedAt: time.Now()},
		{ProxyID: proxies[2].ID, Kind: domain.ProxyReputationKindOverall, Score: 25, Label: "poor", CalculatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&reputations).Error; err != nil {
		t.Fatalf("create reputations: %v", err)
	}

	rotator := domain.RotatingProxy{
		UserID:           user.ID,
		Name:             "filtered-rotator",
		ProtocolID:       protocol.ID,
		ListenPort:       10800,
		ReputationLabels: domain.StringList{"good", "neutral"},
	}
	if err := db.Create(&rotator).Error; err != nil {
		t.Fatalf("create rotating proxy: %v", err)
	}

	first, err := GetNextRotatingProxy(user.ID, rotator.ID)
	if err != nil {
		t.Fatalf("first rotation: %v", err)
	}
	if first.ProxyID != proxies[0].ID {
		t.Fatalf("first proxy id = %d, want %d", first.ProxyID, proxies[0].ID)
	}

	second, err := GetNextRotatingProxy(user.ID, rotator.ID)
	if err != nil {
		t.Fatalf("second rotation: %v", err)
	}
	if second.ProxyID != proxies[1].ID {
		t.Fatalf("second proxy id = %d, want %d", second.ProxyID, proxies[1].ID)
	}

	third, err := GetNextRotatingProxy(user.ID, rotator.ID)
	if err != nil {
		t.Fatalf("third rotation: %v", err)
	}
	if third.ProxyID != proxies[0].ID {
		t.Fatalf("third proxy id = %d, want %d", third.ProxyID, proxies[0].ID)
	}
}

func TestGetNextRotatingProxy_UptimeFilterApplied(t *testing.T) {
	db := setupRotatingProxyTestDB(t)

	user := domain.User{
		Email:        "uptime-filter@example.com",
		Password:     "password123",
		HTTPProtocol: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	protocol := domain.Protocol{Name: "http"}
	if err := db.Create(&protocol).Error; err != nil {
		t.Fatalf("create protocol: %v", err)
	}

	judge := domain.Judge{FullString: "http://judge-uptime.example.com"}
	if err := db.Create(&judge).Error; err != nil {
		t.Fatalf("create judge: %v", err)
	}

	proxies := []domain.Proxy{
		{IP: "10.0.2.10", Port: 9100, Country: "AA", EstimatedType: "residential"}, // 80%
		{IP: "10.0.2.11", Port: 9101, Country: "AA", EstimatedType: "residential"}, // 40%
		{IP: "10.0.2.12", Port: 9102, Country: "AA", EstimatedType: "residential"}, // 100%
	}
	statuses := [][]bool{
		{true, true, true, false, true},
		{false, false, false, true, true},
		{true, true, true, true, true},
	}

	createdAt := time.Unix(10, 0)
	for proxyIdx := range proxies {
		if err := db.Create(&proxies[proxyIdx]).Error; err != nil {
			t.Fatalf("create proxy %d: %v", proxyIdx, err)
		}
		if err := db.Create(&domain.UserProxy{
			UserID:  user.ID,
			ProxyID: proxies[proxyIdx].ID,
		}).Error; err != nil {
			t.Fatalf("link proxy %d: %v", proxyIdx, err)
		}

		for statIdx, alive := range statuses[proxyIdx] {
			stat := domain.ProxyStatistic{
				Alive:        alive,
				ResponseTime: 120,
				Attempt:      1,
				ProtocolID:   protocol.ID,
				ProxyID:      proxies[proxyIdx].ID,
				JudgeID:      judge.ID,
				CreatedAt:    createdAt.Add(time.Duration(proxyIdx*10+statIdx) * time.Minute),
			}
			if err := db.Create(&stat).Error; err != nil {
				t.Fatalf("create statistic proxy=%d stat=%d: %v", proxyIdx, statIdx, err)
			}
			if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
				t.Fatalf("update proxy status cache proxy=%d stat=%d: %v", proxyIdx, statIdx, err)
			}
		}
	}

	rotatorMin := domain.RotatingProxy{
		UserID:           user.ID,
		Name:             "uptime-min-rotator",
		ProtocolID:       protocol.ID,
		ListenPort:       10900,
		UptimeFilterType: "min",
		UptimePercentage: float64Ptr(80),
	}
	if err := db.Create(&rotatorMin).Error; err != nil {
		t.Fatalf("create min-uptime rotating proxy: %v", err)
	}

	minFirst, err := GetNextRotatingProxy(user.ID, rotatorMin.ID)
	if err != nil {
		t.Fatalf("min filter first rotation: %v", err)
	}
	if minFirst.ProxyID != proxies[0].ID {
		t.Fatalf("min filter first proxy id = %d, want %d", minFirst.ProxyID, proxies[0].ID)
	}

	minSecond, err := GetNextRotatingProxy(user.ID, rotatorMin.ID)
	if err != nil {
		t.Fatalf("min filter second rotation: %v", err)
	}
	if minSecond.ProxyID != proxies[2].ID {
		t.Fatalf("min filter second proxy id = %d, want %d", minSecond.ProxyID, proxies[2].ID)
	}

	minThird, err := GetNextRotatingProxy(user.ID, rotatorMin.ID)
	if err != nil {
		t.Fatalf("min filter third rotation: %v", err)
	}
	if minThird.ProxyID != proxies[0].ID {
		t.Fatalf("min filter third proxy id = %d, want %d", minThird.ProxyID, proxies[0].ID)
	}

	rotatorMax := domain.RotatingProxy{
		UserID:           user.ID,
		Name:             "uptime-max-rotator",
		ProtocolID:       protocol.ID,
		ListenPort:       10901,
		UptimeFilterType: "max",
		UptimePercentage: float64Ptr(50),
	}
	if err := db.Create(&rotatorMax).Error; err != nil {
		t.Fatalf("create max-uptime rotating proxy: %v", err)
	}

	maxFirst, err := GetNextRotatingProxy(user.ID, rotatorMax.ID)
	if err != nil {
		t.Fatalf("max filter first rotation: %v", err)
	}
	if maxFirst.ProxyID != proxies[1].ID {
		t.Fatalf("max filter first proxy id = %d, want %d", maxFirst.ProxyID, proxies[1].ID)
	}

	maxSecond, err := GetNextRotatingProxy(user.ID, rotatorMax.ID)
	if err != nil {
		t.Fatalf("max filter second rotation: %v", err)
	}
	if maxSecond.ProxyID != proxies[1].ID {
		t.Fatalf("max filter second proxy id = %d, want %d", maxSecond.ProxyID, proxies[1].ID)
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
		if err := updateProxyStatusCaches(db, []domain.ProxyStatistic{stat}); err != nil {
			t.Fatalf("update proxy status cache %d: %v", i, err)
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

func float64Ptr(value float64) *float64 {
	v := value
	return &v
}
