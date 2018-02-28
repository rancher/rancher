package client

const (
	NamespaceStatusType       = "namespaceStatus"
	NamespaceStatusFieldPhase = "phase"
)

type NamespaceStatus struct {
	Phase string `json:"phase,omitempty" yaml:"phase,omitempty"`
}
