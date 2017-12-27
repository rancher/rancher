package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func Singular(value interface{}) interface{} {
	if slice, ok := value.([]string); ok {
		if len(slice) == 0 {
			return nil
		}
		return slice[0]
	}
	if slice, ok := value.([]interface{}); ok {
		if len(slice) == 0 {
			return nil
		}
		return slice[0]
	}
	return value
}

func ToString(value interface{}) string {
	single := Singular(value)
	if single == nil {
		return ""
	}
	return fmt.Sprint(single)
}

func ToTimestamp(value interface{}) (int64, error) {
	str := ToString(value)
	if str == "" {
		return 0, errors.New("Invalid date")
	}
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return 0, err
	}
	return t.UnixNano() / 1000000, nil
}

func ToBool(value interface{}) bool {
	value = Singular(value)

	b, ok := value.(bool)
	if ok {
		return b
	}

	str := strings.ToLower(ToString(value))
	return str == "true" || str == "t" || str == "yes" || str == "y"
}

func ToNumber(value interface{}) (int64, error) {
	value = Singular(value)

	i, ok := value.(int64)
	if ok {
		return i, nil
	}
	return strconv.ParseInt(ToString(value), 10, 64)
}

func Capitalize(s string) string {
	if len(s) <= 1 {
		return strings.ToUpper(s)
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

func Uncapitalize(s string) string {
	if len(s) <= 1 {
		return strings.ToLower(s)
	}

	return strings.ToLower(s[:1]) + s[1:]
}

func LowerTitle(input string) string {
	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) &&
			(i == 0 ||
				i == len(runes)-1 ||
				unicode.IsUpper(runes[i+1])) {
			runes[i] = unicode.ToLower(runes[i])
		} else {
			break
		}
	}

	return string(runes)
}

func IsEmpty(v interface{}) bool {
	return v == nil || v == "" || v == 0 || v == false
}

func ToMapInterface(obj interface{}) map[string]interface{} {
	v, _ := obj.(map[string]interface{})
	return v
}

func ToMapSlice(obj interface{}) []map[string]interface{} {
	if v, ok := obj.([]map[string]interface{}); ok {
		return v
	}
	vs, _ := obj.([]interface{})
	result := []map[string]interface{}{}
	for _, item := range vs {
		if v, ok := item.(map[string]interface{}); ok {
			result = append(result, v)
		} else {
			return nil
		}
	}

	return result
}

func ToStringSlice(data interface{}) []string {
	if v, ok := data.([]string); ok {
		return v
	}
	if v, ok := data.([]interface{}); ok {
		result := []string{}
		for _, item := range v {
			result = append(result, ToString(item))
		}
		return result
	}
	return nil
}

func ToObj(data interface{}, into interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, into)
}

func EncodeToMap(obj interface{}) (map[string]interface{}, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	return result, json.Unmarshal(bytes, &result)
}
