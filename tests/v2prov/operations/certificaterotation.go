package operations

import (
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func RotateCertificates(clients *clients.Clients, c *v1.Cluster) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
			Generation: 1,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	})
	if err != nil {
		return err
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rotate certificates", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.CertificateRotationGeneration == 1 && capr.Reconciled.IsTrue(rkeControlPlane.Status), nil
	})
	if err != nil {
		return err
	}

	_, err = cluster.WaitForCreate(clients, c)
	return err
}
