package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas"
	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	ErrComplexType = errors.New("complex type")
)

func CheckFieldCriteria(fieldName string, field schemas.Field, value interface{}) error {
	numVal, isNum := value.(int64)
	strVal := ""
	hasStrVal := false

	if value == nil && field.Default != nil {
		value = field.Default
	}

	if value != nil && value != "" {
		hasStrVal = true
		strVal = fmt.Sprint(value)
	}

	if (value == nil || value == "") && !field.Nullable {
		if field.Default == nil {
			return NotNullable
		}
	}

	if isNum {
		if field.Min != nil && numVal < *field.Min {
			return MinLimitExceeded
		}
		if field.Max != nil && numVal > *field.Max {
			return MaxLimitExceeded
		}
	}

	if hasStrVal || value == "" {
		if field.MinLength != nil && int64(len(strVal)) < *field.MinLength {
			return MinLengthExceeded
		}
		if field.MaxLength != nil && int64(len(strVal)) > *field.MaxLength {
			return MaxLengthExceeded
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
				return InvalidOption
			}
		}
	}

	if len(field.ValidChars) > 0 && hasStrVal {
		for _, c := range strVal {
			if !strings.ContainsRune(field.ValidChars, c) {
				return InvalidCharacters
			}

		}
	}

	if len(field.InvalidChars) > 0 && hasStrVal {
		if strings.ContainsAny(strVal, field.InvalidChars) {
			return InvalidCharacters
		}
	}

	return nil
}

func ConvertSimple(fieldType string, value interface{}) (interface{}, error) {
	if value == nil {
		return value, nil
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
	case "float":
		return convert.ToFloat(value)
	case "password":
		return convert.ToString(value), nil
	case "string":
		return convert.ToString(value), nil
	case "dnsLabel":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if errs := validation.IsDNS1123Label(str); len(errs) != 0 {
			return nil, InvalidFormat
		}
		return str, nil
	case "dnsLabelRestricted":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if errs := validation.IsDNS1035Label(str); len(errs) != 0 {
			return value, InvalidFormat
		}
		return str, nil
	case "hostname":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if errs := validation.IsDNS1123Subdomain(str); len(errs) != 0 {
			return value, InvalidFormat
		}
		return str, nil
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

	return nil, ErrComplexType
}
