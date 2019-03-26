package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var (
	rubyCodeBlockReg = regexp.MustCompile(`#\{.*\}`)
)

func ValidateCustomTags(tags map[string]string, enableEscaped bool) (map[string]string, error) {
	newTags := make(map[string]string)
	for key, value := range tags {
		if err := filterFluentdTags(key); err != nil {
			return nil, err
		}

		if err := filterRubyCode(value); err != nil {
			return nil, err
		}

		if enableEscaped {
			newValue := escapeString(value)
			newTags[key] = newValue
		}
	}

	return newTags, nil
}

func filterFluentdTags(key string) error {
	invalidTagKeys := []string{
		"@include",
		"<source", "<parse", "<filter", "<format", "<storage", "<buffer", "<match", "<record", "<system", "<label", "<route",
		"</source>", "</parse>", "</filter>", "</format>", "</storage>", "</buffer>", "</match>", "</record>", "</system>", "</label>", "</route>",
	}

	for _, invalidKey := range invalidTagKeys {
		if strings.Contains(key, invalidKey) {
			return errors.New("invalid custom tag key: " + key)
		}
	}

	return nil
}

func filterRubyCode(s string) error {
	rubyCodeBlocks := rubyCodeBlockReg.FindStringSubmatch(s)
	if len(rubyCodeBlocks) > 0 {
		return errors.New("invalid custom field value: " + fmt.Sprintf("%v", rubyCodeBlocks))
	}
	return nil
}

func escapeString(postDoc string) string {
	var escapeReplacer = strings.NewReplacer(
		"\t", `\\t`,
		"\n", `\\n`,
		"\r", `\\r`,
		"\f", `\\f`,
		"\b", `\\b`,
		"\"", `\\\"`,
		"\\", `\\\\`,
	)

	escapeString := escapeReplacer.Replace(postDoc)
	return fmt.Sprintf(`"%s"`, escapeString)
}
