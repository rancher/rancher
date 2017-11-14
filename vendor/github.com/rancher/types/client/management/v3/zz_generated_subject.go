package client

const (
	SubjectType           = "subject"
	SubjectFieldAPIGroup  = "apiGroup"
	SubjectFieldKind      = "kind"
	SubjectFieldName      = "name"
	SubjectFieldNamespace = "namespace"
)

type Subject struct {
	APIGroup  string `json:"apiGroup,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}
