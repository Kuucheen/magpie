package dto

type PasswordResetConfirm struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}
