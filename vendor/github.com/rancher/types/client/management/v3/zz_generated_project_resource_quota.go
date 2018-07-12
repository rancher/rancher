package client

const (
	ProjectResourceQuotaType           = "projectResourceQuota"
	ProjectResourceQuotaFieldLimit     = "limit"
	ProjectResourceQuotaFieldUsedLimit = "usedLimit"
)

type ProjectResourceQuota struct {
	Limit     *ProjectResourceLimit `json:"limit,omitempty" yaml:"limit,omitempty"`
	UsedLimit *ProjectResourceLimit `json:"usedLimit,omitempty" yaml:"usedLimit,omitempty"`
}
