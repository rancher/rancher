package client

const (
	AppStatusType                      = "appStatus"
	AppStatusFieldConditions           = "conditions"
	AppStatusFieldLastAppliedTemplates = "lastAppliedTemplate"
	AppStatusFieldNotes                = "notes"
)

type AppStatus struct {
	Conditions           []AppCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	LastAppliedTemplates string         `json:"lastAppliedTemplate,omitempty" yaml:"lastAppliedTemplate,omitempty"`
	Notes                string         `json:"notes,omitempty" yaml:"notes,omitempty"`
}
