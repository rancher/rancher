package client

const (
	NamespaceResourceQuotaType       = "namespaceResourceQuota"
	NamespaceResourceQuotaFieldLimit = "limit"
)

type NamespaceResourceQuota struct {
	Limit *ResourceQuotaLimit `json:"limit,omitempty" yaml:"limit,omitempty"`
}
