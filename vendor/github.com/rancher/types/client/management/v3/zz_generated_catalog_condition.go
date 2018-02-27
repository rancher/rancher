package client

const (
	CatalogConditionType                    = "catalogCondition"
	CatalogConditionFieldLastTransitionTime = "lastTransitionTime"
	CatalogConditionFieldLastUpdateTime     = "lastUpdateTime"
	CatalogConditionFieldMessage            = "message"
	CatalogConditionFieldReason             = "reason"
	CatalogConditionFieldStatus             = "status"
	CatalogConditionFieldType               = "type"
)

type CatalogCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
