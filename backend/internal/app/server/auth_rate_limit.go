package server

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"magpie/internal/support"

	"github.com/redis/go-redis/v9"
)

const (
	envAuthRequestWindowSeconds      = "AUTH_REQUEST_RATE_LIMIT_WINDOW_SECONDS"
	envAuthLoginRequestsPerWindow    = "AUTH_LOGIN_RATE_LIMIT_PER_WINDOW"
	envAuthRegisterRequestsPerWindow = "AUTH_REGISTER_RATE_LIMIT_PER_WINDOW"
	envAuthLoginFailureWindowSeconds = "AUTH_LOGIN_FAILURE_WINDOW_SECONDS"
	envAuthLoginFailuresPerIP        = "AUTH_LOGIN_FAILURE_LIMIT_PER_IP"
	envAuthLoginFailuresPerEmail     = "AUTH_LOGIN_FAILURE_LIMIT_PER_EMAIL"
	envAuthLocalFallbackMaxKeys      = "AUTH_RATE_LIMIT_LOCAL_FALLBACK_MAX_KEYS"

	defaultAuthRequestWindowSeconds      = 60
	defaultAuthLoginRequestsPerWindow    = 60
	defaultAuthRegisterRequestsPerWindow = 20
	defaultAuthLoginFailureWindowSeconds = 15 * 60
	defaultAuthLoginFailuresPerIP        = 30
	defaultAuthLoginFailuresPerEmail     = 10
	defaultAuthLocalFallbackMaxKeys      = 10000
	authRedisFailureBackoff              = 5 * time.Second
)

const (
	authLoginRateExceededMessage    = "Too many login requests. Please try again later."
	authRegisterRateExceededMessage = "Too many registration requests. Please try again later."
	authLoginBlockedMessage         = "Too many login attempts. Please try again later."
)

var authRateLimitIncrementScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
	ttl = ARGV[1]
