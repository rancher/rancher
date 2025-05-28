package test

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// This would normally be dynamically created from the API Server.
func NewFakeMapper() *meta.DefaultRESTMapper {
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.AddSpecific(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secret"},
		meta.RESTScopeNamespace)
	mapper.AddSpecific(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "service"},
		meta.RESTScopeNamespace)

	return mapper
}
