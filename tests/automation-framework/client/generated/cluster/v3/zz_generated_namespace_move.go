package client

const (
	NamespaceMoveType           = "namespaceMove"
	NamespaceMoveFieldProjectID = "projectId"
)

type NamespaceMove struct {
	ProjectID string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
}
