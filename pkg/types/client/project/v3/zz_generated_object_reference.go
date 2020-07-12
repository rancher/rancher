package client

const (
	ObjectReferenceType                 = "objectReference"
	ObjectReferenceFieldAPIVersion      = "apiVersion"
	ObjectReferenceFieldFieldPath       = "fieldPath"
	ObjectReferenceFieldKind            = "kind"
	ObjectReferenceFieldName            = "name"
	ObjectReferenceFieldNamespace       = "namespace"
	ObjectReferenceFieldResourceVersion = "resourceVersion"
	ObjectReferenceFieldUID             = "uid"
)

type ObjectReference struct {
	APIVersion      string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	FieldPath       string `json:"fieldPath,omitempty" yaml:"fieldPath,omitempty"`
	Kind            string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Name            string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty" yaml:"resourceVersion,omitempty"`
	UID             string `json:"uid,omitempty" yaml:"uid,omitempty"`
}
