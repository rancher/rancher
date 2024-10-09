package useractivity

import (
	"github.com/rancher/wrangler/v3/pkg/schemes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	UserActivityName = "useractivities"
)

var SchemeGroupVersion = schema.GroupVersion{Group: "ext.cattle.io", Version: "v1"}
var UserActivityAPIResource = metav1.APIResource{
	Name:         "useractivities",
	SingularName: "useractivity",
	Namespaced:   false,
	Kind:         "UserActivity",
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
		&UserActivity{},
		&UserActivityList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func init() {
	schemes.Register(AddToScheme)
}
