package client

const (
	ProjectStatusType                  = "projectStatus"
	ProjectStatusFieldBackingNamespace = "backingNamespace"
	ProjectStatusFieldConditions       = "conditions"
)

type ProjectStatus struct {
	BackingNamespace string             `json:"backingNamespace,omitempty" yaml:"backingNamespace,omitempty"`
	Conditions       []ProjectCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
