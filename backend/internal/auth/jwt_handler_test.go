package auth

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"magpie/internal/config"

	"magpie/internal/support"
)

const envStrictSecretValidation = "STRICT_SECRET_VALIDATION"
const envJWTTTLMinutesKey = "JWT_TTL_MINUTES"

func TestRequireJWTTTLConfigured_DefaultWhenUnset(t *testing.T) {
	resetJWTStateForTests(t)

	if err := RequireJWTTTLConfigured(); err != nil {
		t.Fatalf("expected default JWT TTL to be valid, got %v", err)
	}
}

func TestRequireJWTTTLConfigured_RejectsInvalidValue(t *testing.T) {
	resetJWTStateForTests(t)
	t.Setenv(envJWTTTLMinutesKey, "not-a-number")

	if err := RequireJWTTTLConfigured(); err == nil {
		t.Fatal("expected invalid JWT_TTL_MINUTES to be rejected")
	}
}

func TestRequireJWTTTLConfigured_RejectsOutOfRangeValue(t *testing.T) {
	resetJWTStateForTests(t)
	t.Setenv(envJWTTTLMinutesKey, "5")

	if err := RequireJWTTTLConfigured(); err == nil {
		t.Fatal("expected out-of-range JWT_TTL_MINUTES to be rejected")
	}
}

func TestGenerateJWT_UsesConfiguredTTL(t *testing.T) {
	resetJWTStateForTests(t)
	t.Setenv(envJWTTTLMinutesKey, "60")

	token, err := GenerateJWT(11, "user")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	issuedAt, ok := claimInt64(claims, jwtClaimIssuedAt)
	if !ok {
		t.Fatal("expected iat claim")
	}
	expiresAt, ok := claimInt64(claims, jwtClaimExpiry)
	if !ok {
		t.Fatal("expected exp claim")
	}

	if diff := expiresAt - issuedAt; diff != int64((60 * time.Minute).Seconds()) {
		t.Fatalf("expected token TTL of 60 minutes, got %d seconds", diff)
	}
}

func TestValidateJWTRejectsRevokedToken(t *testing.T) {
	resetJWTStateForTests(t)

	token, err := GenerateJWT(42, "user")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if _, err := ValidateJWT(token); err != nil {
		t.Fatalf("ValidateJWT failed before revoke: %v", err)
	}

	if err := RevokeJWT(token); err != nil {
		t.Fatalf("RevokeJWT failed: %v", err)
	}

	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("ValidateJWT accepted revoked token")
	}
}

func TestRotateJWTRevokesOldToken(t *testing.T) {
	resetJWTStateForTests(t)

	token, err := GenerateJWT(7, "admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	rotated, role, err := RotateJWT(token)
	if err != nil {
		t.Fatalf("RotateJWT failed: %v", err)
	}

	if role != "admin" {
		t.Fatalf("RotateJWT role = %q, want admin", role)
	}

	if rotated == "" {
		t.Fatal("RotateJWT returned empty token")
	}

	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("old token still valid after rotation")
	}

	if _, err := ValidateJWT(rotated); err != nil {
		t.Fatalf("rotated token invalid: %v", err)
	}
}

func TestRevokeAllUserJWTsInvalidatesOlderTokens(t *testing.T) {
	resetJWTStateForTests(t)

	originalToken, err := GenerateJWT(99, "user")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if err := RevokeAllUserJWTs(99); err != nil {
		t.Fatalf("RevokeAllUserJWTs failed: %v", err)
	}

	if _, err := ValidateJWT(originalToken); err == nil {
		t.Fatal("token remained valid after user-wide revoke")
	}

	time.Sleep(1 * time.Millisecond)

	freshToken, err := GenerateJWT(99, "user")
	if err != nil {
		t.Fatalf("GenerateJWT (fresh) failed: %v", err)
	}

	if _, err := ValidateJWT(freshToken); err != nil {
		t.Fatalf("fresh token invalid after user-wide revoke: %v", err)
	}
}

