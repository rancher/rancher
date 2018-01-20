package client

const (
	SetPasswordInputType             = "setPasswordInput"
	SetPasswordInputFieldNewPassword = "newPassword"
)

type SetPasswordInput struct {
	NewPassword string `json:"newPassword,omitempty"`
}
