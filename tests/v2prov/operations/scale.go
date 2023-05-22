package operations

import (
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func Scale(clients *clients.Clients, c *v1.Cluster, index int, quantity int32, waitForCreate bool) (*v1.Cluster, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.MachinePools[index].Quantity = &quantity
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	})

	if err != nil {
		return c, err
	}

	if waitForCreate {
		return cluster.WaitForCreate(clients, c)
	}
	return c, err
}
