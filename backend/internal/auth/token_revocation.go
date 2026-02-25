package auth

import (
	"context"
	"strconv"
	"sync"
	"time"

	"magpie/internal/support"

	"github.com/redis/go-redis/v9"
)

const (
	authRevokedTokenPrefix      = "magpie:auth:revoked:jti:"
	authUserRevokedBeforePrefix = "magpie:auth:revoked_before:user:"
	authRedisOperationTimeout   = 2 * time.Second
	authUserRevokeTTL           = jwtTTL + 24*time.Hour
)

var (
	tokenRevocationMu      sync.Mutex
	localRevokedTokenByID  = make(map[string]time.Time)
	localUserRevokedBefore = make(map[uint]time.Time)
)

func revokeTokenID(tokenID string, until time.Time) error {
	if tokenID == "" {
		return nil
	}

	now := time.Now().UTC()
	if !until.After(now) {
		return nil
	}

	if client := redisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
		defer cancel()

		if err := client.Set(ctx, authRevokedTokenPrefix+tokenID, "1", time.Until(until)).Err(); err == nil {
			return nil
		}
	}

	tokenRevocationMu.Lock()
	purgeExpiredRevocationsLocked(now)
	localRevokedTokenByID[tokenID] = until
	tokenRevocationMu.Unlock()
	return nil
}

func isTokenIDRevoked(tokenID string) bool {
	if tokenID == "" {
		return false
	}

	if client := redisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
		defer cancel()

		exists, err := client.Exists(ctx, authRevokedTokenPrefix+tokenID).Result()
		if err == nil {
			return exists > 0
		}
	}

	now := time.Now().UTC()
	tokenRevocationMu.Lock()
	purgeExpiredRevocationsLocked(now)
	expiry, exists := localRevokedTokenByID[tokenID]
	tokenRevocationMu.Unlock()
	return exists && now.Before(expiry)
}

func revokeUserTokensBefore(userID uint, instant time.Time) error {
	if userID == 0 {
		return nil
	}

	instant = instant.UTC()

	if client := redisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
		defer cancel()

		err := client.Set(ctx, authUserRevokedBeforePrefix+strconv.FormatUint(uint64(userID), 10), strconv.FormatInt(instant.UnixNano(), 10), authUserRevokeTTL).Err()
		if err == nil {
			return nil
		}
	}

	tokenRevocationMu.Lock()
	localUserRevokedBefore[userID] = instant
	tokenRevocationMu.Unlock()
	return nil
}

func userTokensRevokedBefore(userID uint) (time.Time, bool) {
	if userID == 0 {
		return time.Time{}, false
	}

	if client := redisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
		defer cancel()

		value, err := client.Get(ctx, authUserRevokedBeforePrefix+strconv.FormatUint(uint64(userID), 10)).Result()
		if err == nil {
			parsed, parseErr := strconv.ParseInt(value, 10, 64)
			if parseErr == nil && parsed > 0 {
				return time.Unix(0, parsed).UTC(), true
			}
		}
	}

	tokenRevocationMu.Lock()
	revokedBefore, exists := localUserRevokedBefore[userID]
	tokenRevocationMu.Unlock()
	return revokedBefore, exists
}

func purgeExpiredRevocationsLocked(now time.Time) {
	for tokenID, expiresAt := range localRevokedTokenByID {
		if !expiresAt.After(now) {
			delete(localRevokedTokenByID, tokenID)
		}
	}
}

func redisClient() *redis.Client {
	client, err := support.GetRedisClient()
	if err != nil {
		return nil
	}
	return client
}
