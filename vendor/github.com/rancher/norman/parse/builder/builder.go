package builder

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
)

var (
	Create = Operation("create")
	Update = Operation("update")
	Action = Operation("action")
	List   = Operation("list")
)

type Operation string

type Builder struct {
	Version      *types.APIVersion
	Schemas      *types.Schemas
	RefValidator types.ReferenceValidator
}

func NewBuilder(apiRequest *types.APIContext) *Builder {
	return &Builder{
		Version:      apiRequest.Version,
		Schemas:      apiRequest.Schemas,
		RefValidator: apiRequest.ReferenceValidator,
	}
}

func (b *Builder) Construct(schema *types.Schema, input map[string]interface{}, op Operation) (map[string]interface{}, error) {
	return b.copyFields(schema, input, op)
}

func (b *Builder) copyInputs(schema *types.Schema, input map[string]interface{}, op Operation, result map[string]interface{}) error {
	for fieldName, value := range input {
		field, ok := schema.ResourceFields[fieldName]
		if !ok {
			continue
		}

		if !fieldMatchesOp(field, op) {
			continue
		}

		wasNull := value == nil && (field.Nullable || field.Default == nil)
		value, err := b.convert(field.Type, value, op)
		if err != nil {
			return httperror.WrapFieldAPIError(err, httperror.InvalidFormat, fieldName, err.Error())
		}

		if value != nil || wasNull {
			if op != List {
				if slice, ok := value.([]interface{}); ok {
					for _, sliceValue := range slice {
						if sliceValue == nil {
							return httperror.NewFieldAPIError(httperror.NotNullable, fieldName, "Individual array values can not be null")
						}
						if err := checkFieldCriteria(fieldName, field, sliceValue); err != nil {
							return err
						}
					}
				} else {
					if err := checkFieldCriteria(fieldName, field, value); err != nil {
						return err
					}
				}
			}
			result[fieldName] = value

			if op == List && field.Type == "date" && value != "" {
				ts, err := convert.ToTimestamp(value)
				if err == nil {
					result[fieldName+"TS"] = ts
				}
			}
		}
	}

	if op == List {
		if !convert.IsEmpty(input["type"]) {
			result["type"] = input["type"]
		}
		if !convert.IsEmpty(input["id"]) {
			result["id"] = input["id"]
		}
	}

	return nil
}

func (b *Builder) checkDefaultAndRequired(schema *types.Schema, input map[string]interface{}, op Operation, result map[string]interface{}) error {
	for fieldName, field := range schema.ResourceFields {
		_, hasKey := result[fieldName]
		if op == Create && !hasKey && field.Default != nil {
			result[fieldName] = field.Default
		}

		_, hasKey = result[fieldName]
		if op == Create && fieldMatchesOp(field, Create) && field.Required {
			if !hasKey {
				return httperror.NewFieldAPIError(httperror.MissingRequired, fieldName, "")
			}

			if definition.IsArrayType(field.Type) {
				slice, err := b.convertArray(fieldName, result[fieldName], op)
				if err != nil {
					return err
				}
				if len(slice) == 0 {
					return httperror.NewFieldAPIError(httperror.MissingRequired, fieldName, "")
				}
			}
		}

		if op == List && fieldMatchesOp(field, List) && definition.IsReferenceType(field.Type) && !hasKey {
			result[fieldName] = nil
		}
	}

	return nil
}

func (b *Builder) copyFields(schema *types.Schema, input map[string]interface{}, op Operation) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	if err := b.copyInputs(schema, input, op, result); err != nil {
		return nil, err
	}

	return result, b.checkDefaultAndRequired(schema, input, op, result)
}

