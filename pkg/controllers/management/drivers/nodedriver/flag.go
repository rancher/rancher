package nodedriver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

type driverFlagType string

const (
	driverFlagTypeString      = "string"
	driverFlagTypeBoolean     = "boolean"
	driverFlagTypeInt         = "integer"
	driverFlagTypeStringSlice = "stringSlice"
)

var driverFlagToFieldType = map[driverFlagType]string{
	driverFlagTypeString:      "string",
	driverFlagTypeBoolean:     "boolean",
	driverFlagTypeInt:         "string",
	driverFlagTypeStringSlice: "array[string]",
}

type driverFlag struct {
	Type        driverFlagType
	Name        string
	Description string
	Default     any
}

func (f driverFlag) toV3Field() v3.Field {
	res := v3.Field{
		Create:      true,
		Update:      true,
		Type:        driverFlagToFieldType[f.Type],
		Description: f.Description,
	}
	switch f.Type {
	// int flags will appear as a string field in the rancher API, but we are doing this to maintain
	// backward compatibility, at least until we fix a bug that prevents nodeDriver schemas from updating upon
	// a Rancher upgrade
	case driverFlagTypeString, driverFlagTypeInt:
		res.Default.StringValue = f.getStringValue()
	case driverFlagTypeStringSlice:
		res.Default.StringSliceValue = f.getStringSliceValue()
		res.Nullable = true
	}
	return res
}

func (f driverFlag) getStringValue() string {
	if f.Default == nil {
		return ""
	}
	switch v := f.Default.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		// For consistency, legacy implementation only used string or int
		return fmt.Sprintf("%v", v)
	}
}

func (f driverFlag) getStringSliceValue() []string {
	if f.Default == nil {
		return nil
	}
	if v, ok := f.Default.([]string); ok {
		return v
	}
	if slice, ok := f.Default.([]any); ok {
		res := make([]string, 0, len(slice))
		for _, item := range slice {
			if s, ok := item.(string); ok {
				res = append(res, s)
			} else if item != nil {
				res = append(res, fmt.Sprintf("%v", item))
			}
		}
		return res
	}
	return nil
}

// decodeFields unmarshals the JSON payload into a slice of driverFlag structs.
// The caller is responsible for converting these to v3.Field and deriving field names.
func decodeFields(payload []byte) ([]driverFlag, error) {
	var fields []driverFlag
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// fieldNameFromFlag derives the camelCase field name from a driverFlag.
// The flag's Name field is expected to be in the format "driver-flag-name",
// and this function converts it to "flagName" (lowerCamelCase).
func fieldNameFromFlag(f driverFlag) (string, error) {
	return toLowerCamelCase(f.Name)
}

// toLowerCamelCase converts a driver flag name (e.g., "amazonec2-region") to lowerCamelCase (e.g., "region").
func toLowerCamelCase(nodeFlagName string) (string, error) {
	parts := strings.SplitN(nodeFlagName, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("parameter %s does not follow expected naming convention [DRIVER]-[FLAG-NAME]", nodeFlagName)
	}
	flagNameParts := strings.Split(parts[1], "-")
	flagName := flagNameParts[0]
	for _, flagNamePart := range flagNameParts[1:] {
		flagName = flagName + strings.ToUpper(flagNamePart[:1]) + flagNamePart[1:]
	}
	return flagName, nil
}
