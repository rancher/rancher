package client

const (
	SecretReferenceType           = "secretReference"
	SecretReferenceFieldName      = "name"
	SecretReferenceFieldNamespace = "namespace"
)

type SecretReference struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}
