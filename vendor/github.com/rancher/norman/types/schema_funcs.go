package types

import (
	"net/http"

	"github.com/rancher/norman/types/slice"
)

func (s *Schema) MustCustomizeField(name string, f func(f Field) Field) *Schema {
	field, ok := s.ResourceFields[name]
	if !ok {
		panic("Failed to find field " + name + " on schema " + s.ID)
	}
	s.ResourceFields[name] = f(field)
	return s
}

func (v *APIVersion) Equals(other *APIVersion) bool {
	return v.Version == other.Version &&
		v.Group == other.Group &&
		v.Path == other.Path
}

func (s *Schema) CanList(context *APIContext) bool {
	if context == nil {
		return slice.ContainsString(s.CollectionMethods, http.MethodGet)
	}
	return context.AccessControl.CanList(context, s)
}

func (s *Schema) CanCreate(context *APIContext) bool {
	if context == nil {
		return slice.ContainsString(s.CollectionMethods, http.MethodPost)
	}
	return context.AccessControl.CanCreate(context, s)
}

func (s *Schema) CanUpdate(context *APIContext) bool {
	if context == nil {
		return slice.ContainsString(s.ResourceMethods, http.MethodPut)
	}
	return context.AccessControl.CanUpdate(context, nil, s)
}

func (s *Schema) CanDelete(context *APIContext) bool {
	if context == nil {
		return slice.ContainsString(s.ResourceMethods, http.MethodDelete)
	}
	return context.AccessControl.CanDelete(context, nil, s)
}
