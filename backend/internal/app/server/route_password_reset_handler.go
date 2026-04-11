package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"magpie/internal/api/dto"
	"magpie/internal/auth"
	"magpie/internal/database"
	"magpie/internal/domain"
	"magpie/internal/support"

	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

const forgotPasswordSuccessMessage = "If an account exists for that email, a password reset link has been sent."
const defaultEmailOutboxMaxAttempts = 4

var (
	errPasswordResetUnavailable = errors.New("password reset is unavailable")
	errInvalidResetToken        = errors.New("invalid or expired password reset token")
	errResetPasswordFailed      = errors.New("failed to reset password")

	readEmailConfigFn         = support.ReadEmailConfig
	readPasswordResetConfigFn = support.ReadPasswordResetConfig
	buildPasswordResetURLFn   = support.BuildPasswordResetURL
	queueEmailFn              = func(kind, toAddress, subject, body string) error {
		return database.EnqueueEmailOutbox(kind, toAddress, subject, body, resolveEmailOutboxMaxAttempts())
	}
	getUserByEmailFn         = database.GetUserByEmail
	createPasswordResetToken = database.CreatePasswordResetToken
	deleteResetTokensForUser = database.DeletePasswordResetTokensForUser
	resolveResetTokenEmailFn = lookupPasswordResetTokenEmail
	nowFn                    = time.Now
)

