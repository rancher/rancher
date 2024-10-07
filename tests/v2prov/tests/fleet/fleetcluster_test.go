package fleetcluster_test

import (
	"testing"
	"time"

	fleetv1api "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2prov/clients"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	waitFor = 5 * time.Minute
	tick    = 2 * time.Second
)

var (
	builtinAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "fleet.cattle.io/agent",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		},
	}
	linuxAffinity = corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/os",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"linux"},
							},
						},
					},
				},
			},
		},
	}
	resourceReq = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	tolerations = []corev1.Toleration{
		{
			Key:      "key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
		},
	}
)

func Test_Fleet_Cluster(t *testing.T) {
	require := require.New(t)
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	cluster := &fleetv1api.Cluster{}
	// wait for fleet local cluster with default affinity
	require.Eventually(func() bool {
		cluster, err = clients.Fleet.Cluster().Get("fleet-local", "local", metav1.GetOptions{})
		return err == nil && cluster != nil && cluster.Status.Summary.Ready > 0
	}, waitFor, tick)
	require.Equal(&builtinAffinity, cluster.Spec.AgentAffinity)
	require.Nil(cluster.Spec.AgentResources)
	require.Empty(cluster.Spec.AgentTolerations)

	// fleet-agent deployment has affinity
	agent, err := clients.Apps.StatefulSet().Get(cluster.Status.Agent.Namespace, "fleet-agent", metav1.GetOptions{})
	require.NoError(err)
	require.Equal(&builtinAffinity, agent.Spec.Template.Spec.Affinity)
	for _, container := range agent.Spec.Template.Spec.InitContainers {
		require.Empty(container.Resources)
	}
	require.GreaterOrEqual(len(agent.Spec.Template.Spec.Containers), 1)
	for _, container := range agent.Spec.Template.Spec.Containers {
		require.Empty(container.Resources)
	}
	require.NotEmpty(agent.Spec.Template.Spec.Tolerations) // Fleet has built-in tolerations

	// change settings on management cluster, results should show up in fleet-agent deployment
	mc, err := clients.Mgmt.Cluster().Get("local", metav1.GetOptions{})
	require.NoError(err)

	mc.Spec.FleetAgentDeploymentCustomization = &mgmt.AgentDeploymentCustomization{
		OverrideAffinity:             &linuxAffinity,
		OverrideResourceRequirements: resourceReq,
		AppendTolerations:            tolerations,
	}

	_, err = clients.Mgmt.Cluster().Update(mc)
	require.NoError(err)

	// changes propagate to fleet cluster
	require.Eventually(func() bool {
		cluster, err = clients.Fleet.Cluster().Get("fleet-local", "local", metav1.GetOptions{})
		if err == nil && cluster != nil && cluster.Status.Summary.Ready > 0 {
			assert.Equal(t, &linuxAffinity, cluster.Spec.AgentAffinity)
		}
		return false
	}, waitFor, tick)

	require.Equal(&linuxAffinity, cluster.Spec.AgentAffinity)
	require.Equal(resourceReq, cluster.Spec.AgentResources)
	require.Contains(cluster.Spec.AgentTolerations, tolerations[0])

	// changes are present in deployment
	agent, err = clients.Apps.StatefulSet().Get(cluster.Status.Agent.Namespace, "fleet-agent", metav1.GetOptions{})
	require.NoError(err)
	require.Equal(&linuxAffinity, agent.Spec.Template.Spec.Affinity)
	for _, container := range agent.Spec.Template.Spec.InitContainers {
		require.Equal(resourceReq.Limits, container.Resources.Limits)
		require.Equal(resourceReq.Requests, container.Resources.Requests)
	}
	require.GreaterOrEqual(len(agent.Spec.Template.Spec.Containers), 1)
	for _, container := range agent.Spec.Template.Spec.Containers {
		require.Equal(resourceReq.Limits, container.Resources.Limits)
		require.Equal(resourceReq.Requests, container.Resources.Requests)
	}
	require.Contains(tolerations[0], agent.Spec.Template.Spec.Tolerations)
}
