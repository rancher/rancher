package nodedriver

import (
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	FileToFieldAliasesAnno = "nodedriver.cattle.io/file-to-field-aliases"
)


// ParseKeyValueString parses a comma-separated list of "key:value" pairs into a map[string]string
func ParseKeyValueString(input string) map[string]string {
	result := map[string]string{}

	if strings.TrimSpace(input) == "" {
		logrus.Debugf("Empty input string")
		return result
	}

	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		keyVal := strings.Split(pair, ":")
		key := strings.TrimSpace(keyVal[0])
		value := strings.TrimSpace(keyVal[1])
		if len(keyVal) != 2 || key == "" {
			logrus.Errorf("failed to parse pair: %q (expected key:value)", pair)
			continue
		}
		result[key] = value
	}
	return result
}
