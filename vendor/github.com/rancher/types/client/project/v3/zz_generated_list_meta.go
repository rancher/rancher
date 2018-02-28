package client

const (
	ListMetaType                 = "listMeta"
	ListMetaFieldContinue        = "continue"
	ListMetaFieldResourceVersion = "resourceVersion"
	ListMetaFieldSelfLink        = "selfLink"
)

type ListMeta struct {
	Continue        string `json:"continue,omitempty" yaml:"continue,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty" yaml:"resourceVersion,omitempty"`
	SelfLink        string `json:"selfLink,omitempty" yaml:"selfLink,omitempty"`
}
