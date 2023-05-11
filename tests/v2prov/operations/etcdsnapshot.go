package operations

import (
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func CreateSnapshot(clients *clients.Clients, c *v1.Cluster) (*rkev1.ETCDSnapshot, error) {
	// Create an etcd snapshot
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
			Generation: 1,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	}); err != nil {
		return nil, err
	}

	_, err := cluster.WaitForControlPlane(clients, c, "etcd snapshot creation", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotCreatePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		return nil, err
	}

	var snapshot *rkev1.ETCDSnapshot
	// Get the etcd snapshot object
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		if apierrors.IsNotFound(err) || err == nil {
			return true
		}
		return false
	},
		func() error {
			snapshots, err := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{})
			if err != nil || len(snapshots.Items) == 0 {
				return err
			}
			snapshot = snapshots.Items[0].DeepCopy()
			return nil
		}); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func RestoreSnapshot(clients *clients.Clients, c *v1.Cluster, snapshotName string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
			Name:             snapshotName,
			Generation:       1,
			RestoreRKEConfig: "none",
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

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot restore", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotRestorePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		return err
	}

	_, err = cluster.WaitForCreate(clients, c)
	return err
}
