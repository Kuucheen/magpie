package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/support"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestForgotPassword_GenericSuccessAndEmailSentForExistingUser(t *testing.T) {
	setupPasswordResetTestDB(t)

	hashedPassword, err := support.HashPassword("current-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "forgot-rate@example.com", Password: hashedPassword, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	restore := stubPasswordResetConfigForTests(t)

	var deliveredTo string
	var deliveredBody string
	queueEmailFn = func(kind, toAddress, subject, body string) error {
		deliveredTo = toAddress
		deliveredBody = body
		return nil
	}

	body, err := json.Marshal(dto.PasswordResetRequest{Email: user.Email})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/forgotPassword", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	forgotPassword(rec, req)

	restore()

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if deliveredTo != user.Email {
		t.Fatalf("sendEmail recipient = %q, want %q", deliveredTo, user.Email)
	}
	if !strings.Contains(deliveredBody, "https://app.example/reset-password?token=") {
		t.Fatalf("email body did not contain reset URL: %q", deliveredBody)
	}
	if !strings.Contains(deliveredBody, "<!doctype html>") {
		t.Fatalf("email body was not rendered as HTML: %q", deliveredBody)
	}
	if !strings.Contains(deliveredBody, "https://assets.example.com/magpie-email.png") {
		t.Fatalf("email body did not include brand image: %q", deliveredBody)
	}

	var tokens []domain.PasswordResetToken
	if err := database.DB.Find(&tokens).Error; err != nil {
		t.Fatalf("load reset tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("token count = %d, want 1", len(tokens))
	}

	token := extractTokenFromBody(deliveredBody)
	if token == "" {
		t.Fatalf("failed to extract token from email body: %q", deliveredBody)
	}
	if tokens[0].TokenHash != hashPasswordResetToken(token) {
		t.Fatal("stored token hash does not match delivered token")
	}
}

func TestForgotPassword_ReturnsServiceUnavailableWhenEmailOutboxPersistFails(t *testing.T) {
	setupPasswordResetTestDB(t)

	hashedPassword, err := support.HashPassword("current-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset@example.com", Password: hashedPassword, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	restore := stubPasswordResetConfigForTests(t)
	defer restore()

	queueEmailFn = func(kind, toAddress, subject, body string) error {
		return errors.New("outbox unavailable")
	}

	body, err := json.Marshal(dto.PasswordResetRequest{Email: user.Email})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/forgotPassword", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	forgotPassword(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var tokenCount int64
	if err := database.DB.Model(&domain.PasswordResetToken{}).Where("user_id = ?", user.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count reset tokens: %v", err)
	}
	if tokenCount != 0 {
		t.Fatalf("token count = %d, want 0", tokenCount)
	}
}

func TestRenderPasswordResetConfirmationEmail_UsesBrandedHTML(t *testing.T) {
	cfg := support.EmailConfig{
		FromAddress:   "no-reply@example.com",
		FromName:      "Magpie",
		BrandImageURL: "https://assets.example.com/magpie-email.png",
	}

	subject, body := renderPasswordResetConfirmationEmail(cfg)

	if subject != "Your Magpie password was changed" {
		t.Fatalf("subject = %q", subject)
	}
	for _, want := range []string{
		"<!doctype html>",
		"Your password was changed",
		"https://assets.example.com/magpie-email.png",
		"This email was sent by Magpie",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %q", want, body)
		}
	}
}

func TestForgotPassword_GenericSuccessForUnknownUser(t *testing.T) {
	setupPasswordResetTestDB(t)

	restore := stubPasswordResetConfigForTests(t)
	defer restore()

	sent := false
	queueEmailFn = func(kind, toAddress, subject, body string) error {
		sent = true
		return nil
	}

	body, err := json.Marshal(dto.PasswordResetRequest{Email: "missing@example.com"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/forgotPassword", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	forgotPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if sent {
		t.Fatal("sendEmail was called for a missing user")
	}
}

func TestForgotPassword_RateLimitsByEmailAcrossIPs(t *testing.T) {
	setupPasswordResetTestDB(t)
	resetAuthRateLimitsForTest(t)
	t.Setenv(envAuthForgotPasswordPerEmail, "1")

	hashedPassword, err := support.HashPassword("current-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset@example.com", Password: hashedPassword, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	restore := stubPasswordResetConfigForTests(t)
	defer restore()

	limits := getAuthRateLimits()
	if limits.forgotPerEmail == nil {
		t.Fatal("forgotPerEmail limiter was nil")
	}
	if limits.forgotPerEmail.limit != 1 {
		t.Fatalf("forgotPerEmail limit = %d, want 1", limits.forgotPerEmail.limit)
	}
	if blocked, retryAfter := forgotPasswordEmailBlocked(user.Email); blocked {
		t.Fatalf("forgotPasswordEmailBlocked before first request = true (retryAfter=%s)", retryAfter)
	}

	deliveryCount := 0
	queueEmailFn = func(kind, toAddress, subject, body string) error {
		deliveryCount++
		return nil
	}

	body, err := json.Marshal(dto.PasswordResetRequest{Email: user.Email})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/forgotPassword", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.RemoteAddr = "203.0.113.10:1234"
	firstRec := httptest.NewRecorder()
	forgotPassword(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status code = %d, want %d", firstRec.Code, http.StatusOK)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/forgotPassword", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.RemoteAddr = "198.51.100.20:4321"
	secondRec := httptest.NewRecorder()
	forgotPassword(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status code = %d, want %d", secondRec.Code, http.StatusTooManyRequests)
	}
	if deliveryCount != 1 {
		t.Fatalf("delivery count = %d, want 1", deliveryCount)
	}
}

func TestResetPasswordWithToken_ChangesPasswordAndDeletesTokens(t *testing.T) {
	setupPasswordResetTestDB(t)

	oldHash, err := support.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset-rate@example.com", Password: oldHash, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	rawToken := "plain-reset-token"
	if err := database.DB.Create(&domain.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashPasswordResetToken(rawToken),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	previousRevoke := revokeUserSessions
	revokeUserSessions = func(userID uint) error { return nil }
	t.Cleanup(func() {
		revokeUserSessions = previousRevoke
	})

	if _, err := resetPasswordWithToken(rawToken, "New-password-123"); err != nil {
		t.Fatalf("resetPasswordWithToken returned error: %v", err)
	}

	var updatedUser domain.User
	if err := database.DB.First(&updatedUser, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !support.CheckPasswordHash("New-password-123", updatedUser.Password) {
		t.Fatal("new password hash was not stored")
	}

	var tokenCount int64
	if err := database.DB.Model(&domain.PasswordResetToken{}).Where("user_id = ?", user.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if tokenCount != 0 {
		t.Fatalf("token count = %d, want 0", tokenCount)
	}
}

func TestResetPasswordWithToken_RollsBackWhenSessionRevocationFails(t *testing.T) {
	setupPasswordResetTestDB(t)

	oldHash, err := support.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset@example.com", Password: oldHash, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	rawToken := "rollback-reset-token"
	if err := database.DB.Create(&domain.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashPasswordResetToken(rawToken),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	previousRevoke := revokeUserSessions
	revokeUserSessions = func(userID uint) error { return errors.New("redis down") }
	t.Cleanup(func() {
		revokeUserSessions = previousRevoke
	})

	_, err = resetPasswordWithToken(rawToken, "New-password-123")
	if !errors.Is(err, errRevokeActiveSessions) {
		t.Fatalf("expected errRevokeActiveSessions, got %v", err)
	}

	var updatedUser domain.User
	if err := database.DB.First(&updatedUser, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !support.CheckPasswordHash("old-password", updatedUser.Password) {
		t.Fatal("password changed despite session revocation failure")
	}

	var tokenCount int64
	if err := database.DB.Model(&domain.PasswordResetToken{}).Where("user_id = ?", user.ID).Count(&tokenCount).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if tokenCount != 1 {
		t.Fatalf("token count = %d, want 1", tokenCount)
	}
}

func TestResetPassword_RateLimitsByResolvedEmailAcrossIPs(t *testing.T) {
	setupPasswordResetTestDB(t)
	resetAuthRateLimitsForTest(t)
	t.Setenv(envAuthResetPasswordPerEmail, "1")

	oldHash, err := support.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset@example.com", Password: oldHash, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	rawToken := "rate-limited-reset-token"
	if err := database.DB.Create(&domain.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashPasswordResetToken(rawToken),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("create reset token: %v", err)
	}

	restore := stubPasswordResetConfigForTests(t)
	defer restore()

	limits := getAuthRateLimits()
	if limits.resetPerEmail == nil {
		t.Fatal("resetPerEmail limiter was nil")
	}
	if limits.resetPerEmail.limit != 1 {
		t.Fatalf("resetPerEmail limit = %d, want 1", limits.resetPerEmail.limit)
	}
	if blocked, retryAfter := resetPasswordEmailBlocked(user.Email); blocked {
		t.Fatalf("resetPasswordEmailBlocked before first request = true (retryAfter=%s)", retryAfter)
	}

	firstBody, err := json.Marshal(dto.PasswordResetConfirm{
		Token:       rawToken,
		NewPassword: "short",
	})
	if err != nil {
		t.Fatalf("marshal first request: %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/resetPassword", bytes.NewReader(firstBody))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.RemoteAddr = "203.0.113.30:1234"
	firstRec := httptest.NewRecorder()
	resetPassword(firstRec, firstReq)
	if firstRec.Code != http.StatusBadRequest {
		t.Fatalf("first status code = %d, want %d", firstRec.Code, http.StatusBadRequest)
	}

	secondBody, err := json.Marshal(dto.PasswordResetConfirm{
		Token:       rawToken,
		NewPassword: "New-password-123",
	})
	if err != nil {
		t.Fatalf("marshal second request: %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/resetPassword", bytes.NewReader(secondBody))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.RemoteAddr = "198.51.100.40:4321"
	secondRec := httptest.NewRecorder()
	resetPassword(secondRec, secondReq)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status code = %d, want %d", secondRec.Code, http.StatusTooManyRequests)
	}
}

func setupPasswordResetTestDB(t *testing.T) {
	t.Helper()

	prevDB := database.DB
	t.Cleanup(func() {
		database.DB = prevDB
	})

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := database.SetupDB(func(cfg *database.Config) {
		cfg.ExistingDB = db
		cfg.AutoMigrate = true
		cfg.Migrations = []any{domain.User{}, domain.PasswordResetToken{}, domain.EmailOutbox{}}
		cfg.SeedDefaults = false
	}); err != nil {
		t.Fatalf("setup db: %v", err)
	}
}

func stubPasswordResetConfigForTests(t *testing.T) func() {
	t.Helper()

	previousEmailConfig := readEmailConfigFn
	previousResetConfig := readPasswordResetConfigFn
	previousBuildURL := buildPasswordResetURLFn
	previousQueueEmail := queueEmailFn
	previousGetUserByEmail := getUserByEmailFn
	previousCreateToken := createPasswordResetToken
	previousDeleteTokens := deleteResetTokensForUser
	previousNow := nowFn

	readEmailConfigFn = func() (support.EmailConfig, error) {
		return support.EmailConfig{
			FromAddress:   "no-reply@example.com",
			FromName:      "Magpie",
			SMTPHost:      "smtp.example.com",
			SMTPPort:      587,
			BrandImageURL: "https://assets.example.com/magpie-email.png",
		}, nil
	}
	readPasswordResetConfigFn = func() (support.PasswordResetConfig, error) {
		return support.PasswordResetConfig{PublicAppURL: "https://app.example", TokenTTL: 30 * time.Minute}, nil
	}
	buildPasswordResetURLFn = func(cfg support.PasswordResetConfig, token string) (string, error) {
		return "https://app.example/reset-password?token=" + token, nil
	}
	queueEmailFn = previousQueueEmail
	getUserByEmailFn = database.GetUserByEmail
	createPasswordResetToken = database.CreatePasswordResetToken
	deleteResetTokensForUser = database.DeletePasswordResetTokensForUser
	nowFn = time.Now

	return func() {
		readEmailConfigFn = previousEmailConfig
		readPasswordResetConfigFn = previousResetConfig
		buildPasswordResetURLFn = previousBuildURL
		queueEmailFn = previousQueueEmail
		getUserByEmailFn = previousGetUserByEmail
		createPasswordResetToken = previousCreateToken
		deleteResetTokensForUser = previousDeleteTokens
		nowFn = previousNow
	}
}

func extractTokenFromBody(body string) string {
	const marker = "token="
	idx := strings.Index(body, marker)
	if idx == -1 {
		return ""
	}

	token := body[idx+len(marker):]
	token = strings.TrimSpace(token)
	if end := strings.IndexAny(token, "\"'<& \r\n"); end >= 0 {
		token = token[:end]
	}
	return token
}

func resetAuthRateLimitsForTest(t *testing.T) {
	t.Helper()

	previousOnce := authRateLimitsOnce
	previousLimits := globalAuthLimits
	previousRetryAt := authRedisRetryAt

	t.Setenv("REDIS_URL", "redis://127.0.0.1:1")
	t.Setenv("redisUrl", "redis://127.0.0.1:1")
	_ = support.CloseRedisClient()

	authRateLimitsOnce = sync.Once{}
	globalAuthLimits = authRateLimits{}
	authRedisRetryAt = time.Time{}

	t.Cleanup(func() {
		_ = support.CloseRedisClient()
		authRateLimitsOnce = previousOnce
		globalAuthLimits = previousLimits
		authRedisRetryAt = previousRetryAt
	})
}