func forgotPassword(w http.ResponseWriter, r *http.Request) {
	var payload dto.PasswordResetRequest
	if !decodeJSONBodyLimited(w, r, &payload, resolveJSONMaxBodyBytes()) {
		return
	}

	emailCfg, resetCfg, err := resolvePasswordRecoveryConfig()
	if err != nil {
		log.Error("password reset request rejected because recovery is unavailable", "error", err)
		writeError(w, "Password recovery is not configured", http.StatusServiceUnavailable)
		return
	}

	email := auth.NormalizeEmail(payload.Email)
	if email == "" {
		writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
		return
	}
	if blocked, retryAfter := forgotPasswordEmailBlocked(email); blocked {
		setRetryAfterHeader(w, retryAfter)
		recordRateLimitBlockMetric("forgot_password_email")
		writeError(w, authForgotPasswordRateMessage, http.StatusTooManyRequests)
		return
	}
	recordForgotPasswordAttempt(email)

	user, err := getUserByEmailFn(email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error("failed to look up user for password reset", "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
		return
	}

	token, err := generatePasswordResetToken()
	if err != nil {
		log.Error("failed to generate password reset token", "user_id", user.ID, "error", err)
		writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
		return
	}

	tokenHash := hashPasswordResetToken(token)
	expiresAt := nowFn().UTC().Add(resetCfg.TokenTTL)
	if err := createPasswordResetToken(user.ID, tokenHash, expiresAt); err != nil {
		log.Error("failed to store password reset token", "user_id", user.ID, "error", err)
		writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
		return
	}

	resetURL, err := buildPasswordResetURLFn(resetCfg, token)
	if err != nil {
		log.Error("failed to build password reset URL", "user_id", user.ID, "error", err)
		writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
		return
	}

	subject, body := renderPasswordResetEmail(emailCfg, resetURL, expiresAt)
	if err := queueEmailFn("password_reset", user.Email, subject, body); err != nil {
		log.Error("failed to queue password reset email", "user_id", user.ID, "error", err)
		if cleanupErr := deleteResetTokensForUser(nil, user.ID); cleanupErr != nil {
			log.Error("failed to clean up password reset token after enqueue failure", "user_id", user.ID, "error", cleanupErr)
		}
		writeError(w, "Password recovery is temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": forgotPasswordSuccessMessage})
}

func resetPassword(w http.ResponseWriter, r *http.Request) {
	var payload dto.PasswordResetConfirm
	if !decodeJSONBodyLimited(w, r, &payload, resolveJSONMaxBodyBytes()) {
		return
	}

	token := strings.TrimSpace(payload.Token)
	if token == "" {
		writeError(w, "Password reset token is required", http.StatusBadRequest)
		return
	}

	if resolvedEmail, err := resolveResetTokenEmailFn(token); err == nil && resolvedEmail != "" {
		if blocked, retryAfter := resetPasswordEmailBlocked(resolvedEmail); blocked {
			setRetryAfterHeader(w, retryAfter)
			recordRateLimitBlockMetric("reset_password_email")
			writeError(w, authResetPasswordRateMessage, http.StatusTooManyRequests)
			return
		}
		recordResetPasswordAttempt(resolvedEmail)
	}

	if err := support.ValidatePassword(payload.NewPassword); err != nil {
		writeError(w, support.PasswordValidationMessage(), http.StatusBadRequest)
		return
	}

	userEmail, err := resetPasswordWithToken(token, payload.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, errInvalidResetToken):
			writeError(w, "Invalid or expired password reset token", http.StatusUnauthorized)
		case errors.Is(err, errHashNewPassword):
			writeError(w, "Failed to hash password", http.StatusInternalServerError)
		case errors.Is(err, errRevokeActiveSessions):
			writeError(w, "Failed to revoke active sessions; password reset was not finalized", http.StatusServiceUnavailable)
		default:
			writeError(w, "Failed to reset password", http.StatusInternalServerError)
		}
		log.Error("password reset failed", "error", err)
		return
	}

	emailCfg, _, cfgErr := resolvePasswordRecoveryConfig()
	if cfgErr == nil && userEmail != "" {
		subject, body := renderPasswordResetConfirmationEmail(emailCfg)
		if err := queueEmailFn("password_reset_confirmation", userEmail, subject, body); err != nil {
			log.Error("failed to queue password reset confirmation email", "email", userEmail, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully"})
}

func resolvePasswordRecoveryConfig() (support.EmailConfig, support.PasswordResetConfig, error) {
	emailCfg, err := readEmailConfigFn()
	if err != nil {
		return support.EmailConfig{}, support.PasswordResetConfig{}, err
	}
	if !emailCfg.IsConfigured() {
		return support.EmailConfig{}, support.PasswordResetConfig{}, errPasswordResetUnavailable
	}

	resetCfg, err := readPasswordResetConfigFn()
	if err != nil {
		return support.EmailConfig{}, support.PasswordResetConfig{}, err
	}
	if _, err := buildPasswordResetURLFn(resetCfg, "health-check-token"); err != nil {
		return support.EmailConfig{}, support.PasswordResetConfig{}, errPasswordResetUnavailable
	}

	return emailCfg, resetCfg, nil
}

func generatePasswordResetToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func hashPasswordResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func renderPasswordResetEmail(cfg support.EmailConfig, resetURL string, expiresAt time.Time) (string, string) {
	subject := "Reset your Magpie password"
	body := fmt.Sprintf(
		"Hello,\n\nWe received a request to reset the password for your Magpie account.\n\nUse this link to choose a new password:\n%s\n\nThis link expires at %s.\nIf you did not request this, you can ignore this email.\n\n%s\n",
		resetURL,
		expiresAt.UTC().Format(time.RFC1123),
		emailSenderDisplayName(cfg),
	)
	return subject, strings.ReplaceAll(body, "\n", "\r\n")
}

func renderPasswordResetConfirmationEmail(cfg support.EmailConfig) (string, string) {
	subject := "Your Magpie password was changed"
	body := fmt.Sprintf(
		"Hello,\n\nThe password for your Magpie account has just been changed.\nIf you did not perform this action, reset your password again immediately and review your account access.\n\n%s\n",
		emailSenderDisplayName(cfg),
	)
	return subject, strings.ReplaceAll(body, "\n", "\r\n")
}

func resolveEmailOutboxMaxAttempts() int {
	attempts := support.GetEnvInt("EMAIL_MAX_ATTEMPTS", defaultEmailOutboxMaxAttempts)
	if attempts <= 0 {
		return defaultEmailOutboxMaxAttempts
	}
	return attempts
}

func lookupPasswordResetTokenEmail(rawToken string) (string, error) {
	tokenHash := hashPasswordResetToken(strings.TrimSpace(rawToken))
	now := nowFn().UTC()

	var row struct {
		Email string
	}
	err := database.DB.
		Table("password_reset_tokens").
		Select("users.email").
		Joins("JOIN users ON users.id = password_reset_tokens.user_id").
		Where("password_reset_tokens.token_hash = ? AND password_reset_tokens.expires_at > ?", tokenHash, now).
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return "", err
	}
	return auth.NormalizeEmail(row.Email), nil
}

func resetPasswordWithToken(rawToken string, newPassword string) (string, error) {
	hashedPassword, err := support.HashPassword(newPassword)
	if err != nil {
		return "", errors.Join(errHashNewPassword, err)
	}

	tokenHash := hashPasswordResetToken(strings.TrimSpace(rawToken))
	now := nowFn().UTC()
	var userEmail string

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var resetToken domain.PasswordResetToken
		if err := tx.Where("token_hash = ? AND expires_at > ?", tokenHash, now).First(&resetToken).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidResetToken
			}
			return err
		}

		result := tx.Where("id = ? AND token_hash = ?", resetToken.ID, tokenHash).Delete(&domain.PasswordResetToken{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errInvalidResetToken
		}

		var user domain.User
		if err := tx.Select("id", "email", "password").First(&user, resetToken.UserID).Error; err != nil {
			return err
		}
		userEmail = user.Email

		if err := tx.Model(&domain.User{}).Where("id = ?", user.ID).Update("password", hashedPassword).Error; err != nil {
			return errors.Join(errResetPasswordFailed, err)
		}

		if err := revokeUserSessions(user.ID); err != nil {
			return errors.Join(errRevokeActiveSessions, err)
		}

		if err := database.DeletePasswordResetTokensForUser(tx, user.ID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errInvalidResetToken) || errors.Is(err, errRevokeActiveSessions) || errors.Is(err, errHashNewPassword) {
			return "", err
		}
		return "", errors.Join(errResetPasswordFailed, err)
	}

	return userEmail, nil
}

func emailSenderDisplayName(cfg support.EmailConfig) string {
	if strings.TrimSpace(cfg.FromName) != "" {
		return cfg.FromName
	}
	return cfg.FromAddress
}
