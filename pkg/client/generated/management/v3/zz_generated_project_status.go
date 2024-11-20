package client

const (
	ProjectStatusType            = "projectStatus"
	ProjectStatusFieldConditions = "conditions"
)

type ProjectStatus struct {
	Conditions []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
