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
	APIVersion      string `json:"apiVersion,omitempty"`
	FieldPath       string `json:"fieldPath,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Name            string `json:"name,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	UID             string `json:"uid,omitempty"`
}
