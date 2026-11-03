package systemtemplate

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"sigs.k8s.io/yaml"
)

// WebhookConfigMapTemplate generates a ConfigMap YAML that carries webhook Helm
// values derived from the cluster's WebhookDeploymentCustomization. The ConfigMap
// is written to cattle-system/rancher-config with a "rancher-webhook" key so that
// the downstream systemcharts controller picks the values up via its existing
// getChartValues("rancher-webhook") path.
//
// When cluster is nil, returns nil, nil. When customization is nil, emits explicit
// chart defaults so the downstream systemcharts controller detects the change and
// resets any previously-customized fields.
func WebhookConfigMapTemplate(cluster *apimgmtv3.Cluster) ([]byte, error) {
	if cluster == nil {
		return nil, nil
	}

	// wdc may be nil — WebhookHelmValues handles that case and returns explicit
	// defaults so the downstream isInstalled merge-patch check detects the diff.
	helmValues, err := chart.WebhookHelmValues(cluster.Spec.WebhookDeploymentCustomization)
	if err != nil {
		return nil, fmt.Errorf("failed to build webhook Helm values: %w", err)
	}

	valuesYAML, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook Helm values to YAML: %w", err)
	}

	cmYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: rancher-config
  namespace: cattle-system
  labels:
    app.kubernetes.io/managed-by: rancher
    app.kubernetes.io/part-of: rancher
data:
  rancher-webhook: |
%s`, indentBlock(string(valuesYAML), 4))

	return []byte(cmYAML), nil
}

// indentBlock indents every line of s by n spaces.
func indentBlock(s string, n int) string {
	prefix := ""
	for i := 0; i < n; i++ {
		prefix += " "
	}
	result := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			result += "\n"
		}
		if line != "" {
			result += prefix + line
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
