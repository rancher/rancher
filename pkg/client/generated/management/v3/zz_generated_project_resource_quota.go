package client

const (
	ProjectResourceQuotaType           = "projectResourceQuota"
	ProjectResourceQuotaFieldLimit     = "limit"
	ProjectResourceQuotaFieldUsedLimit = "usedLimit"
)

type ProjectResourceQuota struct {
	Limit     *ResourceQuotaLimit `json:"limit,omitempty" yaml:"limit,omitempty"`
	UsedLimit *ResourceQuotaLimit `json:"usedLimit,omitempty" yaml:"usedLimit,omitempty"`
}
