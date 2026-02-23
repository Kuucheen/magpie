package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"time"
)

const jwtSecretEnv = "JWT_SECRET"

var (
	jwtSecretOnce sync.Once
	jwtKey        []byte
	jwtKeyErr     error
)

func RequireJWTSecretConfigured() error {
	_, err := jwtSigningKey()
	return err
}

func jwtSigningKey() ([]byte, error) {
	jwtSecretOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv(jwtSecretEnv))
		if raw == "" {
			jwtKeyErr = fmt.Errorf("jwt secret not configured: %s", jwtSecretEnv)
			return
		}
		jwtKey = []byte(raw)
	})

	if jwtKeyErr != nil {
		return nil, jwtKeyErr
	}

	return jwtKey, nil
}

func GenerateJWT(userId uint, role string) (string, error) {
	signingKey, err := jwtSigningKey()
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"user_id": userId,
		"role":    role,
		"exp":     time.Now().Add(24 * 7 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func ValidateJWT(tokenString string) (map[string]interface{}, error) {
	signingKey, err := jwtSigningKey()
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return signingKey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
