package generator

import (
	"fmt"
	"strings"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
)

func TestValidateFragments(t *testing.T) {

	projectWrap := ProjectLoggingTemplateWrap{
		ContainerLogSourceTag: loggingconfig.ProjectLevel,
		LoggingCommonField:    v32.LoggingCommonField{},
	}

	clusterWrap := ClusterLoggingTemplateWrap{
		ContainerLogSourceTag: loggingconfig.ClusterLevel,
		LoggingCommonField:    v32.LoggingCommonField{},
	}
	// case:
	// 1. custom tags key include valid custom key value, expected validate success
	customTags := map[string]string{
		"key": "value",
	}
	if err := compareCustomTags(clusterWrap, projectWrap, customTags, ""); err != nil {
		t.Error(err)
		return
	}

	// 2. custom tags key include fluentd configure element, expected validate failed
	customTags = map[string]string{
		"<source>":  "value",
		"</source>": "value",
	}
	if err := compareCustomTags(clusterWrap, projectWrap, customTags, "mismatched tags"); err != nil {
		t.Errorf("validate custom tags key include fluentd configure element should return mismatched tags error, %v", err)
		return
	}

	// 3. custom tags key include embedded Ruby code, expected validate failed
	customTags = map[string]string{
		"key#{Ruby}": "value",
	}
	if err := compareCustomTags(clusterWrap, projectWrap, customTags, "embedded Ruby code"); err != nil {
		t.Errorf("validate custom tags key include embedded Ruby code should return embedded Ruby code error, %v", err)
		return
	}

	// 4. custom tags value include embedded Ruby code, expected validate failed
	customTags = map[string]string{
		"key": "value#{Ruby}",
	}
	if err := compareCustomTags(clusterWrap, projectWrap, customTags, "embedded Ruby code"); err != nil {
		t.Errorf("validate custom tags value include embedded Ruby code should return embedded Ruby code error, %v", err)
		return
	}

	// 5. custom tags value include line break and fluentd configure element, expected be convert to escape string success
	customTags = map[string]string{
		"key": `begin
		</record>
	  </filter>
	  <source>
		@type tail
		path /etc/passwd
		tag injection
		read_from_head true
		<parse>
		  @type none
		</parse>
	  </source>

	  <match injection>
		  @type remote_syslog
		  host rancher.com
		  port 60514
		  severity notice
		  protocol udp
		  program injection
	  </match>

	  <filter injection.**>
		@type record_transformer
		<record>`,
	}

	if err := compareCustomTags(clusterWrap, projectWrap, customTags, ""); err != nil {
		t.Errorf("custom tags value include line break and fluentd configure element should not return err, %v", err)
		return
	}

	// 6. valid custom configure, expected validate success
	content := `
	@type remote_syslog
	host rancher.com
	port 8088
	`
	if err := compareCustomConfigure(clusterWrap, projectWrap, content, ""); err != nil {
		t.Errorf("valid custom configure, expected validate success should not return err, %v", err)
		return
	}

	// 7. custom configure include not allowed configure element, expected validate failed
	content = `
	@type remote_syslog
	host rancher.com
	port 8088
	<source>
	  @type tail
	  path /etc/passwd
	  tag injection
	  read_from_head true
	  <parse>
	    @type none
	  </parse>
	</source>
	`
	if err := compareCustomConfigure(clusterWrap, projectWrap, content, "unexpected configure element"); err != nil {
		t.Errorf("custom configure include not allowed configure element should return error, %v", err)
		return
	}

	// 8. custom configure include embedded Ruby code, expected validate failed
	content = `
	@type remote_syslog
	host rancher.com
	port 8088
	key #{ruby}
	`
	if err := compareCustomConfigure(clusterWrap, projectWrap, content, "embedded Ruby code"); err != nil {
		t.Errorf("custom configure include embedded Ruby code should return embedded Ruby code err, %v", err)
		return
	}
}

func compareCustomTags(clusterWrap ClusterLoggingTemplateWrap, projectWrap ProjectLoggingTemplateWrap, customTags map[string]string, expectedErrMsg string) error {
	var actualErrMsg string
	clusterWrap.OutputTags = customTags
	if err := ValidateCustomTags(clusterWrap); err != nil {
		actualErrMsg = err.Error()
	}

	projectWrap.OutputTags = customTags
	if err := ValidateCustomTags(projectWrap); err != nil {
		actualErrMsg = err.Error()
	}
	if err := compareErr(actualErrMsg, expectedErrMsg); err != nil {
		return err
	}

	return compareErr(actualErrMsg, expectedErrMsg)
}

func compareCustomConfigure(clusterWrap ClusterLoggingTemplateWrap, projectWrap ProjectLoggingTemplateWrap, content, expectedErrMsg string) error {
	var actualErrMsg string
	clusterWrap.Content = content
	clusterWrap.CurrentTarget = loggingconfig.CustomTarget
	if err := ValidateCustomTarget(clusterWrap); err != nil {
		actualErrMsg = err.Error()
	}
	if err := compareErr(actualErrMsg, expectedErrMsg); err != nil {
		return err
	}

	projectWrap.Content = content
	projectWrap.CurrentTarget = loggingconfig.CustomTarget
	if err := ValidateCustomTarget(projectWrap); err != nil {
		actualErrMsg = err.Error()
	}

	return compareErr(actualErrMsg, expectedErrMsg)
}

func compareErr(actualErrMsg, expectedErrMsg string) error {
	if !strings.Contains(actualErrMsg, expectedErrMsg) {
		return errors.New("expected compare message: " + expectedErrMsg + ", actual :" + actualErrMsg)
	}
	return nil
}

func TestEscapeString(t *testing.T) {
	testDate := map[string]string{
		//before: after
		"test":     `"test"`,
		"\"test\"": `"\\\"test\\\""`,
		"\ttest":   `"\\ttest"`,
		"\rtest":   `"\\rtest"`,
		"\ntest":   `"\\ntest"`,
		"\btest":   `"\\btest"`,
		"\ftest":   `"\\ftest"`,
		"\r\ntest": `"\\r\\ntest"`,
		"\\test":   `"\\\\test"`,
	}

	for before, after := range testDate {
		if err := compareEscapeString(before, after); err != nil {
			t.Error(err)
		}
	}
}

func compareEscapeString(input, expected string) error {
	actual := escapeString(input)
	if actual != expected {
		return fmt.Errorf("string %s escape output %s not equal to expected %s", input, actual, expected)
	}
	return nil
}
