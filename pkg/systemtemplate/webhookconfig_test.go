package systemtemplate

import (
	"strings"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestWebhookConfigMapTemplate_NilCluster(t *testing.T) {
	out, err := WebhookConfigMapTemplate(nil)
	assert.NoError(t, err)
	assert.Nil(t, out)
}

func TestWebhookConfigMapTemplate_NilCustomization(t *testing.T) {
	cluster := &apimgmtv3.Cluster{}
	out, err := WebhookConfigMapTemplate(cluster)
	assert.NoError(t, err)
	assert.Nil(t, out)
}

func TestWebhookConfigMapTemplate_EmptyCustomization(t *testing.T) {
	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				WebhookDeploymentCustomization: &apimgmtv3.WebhookDeploymentCustomization{},
			},
		},
	}
	out, err := WebhookConfigMapTemplate(cluster)
	assert.NoError(t, err)
	assert.Nil(t, out, "empty customization should produce nil output")
}

func TestWebhookConfigMapTemplate_ReplicaCount(t *testing.T) {
	replicas := int32(3)
	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				WebhookDeploymentCustomization: &apimgmtv3.WebhookDeploymentCustomization{
					ReplicaCount: &replicas,
				},
			},
		},
	}
	out, err := WebhookConfigMapTemplate(cluster)
	require.NoError(t, err)
	require.NotNil(t, out)

	yaml := string(out)
	assert.Contains(t, yaml, "name: rancher-config")
	assert.Contains(t, yaml, "namespace: cattle-system")
	assert.Contains(t, yaml, "rancher-webhook: |")
	assert.Contains(t, yaml, "replicaCount: 3")
}

func TestWebhookConfigMapTemplate_FullCustomization(t *testing.T) {
	replicas := int32(2)
	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				WebhookDeploymentCustomization: &apimgmtv3.WebhookDeploymentCustomization{
					ReplicaCount: &replicas,
					OverrideResourceRequirements: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					PodDisruptionBudget: &apimgmtv3.PodDisruptionBudgetSpec{
						MinAvailable: "1",
					},
				},
			},
		},
	}
	out, err := WebhookConfigMapTemplate(cluster)
	require.NoError(t, err)
	require.NotNil(t, out)

	yaml := string(out)
	assert.Contains(t, yaml, "replicaCount: 2")
	assert.Contains(t, yaml, "enabled: true")
	assert.Contains(t, yaml, "minAvailable:")
}

func TestWebhookConfigMapClearTemplate(t *testing.T) {
	out := WebhookConfigMapClearTemplate()
	yaml := string(out)
	assert.Contains(t, yaml, "name: rancher-config")
	assert.Contains(t, yaml, "namespace: cattle-system")
	assert.Contains(t, yaml, `rancher-webhook: ""`)
}

func TestIndentBlock(t *testing.T) {
	input := "line1\nline2\nline3"
	result := indentBlock(input, 4)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "    "), "each line should be indented by 4 spaces: %q", line)
	}
}
