package tokens

import (
	"github.com/rancher/wrangler/v3/pkg/schemes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	TokenName = "tokens"
)

var SchemeGroupVersion = schema.GroupVersion{Group: "ext.cattle.io", Version: "v1alpha1"}
var TokenAPIResource = metav1.APIResource{
	Name:         "tokens",
	SingularName: "token",
	Namespaced:   false,
	Kind:         "Token",
	Verbs: metav1.Verbs{
		"create",
		"update",
		"patch",
		"get",
		"list",
		"watch",
		"delete",
	},
}

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
		&Token{},
		&TokenList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func init() {
	schemes.Register(AddToScheme)
}
