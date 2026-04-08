package chart

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestWebhookHelmValues_nil(t *testing.T) {
	values, err := WebhookHelmValues(nil)
	require.NoError(t, err)
	assert.Nil(t, values)
}

func TestWebhookHelmValues_empty(t *testing.T) {
	values, err := WebhookHelmValues(&v3.WebhookDeploymentCustomization{})
	require.NoError(t, err)
	assert.Empty(t, values)
}

func TestWebhookHelmValues_replicaCount(t *testing.T) {
	rc := int32(3)
	values, err := WebhookHelmValues(&v3.WebhookDeploymentCustomization{
		ReplicaCount: &rc,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), values["replicaCount"])
}

func TestWebhookHelmValues_tolerations(t *testing.T) {
	wdc := &v3.WebhookDeploymentCustomization{
		AppendTolerations: []corev1.Toleration{
			{Key: "cattle.io/node", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
		},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	tolerations, ok := values["tolerations"].([]interface{})
	require.True(t, ok, "tolerations should be []interface{}")
	require.Len(t, tolerations, 1)
	tol := tolerations[0].(map[string]interface{})
	assert.Equal(t, "cattle.io/node", tol["key"])
	assert.Equal(t, string(corev1.TolerationOpExists), tol["operator"])
	assert.Equal(t, string(corev1.TaintEffectNoSchedule), tol["effect"])
}

func TestWebhookHelmValues_affinity(t *testing.T) {
	wdc := &v3.WebhookDeploymentCustomization{
		OverrideAffinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "kubernetes.io/arch", Operator: corev1.NodeSelectorOpIn, Values: []string{"amd64"}},
							},
						},
					},
				},
			},
		},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	affinity, ok := values["affinity"].(map[string]interface{})
	require.True(t, ok, "affinity should be map[string]interface{}")
	assert.Contains(t, affinity, "nodeAffinity")
}

func TestWebhookHelmValues_resources(t *testing.T) {
	wdc := &v3.WebhookDeploymentCustomization{
		OverrideResourceRequirements: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	resources, ok := values["resources"].(map[string]interface{})
	require.True(t, ok, "resources should be map[string]interface{}")
	assert.Contains(t, resources, "requests")
	assert.Contains(t, resources, "limits")
}

func TestWebhookHelmValues_pdb_minAvailable(t *testing.T) {
	wdc := &v3.WebhookDeploymentCustomization{
		PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
			MinAvailable: "1",
		},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	pdb, ok := values["podDisruptionBudget"].(map[string]interface{})
	require.True(t, ok, "podDisruptionBudget should be map[string]interface{}")
	assert.Equal(t, true, pdb["enabled"])
	assert.Equal(t, "1", pdb["minAvailable"])
	assert.NotContains(t, pdb, "maxUnavailable")
}

func TestWebhookHelmValues_pdb_maxUnavailable(t *testing.T) {
	wdc := &v3.WebhookDeploymentCustomization{
		PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
			MaxUnavailable: "1",
		},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	pdb, ok := values["podDisruptionBudget"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, pdb["enabled"])
	assert.Equal(t, "1", pdb["maxUnavailable"])
	assert.NotContains(t, pdb, "minAvailable")
}

func TestWebhookHelmValues_full(t *testing.T) {
	rc := int32(2)
	wdc := &v3.WebhookDeploymentCustomization{
		ReplicaCount: &rc,
		AppendTolerations: []corev1.Toleration{
			{Key: "cattle.io/node", Operator: corev1.TolerationOpExists},
		},
		OverrideAffinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{},
		},
		OverrideResourceRequirements: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
		},
		PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{MinAvailable: "1"},
	}
	values, err := WebhookHelmValues(wdc)
	require.NoError(t, err)

	assert.Equal(t, int32(2), values["replicaCount"])
	assert.Contains(t, values, "tolerations")
	assert.Contains(t, values, "affinity")
	assert.Contains(t, values, "resources")
	pdb := values["podDisruptionBudget"].(map[string]interface{})
	assert.Equal(t, true, pdb["enabled"])
	assert.Equal(t, "1", pdb["minAvailable"])
}
