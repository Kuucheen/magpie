package database

import (
	"context"
	"errors"
	"time"

	"magpie/internal/domain"

	"gorm.io/gorm"
)

func GetUserByEmail(email string) (domain.User, error) {
	var user domain.User
	err := DB.Where("LOWER(email) = ?", email).First(&user).Error
	return user, err
}

func CreatePasswordResetToken(userID uint, tokenHash string, expiresAt time.Time) error {
	if userID == 0 {
		return errors.New("invalid user id")
	}
	if tokenHash == "" {
		return errors.New("token hash is required")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&domain.PasswordResetToken{}).Error; err != nil {
			return err
		}

		return tx.Create(&domain.PasswordResetToken{
			UserID:    userID,
			TokenHash: tokenHash,
			ExpiresAt: expiresAt.UTC(),
		}).Error
	})
}

func DeletePasswordResetTokensForUser(tx *gorm.DB, userID uint) error {
	if tx == nil {
		tx = DB
	}
	return tx.Where("user_id = ?", userID).Delete(&domain.PasswordResetToken{}).Error
}

func DeleteExpiredPasswordResetTokens(ctx context.Context, before time.Time) (int64, error) {
	tx := DB
	if ctx != nil {
		tx = tx.WithContext(ctx)
	}

	result := tx.Where("expires_at <= ?", before.UTC()).Delete(&domain.PasswordResetToken{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
