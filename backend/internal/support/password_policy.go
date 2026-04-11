package support

import (
	"errors"
	"unicode"
)

const (
	PasswordMinLength = 12
)

var (
	ErrPasswordTooShort      = errors.New("password is too short")
	ErrPasswordMissingLower  = errors.New("password must include a lowercase letter")
	ErrPasswordMissingUpper  = errors.New("password must include an uppercase letter")
	ErrPasswordMissingDigit  = errors.New("password must include a number")
	ErrPasswordContainsSpace = errors.New("password must not contain whitespace")
)

func ValidatePassword(password string) error {
	if len(password) < PasswordMinLength {
		return ErrPasswordTooShort
	}

	var hasLower, hasUpper, hasDigit bool
	for _, r := range password {
		if unicode.IsSpace(r) {
			return ErrPasswordContainsSpace
		}
		if unicode.IsLower(r) {
			hasLower = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}

	switch {
	case !hasLower:
		return ErrPasswordMissingLower
	case !hasUpper:
		return ErrPasswordMissingUpper
	case !hasDigit:
		return ErrPasswordMissingDigit
	default:
		return nil
	}
}

func PasswordValidationMessage() string {
	return "Password must be at least 12 characters long and include uppercase, lowercase, and numeric characters without spaces"
}
