package custom

import (
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/cluster"
	"github.com/rancher/rancher/tests/integration/pkg/systemdnode"
	"github.com/stretchr/testify/assert"
)

func TestCustomOneNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 1)
	assert.Equal(t, machines.Items[0].Labels[planner.WorkerRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[planner.ControlPlaneRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[planner.EtcdRoleLabel], "true")
	assert.Len(t, machines.Items[0].Status.Addresses, 2)
}

func TestCustomThreeNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
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
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane")
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 3)
	for _, m := range machines.Items {
		assert.Equal(t, m.Labels[planner.WorkerRoleLabel], "true")
		assert.Equal(t, m.Labels[planner.ControlPlaneRoleLabel], "true")
		assert.Equal(t, m.Labels[planner.EtcdRoleLabel], "true")
	}
}

func TestCustomUniqueRoles(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
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
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd")
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 1; i++ {
		_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane")
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitFor(clients, c)
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
		if m.Labels[planner.WorkerRoleLabel] == "true" {
			worker++
		}
		if m.Labels[planner.ControlPlaneRoleLabel] == "true" {
			controlPlane++
		}
		if m.Labels[planner.EtcdRoleLabel] == "true" {
			etcd++
		}
	}

	assert.Equal(t, worker, 1)
	assert.Equal(t, etcd, 3)
	assert.Equal(t, controlPlane, 1)
}

func TestCustomAddressesProvided(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --etcd --controlplane --address 123.45.67.89 --internal-address 10.42.0.99")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitFor(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	machines, err := cluster.Machines(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, machines.Items, 1)
	assert.Equal(t, machines.Items[0].Labels[planner.WorkerRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[planner.ControlPlaneRoleLabel], "true")
	assert.Equal(t, machines.Items[0].Labels[planner.EtcdRoleLabel], "true")

	assert.Equal(t, machines.Items[0].Annotations[planner.AddressAnnotation], "123.45.67.89")
	assert.Equal(t, machines.Items[0].Annotations[planner.InternalAddressAnnotation], "10.42.0.99")
}
