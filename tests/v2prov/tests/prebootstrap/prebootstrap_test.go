package prebootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_PreBootstrap_Provisioning_Flow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cluster *provisioningv1.Cluster
	}{
		{
			name: "Generic_Secret_Sync",
			cluster: &provisioningv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prebootstrap-secret-sync",
				},
				Spec: provisioningv1.ClusterSpec{
					RKEConfig: &provisioningv1.RKEConfig{},
				},
			},
		},
		{
			name: "ACE",
			cluster: &provisioningv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prebootstrap-ace",
				},
				Spec: provisioningv1.ClusterSpec{
					RKEConfig: &provisioningv1.RKEConfig{},
					LocalClusterAuthEndpoint: rkev1.LocalClusterAuthEndpoint{
						Enabled: true,
					},
				},
			},
		},
	}

	// Run each test case in parallel
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prebootstrapSetupAndCheck(t, tt.cluster)
		})
	}
}

func prebootstrapSetupAndCheck(t *testing.T, c *provisioningv1.Cluster) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	c, err = cluster.New(client, c)
	if err != nil {
		t.Fatal(err)
	}

	feature, err := client.Mgmt.Feature().Get("provisioningprebootstrap", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the feature flag is enabled for the test to run
	// .Status.Default = false by default (harcoded) but gets updated to true if specified in env.
	// otherwise, if it's enabled by the user it'll show up as .Spec.Value
	if !feature.Status.Default && (feature.Spec.Value != nil && !*feature.Spec.Value) {
		t.Fatalf("provisioningprebootstrap flag needs to be enabled for this test to run")
	}

	// Create a couple secrets with sync annotations and data
	_, err = client.Core.Secret().Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-me",
			Namespace: c.Namespace,
			Annotations: map[string]string{
				"provisioning.cattle.io/sync-bootstrap":        "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "hello-ive-been-synced",
				"rke.cattle.io/object-authorized-for-clusters": c.Name,
			},
		},
		StringData: map[string]string{
			"hello": "world",
			// This is to test that the value gets swapped out properly when synchronized.
			"clusterId": "{{clusterId}}",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Core.Secret().Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-me-basic-auth",
			Namespace: c.Namespace,
			Annotations: map[string]string{
				"provisioning.cattle.io/sync-bootstrap":        "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "hello-ive-been-synced-basic-auth",
				"rke.cattle.io/object-authorized-for-clusters": c.Name,
			},
		},
		Type: v1.SecretTypeBasicAuth,
		StringData: map[string]string{
			"username": "admin",
			"password": "admin",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	defer client.Core.Secret().Delete(c.Namespace, "sync-me", &metav1.DeleteOptions{})
	defer client.Core.Secret().Delete(c.Namespace, "sync-me-basic-auth", &metav1.DeleteOptions{})

	command, err := cluster.CustomCommand(client, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(client, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane --label foobar=bazqux", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(client, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(client, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 1)
	m := machines.Items[0]
	assert.Equal(t, m.Labels[capr.WorkerRoleLabel], "true")
	assert.Equal(t, m.Labels[capr.ControlPlaneRoleLabel], "true")
	assert.Equal(t, m.Labels[capr.EtcdRoleLabel], "true")
	assert.NotNil(t, machines.Items[0].Spec.Bootstrap.ConfigRef)

	secret, err := client.Core.Secret().Get(machines.Items[0].Namespace, capr.PlanSecretFromBootstrapName(machines.Items[0].Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
	assert.NoError(t, err)

	assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
	var labels map[string]string
	if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
		t.Error(err)
	}
	assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "foobar": "bazqux"})

	err = cluster.EnsureMinimalConflictsWithThreshold(client, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)

	// Retrieve the kubeconfig for the downstream cluster
	sec, err := client.Core.Secret().Get(c.Namespace, fmt.Sprintf("%s-kubeconfig", c.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get kubeconfig for cluster %s", c.Name)
	}

	kubeconfig, ok := sec.Data["value"]
	assert.True(t, ok)

	// Create a client for the downstream cluster using the kubeconfig
	downstreamConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	assert.NoError(t, err)
	downstreamClient, err := clients.NewForConfig(context.Background(), downstreamConfig)
	if err != nil {
		t.Fatalf("failed to create downstream cluster client")
	}

	// Retrieve the synchronized basic secret from the downstream cluster.
	synced, err := downstreamClient.Core.Secret().Get("kube-system", "hello-ive-been-synced", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get synchronized downstream secret: %v", err)
	}

	// Retrieve the latest state of the cluster object
	c, err = client.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	// ...and finally, assert the synchronized secret has the expected data
	assert.Equal(t, 2, len(synced.Data))
	assert.Equal(t, string(synced.Data["hello"]), "world")
	assert.Equal(t, string(synced.Data["clusterId"]), c.Status.ClusterName)
	assert.Equal(t, synced.Type, v1.SecretTypeOpaque)

	// largely the same as ^ but just checking that the secret `type` field gets synchronized
	basicAuthSecret, err := downstreamClient.Core.Secret().Get("kube-system", "hello-ive-been-synced-basic-auth", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get synchronized downstream secret: %v", err)
	}

	assert.Equal(t, 2, len(basicAuthSecret.Data))
	assert.Equal(t, basicAuthSecret.Type, v1.SecretTypeBasicAuth)
}
