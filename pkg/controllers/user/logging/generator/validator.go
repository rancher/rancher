package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware/kube-fluentd-operator/config-reloader/fluentd"
)

var (
	recordTransformerType = "record_transformer"
	rubyCodeBlockReg      = regexp.MustCompile(`#\{.*\}`)
	filterAllowFragments  = map[string]int{"record": 1}
	generalAllowFragnent  = map[string]int{"buffer": 1}
)

func ValidateCustomTags(data interface{}) error {
	return validateFragments("filter-custom-tags", "filter", data)
}

func ValidateClusterOutput(data interface{}) error {
	return validateFragments("cluster-match", "match", data)
}

func ValidateClusterSyslogToken(data interface{}) error {
	return validateFragments("cluster-filter-syslog", "filter", data)
}

func ValidateProjectOutput(data interface{}) error {
	return validateFragments("project-match", "match", data)
}

func ValidateProjectSyslogToken(data interface{}) error {
	return validateFragments("project-filter-syslog", "filter", data)
}

func validateFragments(templateName, fragmentName string, data interface{}) error {
	fragments, err := generateFragments(templateName, data)
	if err != nil {
		return errors.Wrapf(err, "generate configure from template %s failed", templateName)
	}

	if err = validateFragmentExist(fragmentName, fragments); err != nil {
		return err
	}

	fragment := fragments[0]
	var allow map[string]int
	switch fragments[0].Type() {
	case recordTransformerType:
		allow = filterAllowFragments
	default:
		allow = generalAllowFragnent
	}

	return validateFragmentsMatchExpected(fragment.Nested, allow)
}

func validateFragmentsMatchExpected(fragments fluentd.Fragment, expected map[string]int) error {
	actual := make(map[string]int)
	for _, v := range fragments {
		actualNum := actual[v.Name] + 1
		expectedNum, ok := expected[v.Name]
		if !ok {
			return errors.New("unexpected configure element: " + v.Name)
		}

		if expectedNum < 0 {
			continue
		}

		if actualNum > expectedNum {
			return errors.New("unexpected configure element: expected " + fmt.Sprint(expectedNum) + " configure element " + v.Name + ", but got " + fmt.Sprint(actualNum))
		}
		actual[v.Name] = actualNum
	}

	return nil
}

func validateFragmentExist(expectedName string, fragments fluentd.Fragment) error {
	if len(fragments) == 0 {
		return errors.New("not configure element found")
	}

	if len(fragments) > 1 {
		return errors.New("expected configure element: " + expectedName + ", detected more than one elements: " + fragments[0].Name + ", " + fragments[1].Name + "...")
	}

	if fragments[0].Name != expectedName {
		return errors.New(fragments[0].Name + "isn't expected configure element" + expectedName)
	}

	return nil
}

func generateFragments(templateName string, data interface{}) (fluentd.Fragment, error) {
	buf, err := GenerateConfig(templateName, data)
	if err != nil {
		return nil, errors.Wrap(err, "generate fluentd configure failed")
	}

	configStr := string(buf)
	if err = filterRubyCode(configStr); err != nil {
		return nil, err
	}

	fragments, err := fluentd.ParseString(configStr)
	if err != nil {
		return nil, errors.Wrap(err, "parse fluentd configure failed")
	}

	return fragments, nil
}

func filterRubyCode(s string) error {
	rubyCodeBlocks := rubyCodeBlockReg.FindStringSubmatch(s)
	if len(rubyCodeBlocks) > 0 {
		return errors.New("not allow embedded Ruby code: " + fmt.Sprintf("%v", rubyCodeBlocks))
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
