package domain

import (
	"encoding/base64"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	judgeIPCacheTTL             = 30 * time.Minute
	judgeIPCacheCleanupInterval = 5 * time.Minute
)

type Judge struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	FullString string `gorm:"size:512;not null;unique"`

	ProxyStatistics []ProxyStatistic `gorm:"foreignKey:JudgeID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Users           []User           `gorm:"many2many:user_judges;"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type cachedJudgeIP struct {
	ip        string
	expiresAt time.Time
}

var (
	judgeIPCacheMu          sync.Mutex
	judgeResolvedIPByURL    = make(map[string]cachedJudgeIP)
	nextJudgeIPCacheCleanup = time.Now().Add(judgeIPCacheCleanupInterval)
)

func (judge *Judge) SetUp() error {
	parsedURL, err := url.Parse(judge.FullString)
	if err != nil {
		return err
	}
	judge.FullString = parsedURL.String()
	return nil
}

func (judge *Judge) UpdateIp() {
	now := time.Now()
	key := judge.cacheKey()
	hostname := judge.GetHostname()

	if hostname == "" {
		deleteCachedJudgeIP(key, now)
		return
	}

	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		deleteCachedJudgeIP(key, now)
		return
	}

	storeCachedJudgeIP(key, addrs[0], now)
}

func (judge *Judge) GetIp() string {
	now := time.Now()
	key := judge.cacheKey()
	if key != "" {
		if cachedIP, ok := loadCachedJudgeIP(key, now); ok && strings.TrimSpace(cachedIP) != "" {
			return cachedIP
		}
	}

	return judge.GetHostname()
}

func (judge *Judge) GetHostname() string {
	parsedURL, err := url.Parse(judge.FullString)
	if err != nil {
		return ""
	}
	return parsedURL.Hostname()
}

func (judge *Judge) GetScheme() string {
	parsedURL, err := url.Parse(judge.FullString)
	if err != nil {
		return ""
	}
	return parsedURL.Scheme
}

func (judge *Judge) cacheKey() string {
	if judge == nil {
		return ""
	}

	normalized := strings.TrimSpace(strings.ToLower(judge.FullString))
	if normalized == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(normalized))
}

func storeCachedJudgeIP(key string, ip string, now time.Time) {
	if key == "" {
		return
	}

	judgeIPCacheMu.Lock()
	defer judgeIPCacheMu.Unlock()

	cleanupExpiredJudgeIPCacheLocked(now)
	judgeResolvedIPByURL[key] = cachedJudgeIP{
		ip:        ip,
		expiresAt: now.Add(judgeIPCacheTTL),
	}
}

func loadCachedJudgeIP(key string, now time.Time) (string, bool) {
	if key == "" {
		return "", false
	}

	judgeIPCacheMu.Lock()
	defer judgeIPCacheMu.Unlock()

	cleanupExpiredJudgeIPCacheLocked(now)
	entry, exists := judgeResolvedIPByURL[key]
	if !exists {
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(judgeResolvedIPByURL, key)
		return "", false
	}
	return entry.ip, true
}

func deleteCachedJudgeIP(key string, now time.Time) {
	if key == "" {
		return
	}

	judgeIPCacheMu.Lock()
	defer judgeIPCacheMu.Unlock()

	cleanupExpiredJudgeIPCacheLocked(now)
	delete(judgeResolvedIPByURL, key)
}

func cleanupExpiredJudgeIPCacheLocked(now time.Time) {
	if now.Before(nextJudgeIPCacheCleanup) {
		return
	}

	for key, entry := range judgeResolvedIPByURL {
		if !entry.expiresAt.After(now) {
			delete(judgeResolvedIPByURL, key)
		}
	}
	nextJudgeIPCacheCleanup = now.Add(judgeIPCacheCleanupInterval)
}
