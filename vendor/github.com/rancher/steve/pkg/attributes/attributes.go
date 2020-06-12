package attributes

import (
	"fmt"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Namespaced(s *types.APISchema) bool {
	if s == nil {
		return false
	}
	return convert.ToBool(s.Attributes["namespaced"])
}

func SetNamespaced(s *types.APISchema, value bool) {
	setVal(s, "namespaced", value)
}

func str(s *types.APISchema, key string) string {
	return convert.ToString(s.Attributes[key])
}

func setVal(s *types.APISchema, key string, value interface{}) {
	if s.Attributes == nil {
		s.Attributes = map[string]interface{}{}
	}
	s.Attributes[key] = value
}

func Group(s *types.APISchema) string {
	return str(s, "group")
}

func SetGroup(s *types.APISchema, value string) {
	setVal(s, "group", value)
}

func Version(s *types.APISchema) string {
	return str(s, "version")
}

func SetVersion(s *types.APISchema, value string) {
	setVal(s, "version", value)
}

func Resource(s *types.APISchema) string {
	return str(s, "resource")
}

func SetResource(s *types.APISchema, value string) {
	setVal(s, "resource", value)
}

func Kind(s *types.APISchema) string {
	return str(s, "kind")
}

func SetKind(s *types.APISchema, value string) {
	setVal(s, "kind", value)
}

func GVK(s *types.APISchema) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   Group(s),
		Version: Version(s),
		Kind:    Kind(s),
	}
}

func SetGVK(s *types.APISchema, gvk schema.GroupVersionKind) {
	SetGroup(s, gvk.Group)
	SetVersion(s, gvk.Version)
	SetKind(s, gvk.Kind)
}

func Table(s *types.APISchema) bool {
	return str(s, "table") != "false"
}

func SetTable(s *types.APISchema, value bool) {
	setVal(s, "table", fmt.Sprint(value))
}

func GVR(s *types.APISchema) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    Group(s),
		Version:  Version(s),
		Resource: Resource(s),
	}
}

func SetGVR(s *types.APISchema, gvk schema.GroupVersionResource) {
	SetGroup(s, gvk.Group)
	SetVersion(s, gvk.Version)
	SetResource(s, gvk.Resource)
}

func Verbs(s *types.APISchema) []string {
	return convert.ToStringSlice(s.Attributes["verbs"])
}

func SetVerbs(s *types.APISchema, verbs []string) {
	setVal(s, "verbs", verbs)
}

func GR(s *types.APISchema) schema.GroupResource {
	return schema.GroupResource{
		Group:    Group(s),
		Resource: Resource(s),
	}
}

func SetGR(s *types.APISchema, gr schema.GroupResource) {
	SetGroup(s, gr.Group)
	SetResource(s, gr.Resource)
}

func SetAccess(s *types.APISchema, access interface{}) {
	setVal(s, "access", access)
}

func Access(s *types.APISchema) interface{} {
	return s.Attributes["access"]
}

func SetAPIResource(s *types.APISchema, resource v1.APIResource) {
	SetResource(s, resource.Name)
	SetVerbs(s, resource.Verbs)
	SetNamespaced(s, resource.Namespaced)
}

func SetColumns(s *types.APISchema, columns interface{}) {
	if s.Attributes == nil {
		s.Attributes = map[string]interface{}{}
	}
	s.Attributes["columns"] = columns
}

func Columns(s *types.APISchema) interface{} {
	return s.Attributes["columns"]
}

func PreferredVersion(s *types.APISchema) string {
	return convert.ToString(s.Attributes["preferredVersion"])
}

func SetPreferredVersion(s *types.APISchema, ver string) {
	if s.Attributes == nil {
		s.Attributes = map[string]interface{}{}
	}
	s.Attributes["preferredVersion"] = ver
}

func PreferredGroup(s *types.APISchema) string {
	return convert.ToString(s.Attributes["preferredGroup"])
}

func SetPreferredGroup(s *types.APISchema, ver string) {
	if s.Attributes == nil {
		s.Attributes = map[string]interface{}{}
	}
	s.Attributes["preferredGroup"] = ver
}
