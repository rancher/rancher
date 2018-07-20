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
	ObjectMetaFieldSelfLink        = "selfLink"
	ObjectMetaFieldUUID            = "uuid"
)

type ObjectMeta struct {
	Annotations     map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Created         string            `json:"created,omitempty" yaml:"created,omitempty"`
	Finalizers      []string          `json:"finalizers,omitempty" yaml:"finalizers,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SelfLink        string            `json:"selfLink,omitempty" yaml:"selfLink,omitempty"`
	UUID            string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
