package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

	user := domain.User{Email: "reset@example.com", Password: hashedPassword, Role: "user"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	restore := stubPasswordResetConfigForTests(t)

	var deliveredTo string
	var deliveredBody string
	sendEmailFn = func(cfg support.EmailConfig, toAddress, subject, body string) error {
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

func TestForgotPassword_GenericSuccessForUnknownUser(t *testing.T) {
	setupPasswordResetTestDB(t)

	restore := stubPasswordResetConfigForTests(t)
	defer restore()

	sent := false
	sendEmailFn = func(cfg support.EmailConfig, toAddress, subject, body string) error {
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

func TestResetPasswordWithToken_ChangesPasswordAndDeletesTokens(t *testing.T) {
	setupPasswordResetTestDB(t)

	oldHash, err := support.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := domain.User{Email: "reset@example.com", Password: oldHash, Role: "user"}
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

	if err := resetPasswordWithToken(rawToken, "new-password-123"); err != nil {
		t.Fatalf("resetPasswordWithToken returned error: %v", err)
	}

	var updatedUser domain.User
	if err := database.DB.First(&updatedUser, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !support.CheckPasswordHash("new-password-123", updatedUser.Password) {
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

	err = resetPasswordWithToken(rawToken, "new-password-123")
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
		cfg.Migrations = []any{domain.User{}, domain.PasswordResetToken{}}
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
	previousSendEmail := sendEmailFn
	previousGetUserByEmail := getUserByEmailFn
	previousCreateToken := createPasswordResetToken
	previousNow := nowFn

	readEmailConfigFn = func() (support.EmailConfig, error) {
		return support.EmailConfig{
			FromAddress: "no-reply@example.com",
			FromName:    "Magpie",
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
		}, nil
	}
	readPasswordResetConfigFn = func() (support.PasswordResetConfig, error) {
		return support.PasswordResetConfig{PublicAppURL: "https://app.example", TokenTTL: 30 * time.Minute}, nil
	}
	buildPasswordResetURLFn = func(cfg support.PasswordResetConfig, r *http.Request, token string) (string, error) {
		return "https://app.example/reset-password?token=" + token, nil
	}
	sendEmailFn = previousSendEmail
	getUserByEmailFn = database.GetUserByEmail
	createPasswordResetToken = database.CreatePasswordResetToken
	nowFn = time.Now

	return func() {
		readEmailConfigFn = previousEmailConfig
		readPasswordResetConfigFn = previousResetConfig
		buildPasswordResetURLFn = previousBuildURL
		sendEmailFn = previousSendEmail
		getUserByEmailFn = previousGetUserByEmail
		createPasswordResetToken = previousCreateToken
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
	if newline := strings.IndexAny(token, "\r\n"); newline >= 0 {
		token = token[:newline]
	}
	return token
}
