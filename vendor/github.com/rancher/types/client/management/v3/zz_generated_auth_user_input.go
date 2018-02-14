package client

const (
	AuthUserInputType                = "authUserInput"
	AuthUserInputFieldCode           = "code"
	AuthUserInputFieldRedirectURL    = "redirectUrl"
	AuthUserInputFieldSourceCodeType = "sourceCodeType"
)

type AuthUserInput struct {
	Code           string `json:"code,omitempty"`
	RedirectURL    string `json:"redirectUrl,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty"`
}
