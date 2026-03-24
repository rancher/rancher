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
// Returns nil, nil when the cluster has no webhook customization set.
func WebhookConfigMapTemplate(cluster *apimgmtv3.Cluster) ([]byte, error) {
	if cluster == nil || cluster.Spec.WebhookDeploymentCustomization == nil {
		return nil, nil
	}

	helmValues, err := chart.WebhookHelmValues(cluster.Spec.WebhookDeploymentCustomization)
	if err != nil {
		return nil, fmt.Errorf("failed to build webhook Helm values: %w", err)
	}
	if len(helmValues) == 0 {
		return nil, nil
	}

	valuesYAML, err := yaml.Marshal(helmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook Helm values to YAML: %w", err)
	}

	// Build the ConfigMap YAML inline. Using server-side apply with a unique
	// field manager lets us own only the "rancher-webhook" key without clobbering
	// other keys that may exist in rancher-config.
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

// WebhookConfigMapClearTemplate generates a ConfigMap YAML that clears the
// "rancher-webhook" key from the rancher-config ConfigMap. This is used when the
// user removes webhook customization so the chart reverts to defaults.
func WebhookConfigMapClearTemplate() []byte {
	return []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: rancher-config
  namespace: cattle-system
  labels:
    app.kubernetes.io/managed-by: rancher
    app.kubernetes.io/part-of: rancher
data:
  rancher-webhook: ""
`)
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
