package mapper

import (
	"strings"

	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
)

type NamespaceReference struct {
	fields      [][]string
	VersionPath string
}

func (n *NamespaceReference) FromInternal(data map[string]interface{}) {
	namespaceID, ok := data["namespaceId"]
	if ok {
		for _, path := range n.fields {
			convert.Transform(data, path, func(input interface{}) interface{} {
				parts := strings.SplitN(convert.ToString(input), ":", 2)
				if len(parts) == 2 {
					return input
				}
				return fmt.Sprintf("%s:%v", namespaceID, input)
			})
		}
	}
}

func (n *NamespaceReference) ToInternal(data map[string]interface{}) error {
	namespaceID, ok := data["namespaceId"]
	for _, path := range n.fields {
		convert.Transform(data, path, func(input interface{}) interface{} {
			parts := strings.SplitN(convert.ToString(input), ":", 2)
			if len(parts) == 2 && (!ok || parts[0] == namespaceID) {
				return parts[1]
			}
			return input
		})
	}

	return nil
}

func (n *NamespaceReference) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	_, hasNamespace := schema.ResourceFields["namespaceId"]
	if schema.Version.Path != n.VersionPath || !hasNamespace {
		return nil
	}
	n.fields = traverse(nil, schema, schemas)
	return nil
}

func traverse(prefix []string, schema *types.Schema, schemas *types.Schemas) [][]string {
	var result [][]string

	for name, field := range schema.ResourceFields {
		localPrefix := []string{name}
		subType := field.Type
		if definition.IsArrayType(field.Type) {
			localPrefix = append(localPrefix, "{ARRAY}")
			subType = definition.SubType(field.Type)
		} else if definition.IsMapType(field.Type) {
			localPrefix = append(localPrefix, "{MAP}")
			subType = definition.SubType(field.Type)
		}
		if definition.IsReferenceType(subType) {
			result = appendReference(result, prefix, localPrefix, field, schema, schemas)
			continue
		}

		subSchema := schemas.Schema(&schema.Version, subType)
		if subSchema != nil {
			result = append(result, traverse(append(prefix, localPrefix...), subSchema, schemas)...)
		}
	}

	return result
}

func appendReference(result [][]string, prefix []string, name []string, field types.Field, schema *types.Schema, schemas *types.Schemas) [][]string {
	targetSchema := schemas.Schema(&schema.Version, definition.SubType(field.Type))
	if targetSchema != nil && targetSchema.Scope == types.NamespaceScope {
		result = append(result, append(prefix, name...))
	}
	return result
}
