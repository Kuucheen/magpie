package database

import (
	"errors"
	"time"

	"magpie/internal/domain"

	"gorm.io/gorm"
)

func GetUserByEmail(email string) (domain.User, error) {
	var user domain.User
	err := DB.Where("email = ?", email).First(&user).Error
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
