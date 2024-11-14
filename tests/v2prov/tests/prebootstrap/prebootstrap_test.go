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

func Test_PreBootstrap_Provisioning_Secret_Sync(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	feature, err := client.Mgmt.Feature().Get("provisioningprebootstrap", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if feature.Spec.Value == nil || *feature.Spec.Value != true {
		t.Fatalf("provisioningprebootstrap flag needs to be enabled for this test to run")
	}

	c, err := cluster.New(client, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret-sync",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Core.Secret().Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-me",
			Namespace: c.Namespace,
			Annotations: map[string]string{
				"provisioning.cattle.io/sync-bootstrap":        "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "hello-ive-been-synced",
				"rke.cattle.io/object-authorized-for-clusters": "test-secret-sync",
			},
		},
		StringData: map[string]string{
			"hello": "world",
			// this is to test that the value gets swapped out properly when synchronized
			"clusterId": "{{clusterId}}",
		},
	})
	defer client.Core.Secret().Delete(c.Namespace, "sync-me", &metav1.DeleteOptions{})

	if err != nil {
		t.Fatal(err)
	}

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

	// reaching to the downstream cluster and making sure the secret was synchronized properly
	sec, err := client.Core.Secret().Get(c.Namespace, fmt.Sprintf("%s-kubeconfig", c.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get kubeconfig for cluster %s", c.Name)
	}

	kubeconfig, ok := sec.Data["value"]
	assert.True(t, ok)

	downstreamConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	assert.NoError(t, err)
	downstreamClient, err := clients.NewForConfig(context.Background(), downstreamConfig)
	if err != nil {
		t.Fatalf("failed to create downstream cluster client")
	}

	synced, err := downstreamClient.Core.Secret().Get("kube-system", "hello-ive-been-synced", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get synchronized downstream secret: %v", err)
	}

	c, err = client.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 2, len(synced.Data))
	assert.Equal(t, "world", string(synced.Data["hello"]))
	assert.Equal(t, c.Status.ClusterName, string(synced.Data["clusterId"]))
}

func Test_PreBootstrap_Provisioning_ACE(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	feature, err := client.Mgmt.Feature().Get("provisioningprebootstrap", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if feature.Spec.Value == nil || *feature.Spec.Value != true {
		t.Fatalf("provisioningprebootstrap flag needs to be enabled for this test to run")
	}

	c, err := cluster.New(client, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret-sync",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
			LocalClusterAuthEndpoint: rkev1.LocalClusterAuthEndpoint{
				Enabled: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Core.Secret().Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-me",
			Namespace: c.Namespace,
			Annotations: map[string]string{
				"provisioning.cattle.io/sync-bootstrap":        "true",
				"provisioning.cattle.io/sync-target-namespace": "kube-system",
				"provisioning.cattle.io/sync-target-name":      "hello-ive-been-synced",
				"rke.cattle.io/object-authorized-for-clusters": "test-secret-sync",
			},
		},
		StringData: map[string]string{
			"hello": "world",
			// this is to test that the value gets swapped out properly when synchronized
			"clusterId": "{{clusterId}}",
		},
	})
	defer client.Core.Secret().Delete(c.Namespace, "sync-me", &metav1.DeleteOptions{})

	if err != nil {
		t.Fatal(err)
	}

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

	// reaching to the downstream cluster and making sure the secret was synchronized properly
	sec, err := client.Core.Secret().Get(c.Namespace, fmt.Sprintf("%s-kubeconfig", c.Name), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get kubeconfig for cluster %s", c.Name)
	}

	kubeconfig, ok := sec.Data["value"]
	assert.True(t, ok)

	downstreamConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	assert.NoError(t, err)
	downstreamClient, err := clients.NewForConfig(context.Background(), downstreamConfig)
	if err != nil {
		t.Fatalf("failed to create downstream cluster client")
	}

	synced, err := downstreamClient.Core.Secret().Get("kube-system", "hello-ive-been-synced", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get synchronized downstream secret: %v", err)
	}

	c, err = client.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 2, len(synced.Data))
	assert.Equal(t, "world", string(synced.Data["hello"]))
	assert.Equal(t, c.Status.ClusterName, string(synced.Data["clusterId"]))
}