func checkFieldCriteria(fieldName string, field types.Field, value interface{}) error {
	numVal, isNum := value.(int64)
	strVal := ""
	hasStrVal := false

	if value == nil && field.Default != nil {
		value = field.Default
	}

	if value != nil {
		hasStrVal = true
		strVal = fmt.Sprint(value)
	}

	if (value == nil || value == "") && !field.Nullable {
		return httperror.NewFieldAPIError(httperror.NotNullable, fieldName, "")
	}

	if isNum {
		if field.Min != nil && numVal < *field.Min {
			return httperror.NewFieldAPIError(httperror.MinLimitExceeded, fieldName, "")
		}
		if field.Max != nil && numVal > *field.Max {
			return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, fieldName, "")
		}
	}

	if hasStrVal {
		if field.MinLength != nil && int64(len(strVal)) < *field.MinLength {
			return httperror.NewFieldAPIError(httperror.MinLengthExceeded, fieldName, "")
		}
		if field.MaxLength != nil && int64(len(strVal)) > *field.MaxLength {
			return httperror.NewFieldAPIError(httperror.MaxLengthExceeded, fieldName, "")
		}
	}

	if len(field.Options) > 0 {
		if hasStrVal || !field.Nullable {
			found := false
			for _, option := range field.Options {
				if strVal == option {
					found = true
					break
				}
			}

			if !found {
				return httperror.NewFieldAPIError(httperror.InvalidOption, fieldName, "")
			}
		}
	}

	if len(field.ValidChars) > 0 && hasStrVal {
		for _, c := range strVal {
			if !strings.ContainsRune(field.ValidChars, c) {
				return httperror.NewFieldAPIError(httperror.InvalidCharacters, fieldName, "")
			}

		}
	}

	if len(field.InvalidChars) > 0 && hasStrVal {
		if strings.ContainsAny(strVal, field.InvalidChars) {
			return httperror.NewFieldAPIError(httperror.InvalidCharacters, fieldName, "")
		}
	}

	return nil
}

func (b *Builder) convert(fieldType string, value interface{}, op Operation) (interface{}, error) {
	if value == nil {
		return value, nil
	}

	switch {
	case definition.IsMapType(fieldType):
		return b.convertMap(fieldType, value, op)
	case definition.IsArrayType(fieldType):
		return b.convertArray(fieldType, value, op)
	case definition.IsReferenceType(fieldType):
		return b.convertReferenceType(fieldType, value)
	}

	switch fieldType {
	case "json":
		return value, nil
	case "date":
		v := convert.ToString(value)
		if v == "" {
			return nil, nil
		}
		return v, nil
	case "boolean":
		return convert.ToBool(value), nil
	case "enum":
		return convert.ToString(value), nil
	case "int":
		return convert.ToNumber(value)
	case "password":
		return convert.ToString(value), nil
	case "string":
		return convert.ToString(value), nil
	case "dnsLabel":
		return convert.ToString(value), nil
	case "intOrString":
		num, err := convert.ToNumber(value)
		if err == nil {
			return num, nil
		}
		return convert.ToString(value), nil
	case "base64":
		return convert.ToString(value), nil
	case "reference":
		return convert.ToString(value), nil
	}

	return b.convertType(fieldType, value, op)
}

func (b *Builder) convertType(fieldType string, value interface{}, op Operation) (interface{}, error) {
	schema := b.Schemas.Schema(b.Version, fieldType)
	if schema == nil {
		return nil, httperror.NewAPIError(httperror.InvalidType, "Failed to find type "+fieldType)
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("Value can not be converted to type %s: %v", fieldType, value))
	}

	return b.Construct(schema, mapValue, op)
}

func (b *Builder) convertReferenceType(fieldType string, value interface{}) (string, error) {
	subType := definition.SubType(fieldType)
	strVal := convert.ToString(value)
	if b.RefValidator != nil && !b.RefValidator.Validate(subType, strVal) {
		return "", httperror.NewAPIError(httperror.InvalidReference, fmt.Sprintf("Not found type: %s id: %s", subType, strVal))
	}
	return strVal, nil
}

func (b *Builder) convertArray(fieldType string, value interface{}, op Operation) ([]interface{}, error) {
	if strSliceValue, ok := value.([]string); ok {
		// Form data will be []string
		var result []interface{}
		for _, value := range strSliceValue {
			result = append(result, value)
		}
		return result, nil
	}

	sliceValue, ok := value.([]interface{})
	if !ok {
		return nil, nil
	}

	result := []interface{}{}
	subType := definition.SubType(fieldType)

	for _, value := range sliceValue {
		val, err := b.convert(subType, value, op)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

func (b *Builder) convertMap(fieldType string, value interface{}, op Operation) (map[string]interface{}, error) {
	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	result := map[string]interface{}{}
	subType := definition.SubType(fieldType)

	for key, value := range mapValue {
		val, err := b.convert(subType, value, op)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.InvalidFormat, err.Error())
		}
		result[key] = val
	}

	return result, nil
}

func fieldMatchesOp(field types.Field, op Operation) bool {
	switch op {
	case Create:
		return field.Create
	case Update:
		return field.Update
	case List:
		return !field.WriteOnly
	default:
		return false
	}
}
