package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"magpie/internal/config"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const jwtSecretEnv = "JWT_SECRET"
const jwtSecretMinLength = 32

const (
	envJWTTTLMinutes = "JWT_TTL_MINUTES"
	defaultJWTTTL    = 24 * 7 * time.Hour
	minJWTTTL        = 15 * time.Minute
	maxJWTTTL        = 24 * 7 * time.Hour

	jwtClaimUserID   = "user_id"
	jwtClaimRole     = "role"
	jwtClaimExpiry   = "exp"
	jwtClaimIssuedAt = "iat"
	jwtClaimTokenID  = "jti"
	jwtClaimIssuedNs = "iat_ns"
	jwtTokenIDBytes  = 16
)

var (
	jwtSecretOnce sync.Once
	jwtKey        []byte
	jwtKeyErr     error
)

func RequireJWTSecretConfigured() error {
	_, err := jwtSigningKey()
	return err
}

func RequireJWTTTLConfigured() error {
	_, err := resolveJWTTTL()
	return err
}

func jwtSigningKey() ([]byte, error) {
	jwtSecretOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv(jwtSecretEnv))
		if raw == "" {
			jwtKeyErr = fmt.Errorf("jwt secret not configured: %s", jwtSecretEnv)
			return
		}
		if err := validateJWTSecret(raw); err != nil {
			jwtKeyErr = err
			return
		}
		jwtKey = []byte(raw)
	})

	if jwtKeyErr != nil {
		return nil, jwtKeyErr
	}

	return jwtKey, nil
}

func validateJWTSecret(secret string) error {
	if !config.StrictSecretValidationEnabled() {
		return nil
	}

	if len(secret) < jwtSecretMinLength {
		return fmt.Errorf("jwt secret is too short for strict mode: need at least %d characters", jwtSecretMinLength)
	}

	if isKnownPlaceholderSecret(secret, knownJWTSecretPlaceholders()) {
		return errors.New("jwt secret uses a known placeholder value; set a strong unique secret")
	}

	return nil
}

func knownJWTSecretPlaceholders() map[string]struct{} {
	return map[string]struct{}{
		"changeme":      {},
		"changemetoo":   {},
		"jwtsecret":     {},
		"yourjwtsecret": {},
	}
}

func isKnownPlaceholderSecret(secret string, placeholders map[string]struct{}) bool {
	normalized := normalizeSecretValue(secret)
	if normalized == "" {
		return false
	}

	_, exists := placeholders[normalized]
	return exists
}

func normalizeSecretValue(secret string) string {
	secret = strings.ToLower(strings.TrimSpace(secret))
	if secret == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(secret))
	for _, r := range secret {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func resolveJWTTTL() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(envJWTTTLMinutes))
	if raw == "" {
		return defaultJWTTTL, nil
	}

	minutes, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: must be an integer number of minutes", envJWTTTLMinutes, raw)
	}

	ttl := time.Duration(minutes) * time.Minute
	if ttl < minJWTTTL || ttl > maxJWTTTL {
		return 0, fmt.Errorf("%s out of range: got %d minutes, expected %d-%d", envJWTTTLMinutes, minutes, int(minJWTTTL/time.Minute), int(maxJWTTTL/time.Minute))
	}

	return ttl, nil
}

