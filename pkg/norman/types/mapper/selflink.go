package mapper

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/wrangler/v3/pkg/name"
)

type SelfLink struct {
	resource string
}

func (s *SelfLink) FromInternal(data map[string]interface{}) {
	if data != nil {
		sl, ok := data["selfLink"].(string)
		if !ok || sl == "" {
			data["selfLink"] = s.selflink(data)
		}
	}
}

func (s *SelfLink) ToInternal(data map[string]interface{}) error {
	return nil
}

func (s *SelfLink) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	s.resource = name.GuessPluralName(strings.ToLower(schema.ID))
	return nil
}

func (s *SelfLink) selflink(data map[string]interface{}) string {
	buf := &strings.Builder{}
	name, ok := data["name"].(string)
	if !ok || name == "" {
		return ""
	}
	apiVersion, ok := data["apiVersion"].(string)
	if !ok || apiVersion == "v1" {
		buf.WriteString("/api/v1/")
	} else {
		buf.WriteString("/apis/")
		buf.WriteString(apiVersion)
		buf.WriteString("/")
	}
	namespace, ok := data["namespace"].(string)
	if ok && namespace != "" {
		buf.WriteString("namespaces/")
		buf.WriteString(namespace)
		buf.WriteString("/")
	}
	buf.WriteString(s.resource)
	buf.WriteString("/")
	buf.WriteString(name)
	return buf.String()
}
