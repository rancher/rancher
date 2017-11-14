package client

const (
	AllowedHostPathType            = "allowedHostPath"
	AllowedHostPathFieldPathPrefix = "pathPrefix"
)

type AllowedHostPath struct {
	PathPrefix string `json:"pathPrefix,omitempty"`
}
