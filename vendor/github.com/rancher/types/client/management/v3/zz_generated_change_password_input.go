package client

const (
	ChangePasswordInputType             = "changePasswordInput"
	ChangePasswordInputFieldNewPassword = "newPassword"
)

type ChangePasswordInput struct {
	NewPassword string `json:"newPassword,omitempty"`
}
