package client

const (
	ChangePasswordInputType                 = "changePasswordInput"
	ChangePasswordInputFieldCurrentPassword = "currentPassword"
	ChangePasswordInputFieldNewPassword     = "newPassword"
)

type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword,omitempty"`
	NewPassword     string `json:"newPassword,omitempty"`
}
