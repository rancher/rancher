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

func (s *Schema) CanList() bool {
	return slice.ContainsString(s.CollectionMethods, http.MethodGet)
}

func (s *Schema) CanCreate() bool {
	return slice.ContainsString(s.CollectionMethods, http.MethodPost)
}

func (s *Schema) CanUpdate() bool {
	return slice.ContainsString(s.ResourceMethods, http.MethodPut)
}

func (s *Schema) CanDelete() bool {
	return slice.ContainsString(s.ResourceMethods, http.MethodDelete)
}
