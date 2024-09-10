package custom

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Provisioning_Custom_OneNodeWithDelete(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-one-node",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane --label foo=bar --label ball=life", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 1)
	assert.Equal(t, machines.Items[0].Labels[capr.WorkerRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[capr.ControlPlaneRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[capr.EtcdRoleLabel], "true")
	assert.Len(t, machines.Items[0].Status.Addresses, 2)
	assert.NotNil(t, machines.Items[0].Spec.Bootstrap.ConfigRef)

	secret, err := clients.Core.Secret().Get(machines.Items[0].Namespace, capr.PlanSecretFromBootstrapName(machines.Items[0].Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
	assert.NoError(t, err)

	assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
	var labels map[string]string
	if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
		t.Error(err)
	}
	assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "foo": "bar", "ball": "life"})

	// Delete the cluster and wait for cleanup.
	err = clients.Provisioning.Cluster().Delete(c.Namespace, c.Name, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForDelete(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_Custom_ThreeNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-three-node",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	for i := 0; i < 3; i++ {
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane --label rancher=awesome", map[string]string{"custom-cluster-name": c.Name}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 3)
	for _, m := range machines.Items {
		assert.Equal(t, m.Labels[capr.WorkerRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.ControlPlaneRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.EtcdRoleLabel], "true")
		assert.NotNil(t, machines.Items[0].Spec.Bootstrap.ConfigRef)

		secret, err := clients.Core.Secret().Get(machines.Items[0].Namespace, capr.PlanSecretFromBootstrapName(machines.Items[0].Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
		assert.NoError(t, err)

		assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
		var labels map[string]string
		if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
			t.Error(err)
		}
		assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "rancher": "awesome"})
	}
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_Custom_UniqueRoles(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-unique-roles",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	for i := 0; i < 3; i++ {
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 1; i++ {
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 5)
	var (
		worker       = 0
		controlPlane = 0
		etcd         = 0
	)
	for _, m := range machines.Items {
		if m.Labels[capr.WorkerRoleLabel] == "true" {
			worker++
		}
		if m.Labels[capr.ControlPlaneRoleLabel] == "true" {
			controlPlane++
		}
		if m.Labels[capr.EtcdRoleLabel] == "true" {
			etcd++
		}
	}

	assert.Equal(t, worker, 1)
	assert.Equal(t, etcd, 3)
	assert.Equal(t, controlPlane, 1)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}

func Test_Provisioning_Custom_ThreeNodeWithTaints(t *testing.T) {
	if strings.ToLower(os.Getenv("DIST")) == "rke2" {
		t.Skip()
	}
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-three-node-with-taints",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	for i := 0; i < 3; i++ {
		var taint string
		// Put a taint on one of the nodes.
		if i == 1 {
			taint = " --taint key=value:NoExecute"
		}
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane --label rancher=awesome"+taint, map[string]string{"custom-cluster-name": c.Name}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	var taintFound bool
	assert.Len(t, machines.Items, 3)
	for _, m := range machines.Items {
		assert.Equal(t, m.Labels[capr.WorkerRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.ControlPlaneRoleLabel], "true")
		assert.Equal(t, m.Labels[capr.EtcdRoleLabel], "true")
		assert.NotNil(t, m.Spec.Bootstrap.ConfigRef)

		secret, err := clients.Core.Secret().Get(m.Namespace, capr.PlanSecretFromBootstrapName(m.Spec.Bootstrap.ConfigRef.Name), metav1.GetOptions{})
		assert.NoError(t, err)

		assert.NotEmpty(t, secret.Annotations[capr.LabelsAnnotation])
		var labels map[string]string
		if err := json.Unmarshal([]byte(secret.Annotations[capr.LabelsAnnotation]), &labels); err != nil {
			t.Error(err)
		}
		assert.Equal(t, labels, map[string]string{"cattle.io/os": "linux", "rancher": "awesome"})

		if len(secret.Annotations[capr.TaintsAnnotation]) != 0 {
			// Only one node should have the taint
			assert.False(t, taintFound)

			var taints []corev1.Taint
			if err := json.Unmarshal([]byte(secret.Annotations[capr.TaintsAnnotation]), &taints); err != nil {
				t.Error(err)
			}

			assert.Equal(t, len(taints), 1)
			assert.Equal(t, taints[0].Key, "key")
			assert.Equal(t, taints[0].Value, "value")
			assert.Equal(t, taints[0].Effect, corev1.TaintEffect("NoExecute"))
			taintFound = true
		}
	}

	assert.True(t, taintFound)
	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	assert.NoError(t, err)
}