func TestValidateJWTRejectsTokenWithoutJTI(t *testing.T) {
	resetJWTStateForTests(t)

	signingKey, err := jwtSigningKey()
	if err != nil {
		t.Fatalf("jwtSigningKey failed: %v", err)
	}

	claims := jwt.MapClaims{
		jwtClaimUserID:   1,
		jwtClaimRole:     "user",
		jwtClaimIssuedAt: time.Now().Unix(),
		jwtClaimExpiry:   time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(signingKey)
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	if _, err := ValidateJWT(signed); err == nil {
		t.Fatal("ValidateJWT accepted token without jti")
	}
}

func TestValidateJWTFailOpensWhenRevocationStoreUnavailableByDefault(t *testing.T) {
	resetJWTStateForTests(t)

	token, err := GenerateJWT(42, "user")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if _, err := ValidateJWT(token); err != nil {
		t.Fatalf("ValidateJWT failed before redis outage: %v", err)
	}

	t.Setenv("redisUrl", "redis://127.0.0.1:1")
	if err := support.CloseRedisClient(); err != nil {
		t.Fatalf("CloseRedisClient failed: %v", err)
	}

	tokenRevocationMu.Lock()
	redisRetryAfter = time.Time{}
	tokenRevocationMu.Unlock()

	if _, err := ValidateJWT(token); err != nil {
		t.Fatalf("ValidateJWT should fail-open while revocation store is unavailable by default, got: %v", err)
	}
}

func TestValidateJWTFailsClosedWhenRevocationStoreUnavailableWhenDisabled(t *testing.T) {
	resetJWTStateForTests(t)
	t.Setenv(envAuthRevocationFailOpen, "false")

	token, err := GenerateJWT(42, "user")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if _, err := ValidateJWT(token); err != nil {
		t.Fatalf("ValidateJWT failed before redis outage: %v", err)
	}

	t.Setenv("redisUrl", "redis://127.0.0.1:1")
	if err := support.CloseRedisClient(); err != nil {
		t.Fatalf("CloseRedisClient failed: %v", err)
	}

	tokenRevocationMu.Lock()
	redisRetryAfter = time.Time{}
	tokenRevocationMu.Unlock()

	if _, err := ValidateJWT(token); err == nil {
		t.Fatal("ValidateJWT accepted token while revocation store was unavailable and fail-open was explicitly disabled")
	}
}

func TestRequireJWTSecretConfigured_StrictModeRejectsPlaceholderByDefaultInProduction(t *testing.T) {
	resetJWTSigningKeyStateForTests()
	setProductionModeForJWTTests(t, true)
	unsetEnvForJWTTests(t, envStrictSecretValidation)
	t.Setenv(jwtSecretEnv, "ChangeMeToo")

	if err := RequireJWTSecretConfigured(); err == nil {
		t.Fatal("expected strict validation to reject placeholder JWT secret in production")
	}
}

func TestRequireJWTSecretConfigured_StrictModeOverrideCanBeDisabled(t *testing.T) {
	resetJWTSigningKeyStateForTests()
	setProductionModeForJWTTests(t, true)
	t.Setenv(envStrictSecretValidation, "false")
	t.Setenv(jwtSecretEnv, "ChangeMeToo")

	if err := RequireJWTSecretConfigured(); err != nil {
		t.Fatalf("expected strict validation override to allow placeholder secret, got %v", err)
	}
}

func TestRequireJWTSecretConfigured_StrictModeRejectsShortSecret(t *testing.T) {
	resetJWTSigningKeyStateForTests()
	setProductionModeForJWTTests(t, false)
	t.Setenv(envStrictSecretValidation, "true")
	t.Setenv(jwtSecretEnv, "this-secret-is-too-short")

	if err := RequireJWTSecretConfigured(); err == nil {
		t.Fatal("expected strict validation to reject short JWT secret")
	}
}

func resetJWTStateForTests(t *testing.T) {
	t.Helper()

	t.Setenv("JWT_SECRET", "unit-test-secret")
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run failed: %v", err)
	}
	t.Cleanup(redisServer.Close)
	t.Setenv("redisUrl", "redis://"+redisServer.Addr())

	jwtSecretOnce = sync.Once{}
	jwtKey = nil
	jwtKeyErr = nil

	tokenRevocationMu.Lock()
	redisRetryAfter = time.Time{}
	tokenRevocationMu.Unlock()
	authRevocationLastDegradedLog.Store(0)

	if err := support.CloseRedisClient(); err != nil {
		t.Fatalf("CloseRedisClient failed: %v", err)
	}
	t.Cleanup(func() {
		_ = support.CloseRedisClient()
	})
}

func resetJWTSigningKeyStateForTests() {
	jwtSecretOnce = sync.Once{}
	jwtKey = nil
	jwtKeyErr = nil
}

func setProductionModeForJWTTests(t *testing.T, production bool) {
	t.Helper()

	prev := config.InProductionMode
	config.SetProductionMode(production)
	t.Cleanup(func() {
		config.SetProductionMode(prev)
	})
}

func unsetEnvForJWTTests(t *testing.T, key string) {
	t.Helper()

	prev, had := os.LookupEnv(key)
	if had {
		t.Cleanup(func() {
			_ = os.Setenv(key, prev)
		})
	} else {
		t.Cleanup(func() {
			_ = os.Unsetenv(key)
		})
	}

	_ = os.Unsetenv(key)
}
