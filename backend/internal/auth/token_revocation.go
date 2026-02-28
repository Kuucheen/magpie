package auth

import (
	"context"
	"errors"
	"fmt"
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
	authUserRevokeTTL           = maxJWTTTL + 24*time.Hour
	authRedisFailureBackoff     = 5 * time.Second
)

var (
	tokenRevocationMu sync.Mutex
	redisRetryAfter   time.Time

	ErrTokenRevocationStoreUnavailable = errors.New("token revocation store unavailable")
)

func revokeTokenID(tokenID string, until time.Time) error {
	if tokenID == "" {
		return nil
	}

	now := time.Now().UTC()
	if !until.After(now) {
		return nil
	}

	client, err := redisClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
	defer cancel()
	if err := client.Set(ctx, authRevokedTokenPrefix+tokenID, "1", time.Until(until)).Err(); err != nil {
		markRedisFailure()
		return wrapRevocationStoreError(err)
	}
	markRedisSuccess()
	return nil
}

func isTokenIDRevoked(tokenID string) (bool, error) {
	if tokenID == "" {
		return false, nil
	}

	client, err := redisClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
	defer cancel()
	exists, err := client.Exists(ctx, authRevokedTokenPrefix+tokenID).Result()
	if err != nil {
		markRedisFailure()
		return false, wrapRevocationStoreError(err)
	}
	markRedisSuccess()
	return exists > 0, nil
}

func revokeUserTokensBefore(userID uint, instant time.Time) error {
	if userID == 0 {
		return nil
	}

	instant = instant.UTC()

	client, err := redisClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
	defer cancel()
	if err := client.Set(ctx, authUserRevokedBeforePrefix+strconv.FormatUint(uint64(userID), 10), strconv.FormatInt(instant.UnixNano(), 10), authUserRevokeTTL).Err(); err != nil {
		markRedisFailure()
		return wrapRevocationStoreError(err)
	}
	markRedisSuccess()
	return nil
}

func userTokensRevokedBefore(userID uint) (time.Time, bool, error) {
	if userID == 0 {
		return time.Time{}, false, nil
	}

	client, err := redisClient()
	if err != nil {
		return time.Time{}, false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), authRedisOperationTimeout)
	defer cancel()
	value, err := client.Get(ctx, authUserRevokedBeforePrefix+strconv.FormatUint(uint64(userID), 10)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			markRedisSuccess()
			return time.Time{}, false, nil
		}
		markRedisFailure()
		return time.Time{}, false, wrapRevocationStoreError(err)
	}

	parsed, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil {
		markRedisFailure()
		return time.Time{}, false, wrapRevocationStoreError(fmt.Errorf("invalid revoked-before value for user %d: %w", userID, parseErr))
	}
	if parsed <= 0 {
		markRedisSuccess()
		return time.Time{}, false, nil
	}
	markRedisSuccess()
	return time.Unix(0, parsed).UTC(), true, nil
}

func redisClient() (*redis.Client, error) {
	if !redisAttemptAllowed() {
		return nil, ErrTokenRevocationStoreUnavailable
	}

	client, err := support.GetRedisClient()
	if err != nil {
		markRedisFailure()
		return nil, wrapRevocationStoreError(err)
	}

	markRedisSuccess()
	return client, nil
}

func wrapRevocationStoreError(err error) error {
	if err == nil {
		return ErrTokenRevocationStoreUnavailable
	}
	return errors.Join(ErrTokenRevocationStoreUnavailable, err)
}

func redisAttemptAllowed() bool {
	tokenRevocationMu.Lock()
	defer tokenRevocationMu.Unlock()

	if redisRetryAfter.IsZero() {
		return true
	}

	if time.Now().After(redisRetryAfter) {
		redisRetryAfter = time.Time{}
		return true
	}

	return false
}

func markRedisFailure() {
	tokenRevocationMu.Lock()
	redisRetryAfter = time.Now().Add(authRedisFailureBackoff)
	tokenRevocationMu.Unlock()
}

func markRedisSuccess() {
	tokenRevocationMu.Lock()
	redisRetryAfter = time.Time{}
	tokenRevocationMu.Unlock()
}
