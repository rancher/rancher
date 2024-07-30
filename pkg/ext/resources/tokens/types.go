package tokens

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	RancherTokenName = "ranchertokens"
)

var SchemeGroupVersion = schema.GroupVersion{Group: "ext.cattle.io", Version: "v1alpha1"}

func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&RancherToken{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

var _ runtime.Object = (*RancherToken)(nil)

type RancherToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RancherTokenSpec   `json:"spec"`
	Status RancherTokenStatus `json:"status"`
}

type RancherTokenSpec struct {
	UserID      string `json:"userID"`
	ClusterName string `json:"clusterName"`
	TTL         string `json:"ttl"`
	Enabled     string `json:"enabled"`
}

type RancherTokenStatus struct {
	PlaintextToken string `json:"plaintextToken,omitempty"`
	HashedToken    string `json:"hashedToken"`
}

func (in *RancherToken) DeepCopyInto(out *RancherToken) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *RancherToken) DeepCopy() *RancherToken {
	if in == nil {
		return nil
	}
	out := new(RancherToken)
	in.DeepCopyInto(out)
	return out
}

func (r *RancherToken) DeepCopyObject() runtime.Object {
	if c := r.DeepCopy(); c != nil {
		return c
	}
	return nil
}