func GenerateJWT(userId uint, role string) (string, error) {
	signingKey, err := jwtSigningKey()
	if err != nil {
		return "", err
	}

	jwtTTL, err := resolveJWTTTL()
	if err != nil {
		return "", err
	}

	tokenID, err := generateTokenID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.MapClaims{
		jwtClaimUserID:   userId,
		jwtClaimRole:     role,
		jwtClaimIssuedAt: now.Unix(),
		jwtClaimIssuedNs: strconv.FormatInt(now.UnixNano(), 10),
		jwtClaimExpiry:   now.Add(jwtTTL).Unix(),
		jwtClaimTokenID:  tokenID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func ValidateJWT(tokenString string) (map[string]interface{}, error) {
	claims, err := parseSignedJWT(tokenString)
	if err != nil {
		return nil, err
	}

	if err := validateTokenRevocation(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func RevokeJWT(tokenString string) error {
	claims, err := parseSignedJWT(tokenString)
	if err != nil {
		return err
	}
	return RevokeJWTClaims(claims)
}

func RevokeJWTClaims(claims map[string]interface{}) error {
	tokenID, ok := claimString(claims, jwtClaimTokenID)
	if !ok || tokenID == "" {
		return errors.New("token missing jti claim")
	}

	expiryUnix, ok := claimInt64(claims, jwtClaimExpiry)
	if !ok {
		return errors.New("token missing exp claim")
	}

	revokeUntil := time.Unix(expiryUnix, 0).UTC()
	if !revokeUntil.After(time.Now().UTC()) {
		return nil
	}

	return revokeTokenID(tokenID, revokeUntil)
}

func RevokeAllUserJWTs(userID uint) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	return revokeUserTokensBefore(userID, time.Now().UTC())
}

func RotateJWT(tokenString string) (string, string, error) {
	claims, err := ValidateJWT(tokenString)
	if err != nil {
		return "", "", err
	}

	userID, ok := claimUint(claims, jwtClaimUserID)
	if !ok || userID == 0 {
		return "", "", errors.New("token missing user_id claim")
	}

	role, ok := claimString(claims, jwtClaimRole)
	if !ok || role == "" {
		return "", "", errors.New("token missing role claim")
	}

	if err := RevokeJWTClaims(claims); err != nil {
		return "", "", err
	}

	token, err := GenerateJWT(userID, role)
	if err != nil {
		return "", "", err
	}

	return token, role, nil
}

func parseSignedJWT(tokenString string) (map[string]interface{}, error) {
	signingKey, err := jwtSigningKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return signingKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	if token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func validateTokenRevocation(claims map[string]interface{}) error {
	tokenID, ok := claimString(claims, jwtClaimTokenID)
	if !ok || tokenID == "" {
		return errors.New("invalid token")
	}

	revoked, err := isTokenIDRevoked(tokenID)
	if err != nil {
		return errors.New("invalid token")
	}
	if revoked {
		return errors.New("invalid token")
	}

	userID, ok := claimUint(claims, jwtClaimUserID)
	if !ok || userID == 0 {
		return errors.New("invalid token")
	}

	issuedAtNs, ok := claimIssuedAtUnixNano(claims)
	if !ok {
		return errors.New("invalid token")
	}

	revokedBefore, exists, err := userTokensRevokedBefore(userID)
	if err != nil {
		return errors.New("invalid token")
	}
	if exists && issuedAtNs <= revokedBefore.UnixNano() {
		return errors.New("invalid token")
	}

	return nil
}

func claimString(claims map[string]interface{}, key string) (string, bool) {
	raw, ok := claims[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func claimInt64(claims map[string]interface{}, key string) (int64, bool) {
	raw, ok := claims[key]
	if !ok {
		return 0, false
	}

	switch value := raw.(type) {
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case json.Number:
		out, err := value.Int64()
		return out, err == nil
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return 0, false
		}
		out, err := strconv.ParseInt(trimmed, 10, 64)
		return out, err == nil
	default:
		return 0, false
	}
}

func claimUint(claims map[string]interface{}, key string) (uint, bool) {
	value, ok := claimInt64(claims, key)
	if !ok || value <= 0 {
		return 0, false
	}
	return uint(value), true
}

func claimIssuedAtUnixNano(claims map[string]interface{}) (int64, bool) {
	if nsRaw, ok := claims[jwtClaimIssuedNs]; ok {
		if nsString, ok := nsRaw.(string); ok {
			nsString = strings.TrimSpace(nsString)
			if nsString != "" {
				ns, err := strconv.ParseInt(nsString, 10, 64)
				if err == nil && ns > 0 {
					return ns, true
				}
			}
		}
	}

	issuedAt, ok := claimInt64(claims, jwtClaimIssuedAt)
	if !ok || issuedAt <= 0 {
		return 0, false
	}
	return issuedAt * int64(time.Second), true
}

func generateTokenID() (string, error) {
	buffer := make([]byte, jwtTokenIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate token id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