end
return {count, ttl}
`)

type authRateLimits struct {
	loginRequests    *fixedWindowLimiter
	registerRequests *fixedWindowLimiter
	loginFailures    *loginFailureLimiter
}

type fixedWindowLimiter struct {
	prefix          string
	limit           int64
	window          time.Duration
	maxLocalEntries int

	mu       sync.Mutex
	counters map[string]localCounter
	expiries localCounterExpiryHeap
}

type localCounter struct {
	count     int64
	expiresAt time.Time
}

type localCounterExpiry struct {
	key       string
	expiresAt time.Time
}

type localCounterExpiryHeap []localCounterExpiry

type loginFailureLimiter struct {
	perIP    *fixedWindowLimiter
	perEmail *fixedWindowLimiter
}

var (
	authRateLimitsOnce sync.Once
	globalAuthLimits   authRateLimits
	authRedisMu        sync.Mutex
	authRedisRetryAt   time.Time
)

func withLoginRateLimit(next http.Handler) http.Handler {
	limits := getAuthRateLimits()
	return limits.loginRequests.wrap(next, authLoginRateExceededMessage, "login_request")
}

func withRegisterRateLimit(next http.Handler) http.Handler {
	limits := getAuthRateLimits()
	return limits.registerRequests.wrap(next, authRegisterRateExceededMessage, "register_request")
}

func loginFailuresBlocked(r *http.Request, email string) (bool, time.Duration) {
	limits := getAuthRateLimits()
	return limits.loginFailures.isBlocked(getAuthRateLimitKey(r), email)
}

func recordLoginFailure(r *http.Request, email string) {
	limits := getAuthRateLimits()
	limits.loginFailures.recordFailure(getAuthRateLimitKey(r), email)
}

func clearLoginFailures(r *http.Request, email string) {
	limits := getAuthRateLimits()
	limits.loginFailures.recordSuccess(getAuthRateLimitKey(r), email)
}

func getAuthRateLimits() *authRateLimits {
	authRateLimitsOnce.Do(func() {
		requestWindow := time.Duration(resolvePositiveEnvInt(envAuthRequestWindowSeconds, defaultAuthRequestWindowSeconds)) * time.Second
		loginFailureWindow := time.Duration(resolvePositiveEnvInt(envAuthLoginFailureWindowSeconds, defaultAuthLoginFailureWindowSeconds)) * time.Second
		localFallbackMaxKeys := resolvePositiveEnvInt(envAuthLocalFallbackMaxKeys, defaultAuthLocalFallbackMaxKeys)

		globalAuthLimits = authRateLimits{
			loginRequests: newFixedWindowLimiter(
				"magpie:ratelimit:login:request",
				int64(resolvePositiveEnvInt(envAuthLoginRequestsPerWindow, defaultAuthLoginRequestsPerWindow)),
				requestWindow,
				localFallbackMaxKeys,
			),
			registerRequests: newFixedWindowLimiter(
				"magpie:ratelimit:register:request",
				int64(resolvePositiveEnvInt(envAuthRegisterRequestsPerWindow, defaultAuthRegisterRequestsPerWindow)),
				requestWindow,
				localFallbackMaxKeys,
			),
			loginFailures: &loginFailureLimiter{
				perIP: newFixedWindowLimiter(
					"magpie:ratelimit:login:fail:ip",
					int64(resolvePositiveEnvInt(envAuthLoginFailuresPerIP, defaultAuthLoginFailuresPerIP)),
					loginFailureWindow,
					localFallbackMaxKeys,
				),
				perEmail: newFixedWindowLimiter(
					"magpie:ratelimit:login:fail:email",
					int64(resolvePositiveEnvInt(envAuthLoginFailuresPerEmail, defaultAuthLoginFailuresPerEmail)),
					loginFailureWindow,
					localFallbackMaxKeys,
				),
			},
		}
	})

	return &globalAuthLimits
}

func newFixedWindowLimiter(prefix string, limit int64, window time.Duration, maxLocalEntries int) *fixedWindowLimiter {
	if maxLocalEntries <= 0 {
		maxLocalEntries = defaultAuthLocalFallbackMaxKeys
	}

	limiter := &fixedWindowLimiter{
		prefix:          prefix,
		limit:           limit,
		window:          window,
		maxLocalEntries: maxLocalEntries,
		counters:        make(map[string]localCounter),
		expiries:        make(localCounterExpiryHeap, 0),
	}
	heap.Init(&limiter.expiries)
	return limiter
}

func (l *fixedWindowLimiter) wrap(next http.Handler, message string, scope string) http.Handler {
	if l == nil || next == nil || l.limit <= 0 || l.window <= 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := l.key(getAuthRateLimitKey(r))

		allowed, retryAfter := l.allow(key)
		if !allowed {
			setRetryAfterHeader(w, retryAfter)
			recordRateLimitBlockMetric(scope)
			writeError(w, message, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *fixedWindowLimiter) allow(key string) (bool, time.Duration) {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true, 0
	}

	count, ttl := l.increment(key)
	if count <= l.limit {
		return true, 0
	}

	if ttl <= 0 {
		ttl = l.window
	}
	return false, ttl
}

func (l *fixedWindowLimiter) increment(key string) (int64, time.Duration) {
	if l == nil {
		return 0, 0
	}

	if count, ttl, ok := l.incrementRedis(key); ok {
		return count, ttl
	}

	return l.incrementLocal(key)
}

func (l *fixedWindowLimiter) current(key string) (int64, time.Duration) {
	if l == nil {
		return 0, 0
	}

	if count, ttl, ok := l.currentRedis(key); ok {
		return count, ttl
	}

	return l.currentLocal(key)
}

func (l *fixedWindowLimiter) reset(key string) {
	if l == nil {
		return
	}

	if client := redisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Del(ctx, l.redisKey(key)).Err(); err != nil {
			markAuthRedisFailure()
		} else {
			markAuthRedisSuccess()
		}
	}

	l.mu.Lock()
	delete(l.counters, key)
	l.mu.Unlock()
}

func (l *fixedWindowLimiter) incrementRedis(key string) (int64, time.Duration, bool) {
	client := redisClient()
	if client == nil {
		return 0, 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	windowMS := strconv.FormatInt(l.window.Milliseconds(), 10)
	result, err := authRateLimitIncrementScript.Run(ctx, client, []string{l.redisKey(key)}, windowMS).Result()
	if err != nil {
		markAuthRedisFailure()
		return 0, 0, false
	}
	markAuthRedisSuccess()

	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		return 0, 0, false
	}

	count, ok := toInt64(values[0])
	if !ok {
		return 0, 0, false
	}

	ttlMS, ok := toInt64(values[1])
	if !ok {
		return 0, 0, false
	}
	if ttlMS <= 0 {
		ttlMS = l.window.Milliseconds()
	}

	return count, time.Duration(ttlMS) * time.Millisecond, true
}

func (l *fixedWindowLimiter) currentRedis(key string) (int64, time.Duration, bool) {
	client := redisClient()
	if client == nil {
		return 0, 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	count, err := client.Get(ctx, l.redisKey(key)).Int64()
	if err != nil {
		if err == redis.Nil {
			markAuthRedisSuccess()
			return 0, 0, true
		}
		markAuthRedisFailure()
		return 0, 0, false
	}

	ttl, err := client.PTTL(ctx, l.redisKey(key)).Result()
	if err != nil {
		markAuthRedisFailure()
		return 0, 0, false
	}
	markAuthRedisSuccess()
	if ttl < 0 {
		ttl = 0
	}

	return count, ttl, true
}

func (l *fixedWindowLimiter) incrementLocal(key string) (int64, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.purgeExpiredLocalLocked(now)

	entry, exists := l.counters[key]
	if !exists {
		l.enforceLocalCapacityLocked(now)
		entry = localCounter{count: 1, expiresAt: now.Add(l.window)}
		l.counters[key] = entry
		heap.Push(&l.expiries, localCounterExpiry{key: key, expiresAt: entry.expiresAt})
		return entry.count, time.Until(entry.expiresAt)
	}

	entry.count++
	l.counters[key] = entry
	return entry.count, time.Until(entry.expiresAt)
}

func (l *fixedWindowLimiter) currentLocal(key string) (int64, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.purgeExpiredLocalLocked(now)

	entry, exists := l.counters[key]
	if !exists {
		return 0, 0
	}

	return entry.count, time.Until(entry.expiresAt)
}

func (l *fixedWindowLimiter) purgeExpiredLocalLocked(now time.Time) {
	for len(l.expiries) > 0 {
		next := l.expiries[0]
		if next.expiresAt.After(now) {
			return
		}

		expired := heap.Pop(&l.expiries).(localCounterExpiry)
		entry, exists := l.counters[expired.key]
		if !exists {
			continue
		}
		if !entry.expiresAt.Equal(expired.expiresAt) {
			continue
		}
		delete(l.counters, expired.key)
	}
}

func (l *fixedWindowLimiter) enforceLocalCapacityLocked(now time.Time) {
	if l.maxLocalEntries <= 0 {
		return
	}

	l.purgeExpiredLocalLocked(now)
	for len(l.counters) >= l.maxLocalEntries {
		if l.evictOneLocalLocked() {
			continue
		}
		for key := range l.counters {
			delete(l.counters, key)
			return
		}
		return
	}
}

func (l *fixedWindowLimiter) evictOneLocalLocked() bool {
	for len(l.expiries) > 0 {
		evicted := heap.Pop(&l.expiries).(localCounterExpiry)
		entry, exists := l.counters[evicted.key]
		if !exists {
			continue
		}
		if !entry.expiresAt.Equal(evicted.expiresAt) {
			continue
		}

		delete(l.counters, evicted.key)
		return true
	}
	return false
}

func (h localCounterExpiryHeap) Len() int {
	return len(h)
}

func (h localCounterExpiryHeap) Less(i, j int) bool {
	return h[i].expiresAt.Before(h[j].expiresAt)
}

func (h localCounterExpiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *localCounterExpiryHeap) Push(x any) {
	entry, ok := x.(localCounterExpiry)
	if !ok {
		return
	}
	*h = append(*h, entry)
}

func (h *localCounterExpiryHeap) Pop() any {
	old := *h
	n := len(old)
	if n == 0 {
		return localCounterExpiry{}
	}
	entry := old[n-1]
	*h = old[:n-1]
	return entry
}

func (l *fixedWindowLimiter) redisKey(key string) string {
	return fmt.Sprintf("%s:%s", l.prefix, key)
}

func (l *fixedWindowLimiter) key(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return "unknown"
	}
	return strings.Join(filtered, ":")
}

func (l *loginFailureLimiter) isBlocked(ip string, email string) (bool, time.Duration) {
	ipKey := l.perIP.key(normalizeRateLimitIP(ip))
	ipCount, ipTTL := l.perIP.current(ipKey)
	if ipCount >= l.perIP.limit {
		return true, ipTTL
	}

	emailKey := l.perEmail.key(hashIdentifier(strings.ToLower(strings.TrimSpace(email))))
	emailCount, emailTTL := l.perEmail.current(emailKey)
	if emailCount >= l.perEmail.limit {
		return true, emailTTL
	}

	return false, 0
}

func (l *loginFailureLimiter) recordFailure(ip string, email string) {
	l.perIP.increment(l.perIP.key(normalizeRateLimitIP(ip)))
	l.perEmail.increment(l.perEmail.key(hashIdentifier(strings.ToLower(strings.TrimSpace(email)))))
}

func (l *loginFailureLimiter) recordSuccess(ip string, email string) {
	l.perIP.reset(l.perIP.key(normalizeRateLimitIP(ip)))
	l.perEmail.reset(l.perEmail.key(hashIdentifier(strings.ToLower(strings.TrimSpace(email)))))
}

func setRetryAfterHeader(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter <= 0 {
		return
	}

	seconds := int(retryAfter / time.Second)
	if retryAfter%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func normalizeRateLimitIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "unknown"
	}
	return ip
}

func hashIdentifier(value string) string {
	if value == "" {
		return "unknown"
	}

	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func toInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func redisClient() *redis.Client {
	if !authRedisAttemptAllowed() {
		return nil
	}

	client, err := support.GetRedisClient()
	if err != nil {
		markAuthRedisFailure()
		return nil
	}

	markAuthRedisSuccess()
	return client
}

func authRedisAttemptAllowed() bool {
	authRedisMu.Lock()
	defer authRedisMu.Unlock()

	if authRedisRetryAt.IsZero() {
		return true
	}

	if time.Now().After(authRedisRetryAt) {
		authRedisRetryAt = time.Time{}
		return true
	}

	return false
}

func markAuthRedisFailure() {
	authRedisMu.Lock()
	authRedisRetryAt = time.Now().Add(authRedisFailureBackoff)
	authRedisMu.Unlock()
}

func markAuthRedisSuccess() {
	authRedisMu.Lock()
	authRedisRetryAt = time.Time{}
	authRedisMu.Unlock()
}
