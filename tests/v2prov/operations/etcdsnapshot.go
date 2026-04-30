package operations

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func RunSnapshotRestoreTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, snapshotName string, expectedConfigMap corev1.ConfigMap, expectedNodeCount int, restoreRKEConfig string) {
	t.Helper()

	defer func() {
		if t.Failed() {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			fmt.Printf("cluster %s etcd snapshot restore operation failed\ncluster %s test data bundle: \n%s\n", c.Name, c.Name, data)
		}
	}()

	// Update the cluster spec to trigger restore
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Dynamically increment generation to support multiple restores in one test sequence
		generation := 1
		if newC.Spec.RKEConfig.ETCDSnapshotRestore != nil {
			generation = newC.Spec.RKEConfig.ETCDSnapshotRestore.Generation + 1
		}

		newC.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
			Name:             snapshotName,
			Generation:       generation,
			RestoreRKEConfig: restoreRKEConfig,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	})
	require.NoError(t, err, "Failed to update cluster spec for restore")

	logrus.Infof("Waiting for control plane to start restore type: %s", restoreRKEConfig)

	err = wait.PollUntilContextTimeout(clients.Ctx, 2*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if restore has been acknowledged - the planner has picked up the restore request
		// and set the status to match the spec, with a phase indicating in-progress work
		if cp.Status.ETCDSnapshotRestore != nil &&
			cp.Status.ETCDSnapshotRestore.Generation == c.Spec.RKEConfig.ETCDSnapshotRestore.Generation &&
			cp.Status.ETCDSnapshotRestorePhase != "" &&
			cp.Status.ETCDSnapshotRestorePhase != rkev1.ETCDSnapshotPhaseFinished &&
			cp.Status.ETCDSnapshotRestorePhase != rkev1.ETCDSnapshotPhaseFailed {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err, "Timeout waiting for snapshot restore to start")

	logrus.Infof("Waiting for control plane to complete restore type: %s", restoreRKEConfig)

	var lastStatus string
	err = wait.PollUntilContextTimeout(clients.Ctx, 30*time.Second, 35*time.Minute, true, func(ctx context.Context) (bool, error) {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check specific restore phase and ready status
		if cp.Status.ETCDSnapshotRestorePhase == rkev1.ETCDSnapshotPhaseFinished && capr.Ready.IsTrue(cp) {
			return true, nil
		}

		return false, nil
	})
	require.NoError(t, err, "Timeout waiting for snapshot restore to finish. Last status: %s", lastStatus)

	_, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
	require.NoError(t, err)

	ns := corev1.NamespaceDefault
	if expectedConfigMap.Namespace != "" {
		ns = expectedConfigMap.Namespace
	}

	retrievedConfigMap, err := clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), expectedConfigMap.Name, metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, expectedConfigMap.Name, retrievedConfigMap.Name)
	assert.Equal(t, expectedConfigMap.Data, retrievedConfigMap.Data)

	allNodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)

	nonDeletingNodes := 0
	for _, n := range allNodes.Items {
		if n.GetDeletionTimestamp() == nil {
			nonDeletingNodes++
		}
	}
	assert.Equal(t, expectedNodeCount, nonDeletingNodes, "Unexpected number of nodes after restore")
}

// RunSnapshotCreateTest creates multiple etcd snapshots and returns all of them.
// This is useful for tests that need to perform multiple restores using different snapshots.
func RunSnapshotCreateTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, configMap corev1.ConfigMap, targetNode string, count int) []*rkev1.ETCDSnapshot {
	t.Helper()

	defer func() {
		if t.Failed() {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			fmt.Printf("cluster %s etcd snapshot creation operation failed\ncluster %s test data bundle: \n%s\n", c.Name, c.Name, data)
		}
	}()

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
	require.NoError(t, err)

	ns := corev1.NamespaceDefault
	if configMap.Namespace != "" {
		ns = configMap.Namespace
	}

	_, err = clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), &configMap, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	require.NoError(t, err)

	snapshotsValidTime := time.Now()
	snapshots := make([]*rkev1.ETCDSnapshot, 0, count)
	re := regexp.MustCompile(".*-([0-9]+)$")

	for i := 1; i <= count; i++ {
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			newC.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
				Generation: i,
			}
			newC, err = clients.Provisioning.Cluster().Update(newC)
			if err != nil {
				return err
			}
			c = newC
			return nil
		})
		require.NoError(t, err)

		gen := i
		_, err = cluster.WaitForControlPlane(clients, c, fmt.Sprintf("etcd snapshot creation %d", i), func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
			return rkeControlPlane.Status.ETCDSnapshotCreate != nil &&
				rkeControlPlane.Status.ETCDSnapshotCreate.Generation == gen &&
				rkeControlPlane.Status.ETCDSnapshotCreatePhase == rkev1.ETCDSnapshotPhaseFinished &&
				capr.Ready.IsTrue(rkeControlPlane), nil
		})
		require.NoError(t, err)

		// Wait for snapshot to appear in the list
		var snapshot *rkev1.ETCDSnapshot
		err = wait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			snapshotsList, err := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name),
			})
			if err != nil {
				return false, err
			}
			for _, s := range snapshotsList.Items {
				if (s.SnapshotFile.NodeName == targetNode || (targetNode == "s3" && s.Annotations[snapshotbackpopulate.StorageAnnotationKey] == string(snapshotbackpopulate.S3))) && s.SnapshotFile.Size > 0 {
					matches := re.FindStringSubmatch(s.SnapshotFile.Name)
					if len(matches) != 2 {
						continue
					}
					rawTime, err := strconv.ParseInt(matches[1], 10, 64)
					if err != nil {
						continue
					}
					snapshotTime := time.Unix(rawTime, 0)
					if snapshotTime.After(snapshotsValidTime) {
						// Check if we already have this snapshot
						found := false
						for _, existing := range snapshots {
							if existing.Name == s.Name {
								found = true
								break
							}
						}
						if !found {
							snapshot = s.DeepCopy()
							return true, nil
						}
					}
				}
			}
			return false, nil
		})
		require.NoError(t, err, "Failed to find snapshot %d", i)

		snapshots = append(snapshots, snapshot)
		snapshotsValidTime = time.Now()
	}

	// Delete the configmap after all snapshots are created
	err = clientset.CoreV1().ConfigMaps(ns).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
	require.NoError(t, err)

	return snapshots
}

// ModifyClusterAdditionalManifest updates the AdditionalManifest field on the cluster spec.
// This is used to validate that restore modes properly revert (or preserve) spec changes.
func ModifyClusterAdditionalManifest(t *testing.T, clients *clients.Clients, c *v1.Cluster, value string) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.AdditionalManifest = value
		_, err = clients.Provisioning.Cluster().Update(newC)
		return err
	})
	require.NoError(t, err, "failed to modify cluster AdditionalManifest")
}
