package client

const (
	ObjectMetaType                 = "objectMeta"
	ObjectMetaFieldAnnotations     = "annotations"
	ObjectMetaFieldCreated         = "created"
	ObjectMetaFieldFinalizers      = "finalizers"
	ObjectMetaFieldLabels          = "labels"
	ObjectMetaFieldName            = "name"
	ObjectMetaFieldNamespace       = "namespace"
	ObjectMetaFieldOwnerReferences = "ownerReferences"
	ObjectMetaFieldRemoved         = "removed"
	ObjectMetaFieldUuid            = "uuid"
)

type ObjectMeta struct {
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
