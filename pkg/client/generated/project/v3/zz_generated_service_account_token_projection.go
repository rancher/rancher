package client

const (
	ServiceAccountTokenProjectionType                   = "serviceAccountTokenProjection"
	ServiceAccountTokenProjectionFieldAudience          = "audience"
	ServiceAccountTokenProjectionFieldExpirationSeconds = "expirationSeconds"
	ServiceAccountTokenProjectionFieldPath              = "path"
	ServiceAccountTokenProjectionFieldUser              = "user"
)

type ServiceAccountTokenProjection struct {
	Audience          string `json:"audience,omitempty" yaml:"audience,omitempty"`
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty" yaml:"expirationSeconds,omitempty"`
	Path              string `json:"path,omitempty" yaml:"path,omitempty"`
	User              *int64 `json:"user,omitempty" yaml:"user,omitempty"`
}
