package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"magpie/internal/support"
)

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

func resetJWTStateForTests(t *testing.T) {
	t.Helper()

	t.Setenv("JWT_SECRET", "unit-test-secret")
	t.Setenv("redisUrl", "redis://127.0.0.1:1")

	jwtSecretOnce = sync.Once{}
	jwtKey = nil
	jwtKeyErr = nil

	tokenRevocationMu.Lock()
	localRevokedTokenByID = make(map[string]time.Time)
	localUserRevokedBefore = make(map[uint]time.Time)
	redisRetryAfter = time.Time{}
	tokenRevocationMu.Unlock()

	if err := support.CloseRedisClient(); err != nil {
		t.Fatalf("CloseRedisClient failed: %v", err)
	}
}
