package operations

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// alternateRKE2KubernetesVersion and alternateK3sKubernetesVersion are synthetic versions used
// to verify that a restore replaces KubernetesVersion before provisioning starts.
const (
	alternateRKE2KubernetesVersion = "v1.36.0+rke2test1"
	alternateK3sKubernetesVersion  = "v1.36.0+k3stest1"
)

func configMapNamespace(configMap corev1.ConfigMap) string {
	if configMap.Namespace != "" {
		return configMap.Namespace
	}
	return corev1.NamespaceDefault
}

func RunSnapshotCreateTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, configMap corev1.ConfigMap, targetNode string) *rkev1.ETCDSnapshot {
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
	if err != nil {
		t.Fatal(err)
	}

	ns := corev1.NamespaceDefault

	if configMap.Namespace != "" {
		ns = configMap.Namespace
	}

	_, err = clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), &configMap, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("system://provisioning/%s/%s", c.Namespace, c.Name)))
	sha := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))[:10]
	ciSAName := "cattle-impersonation-u-" + strings.ToLower(sha)

	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		if apierrors.IsNotFound(err) || err == nil {
			return true
		}
		return false
	}, func() error {
		_, e := clientset.CoreV1().ServiceAccounts("cattle-impersonation-system").Get(context.TODO(), ciSAName, metav1.GetOptions{})
		if e != nil {
			return e
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	snapshotsValidTime := time.Now()

	// Create an etcd snapshot
	var snapshotGeneration int
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if newC.Spec.RKEConfig.ETCDSnapshotCreate == nil {
			snapshotGeneration = 1
		} else {
			snapshotGeneration = newC.Spec.RKEConfig.ETCDSnapshotCreate.Generation + 1
		}
		newC.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
			Generation: snapshotGeneration,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot creation", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotCreate != nil && rkeControlPlane.Status.ETCDSnapshotCreate.Generation == snapshotGeneration && rkeControlPlane.Status.ETCDSnapshotCreatePhase == rkev1.ETCDSnapshotPhaseFinished && capr.Ready.IsTrue(rkeControlPlane), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var snapshot *rkev1.ETCDSnapshot
	re := regexp.MustCompile(".*-([0-9]+)$")
	// Get the etcd snapshot object
	if err := retry.OnError(wait.Backoff{
		Steps:    30,
		Duration: 30 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		return !apierrors.IsForbidden(err)
	},
		func() error {
			snapshotsList, err := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name)})
			if err != nil {
				return err
			}
			var snapshots []*rkev1.ETCDSnapshot
			// Parse the snapshot time from the snapshot file name
			for _, s := range snapshotsList.Items {
				// 1.32.4, 1.31.8, and 1.30.12 onwards set the nodename correctly, so use the annotation to determine the target node.
				// todo: refactor this function to pass both nodename and storage once 1.33.0 is the lowest supported version
				if (s.SnapshotFile.NodeName == targetNode || (targetNode == "s3" && s.Annotations[snapshotbackpopulate.StorageAnnotationKey] == string(snapshotbackpopulate.S3))) && s.SnapshotFile.Size > 0 {
					spec, err := snapshotutil.ParseSnapshotClusterSpecOrError(&s)
					if err != nil || spec == nil {
						continue // ignore errors parsing the snapshot
					}
					// Parse the unix time out of the snapshot file name as CreatedAt is not set on S3 snapshots on older K3s/RKE2 versions
					matches := re.FindStringSubmatch(s.SnapshotFile.Name)
					if len(matches) != 2 {
						continue
					}
					rawTime, err := strconv.ParseInt(matches[1], 10, 64)
					if err != nil {
						continue
					}
					// Snapshot file names only carry second-level precision, so compare against
					// snapshotsValidTime truncated to whole seconds rather than time.Time.After,
					// which would incorrectly exclude a snapshot taken in the same wall-clock
					// second as snapshotsValidTime but before its nanosecond component.
					if rawTime >= snapshotsValidTime.Unix() {
						snapshotTime := metav1.NewTime(time.Unix(rawTime, 0))
						sCopy := s.DeepCopy()
						if sCopy.SnapshotFile.CreatedAt == nil {
							sCopy.SnapshotFile.CreatedAt = &snapshotTime
						}
						snapshots = append(snapshots, sCopy)
					}
				}
			}
			if len(snapshots) > 0 {
				sort.Slice(snapshots, func(i, j int) bool {
					return snapshots[i].SnapshotFile.CreatedAt.Before(snapshots[j].SnapshotFile.CreatedAt)
				})
				snapshot = snapshots[len(snapshots)-1]
				return nil
			}

			return fmt.Errorf("snapshot of target was not found")
		}); err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, snapshot)
	assert.NotEqual(t, "failed", strings.ToLower(snapshot.SnapshotFile.Status))

	err = clientset.CoreV1().ConfigMaps(ns).Delete(context.TODO(), configMap.Name, metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	newCM, expectedErr := clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(expectedErr) {
		t.Fatal(expectedErr)
	}

	// The client will return a configmap object but it will not have anything populated.
	assert.Equal(t, "", newCM.Name)
	return snapshot
}

// RunSnapshotCreateTests creates count distinct snapshots for c.
func RunSnapshotCreateTests(t *testing.T, clients *clients.Clients, c *v1.Cluster, configMap corev1.ConfigMap, targetNode string, count int) []*rkev1.ETCDSnapshot {
	t.Helper()

	if count <= 0 {
		t.Fatal("snapshot count must be positive")
	}

	seen := make(map[string]bool, count)
	snapshots := make([]*rkev1.ETCDSnapshot, 0, count)
	for i := 0; i < count; i++ {
		snapshot := RunSnapshotCreateTest(t, clients, c, configMap, targetNode)
		if snapshot == nil {
			t.Fatalf("snapshot create %d/%d returned a nil snapshot", i+1, count)
		}
		if seen[snapshot.Name] {
			t.Fatalf("snapshot create %d/%d returned duplicate snapshot %q", i+1, count, snapshot.Name)
		}
		seen[snapshot.Name] = true
		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}

func RunSnapshotRestoreTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, snapshotName string, expectedConfigMap corev1.ConfigMap, expectedNodeCount int) {
	t.Helper()
	runSnapshotRestoreTest(t, clients, c, snapshotName, expectedConfigMap, expectedNodeCount, rkev1.RestoreRKEConfigNone, "")
}

// RunSnapshotRestoreTestWithRKEConfig restores a snapshot using restoreRKEConfig. A non-empty
// kubernetesVersion is applied with the restore request to test whether the selected mode restores it.
func RunSnapshotRestoreTestWithRKEConfig(t *testing.T, clients *clients.Clients, c *v1.Cluster, snapshotName string, expectedConfigMap corev1.ConfigMap, expectedNodeCount int, restoreRKEConfig, kubernetesVersion string) {
	t.Helper()
	runSnapshotRestoreTest(t, clients, c, snapshotName, expectedConfigMap, expectedNodeCount, restoreRKEConfig, kubernetesVersion)
}

func runSnapshotRestoreTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, snapshotName string, expectedConfigMap corev1.ConfigMap, expectedNodeCount int, restoreRKEConfig, kubernetesVersion string) {
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

	generation := 1
	missingSnapshotMessage := fmt.Sprintf("etcd restore references missing snapshot %s", snapshotName)
	// Snapshot back-population is asynchronous, so admission can reject a restore request until
	// the corresponding ETCDSnapshot resource is available.
	err := retry.OnError(wait.Backoff{
		Steps:    18,
		Duration: 5 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		return apierrors.IsBadRequest(err) && strings.Contains(err.Error(), missingSnapshotMessage)
	}, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			generation = 1
			if newC.Spec.RKEConfig.ETCDSnapshotRestore != nil {
				generation = newC.Spec.RKEConfig.ETCDSnapshotRestore.Generation + 1
			}
			newC.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
				Name:             snapshotName,
				Generation:       generation,
				RestoreRKEConfig: restoreRKEConfig,
			}
			if kubernetesVersion != "" {
				newC.Spec.KubernetesVersion = kubernetesVersion
			}
			newC, err = clients.Provisioning.Cluster().Update(newC)
			if err != nil {
				return err
			}
			c = newC
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Match the requested generation so stale status from a previous restore cannot satisfy the wait.
	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot restore", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		restoreComplete := rkeControlPlane.Status.ETCDSnapshotRestorePhase == rkev1.ETCDSnapshotPhaseFinished && capr.Ready.IsTrue(rkeControlPlane)
		return rkeControlPlane.Status.ETCDSnapshotRestore != nil &&
			rkeControlPlane.Status.ETCDSnapshotRestore.Generation == generation &&
			restoreComplete, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	ns := configMapNamespace(expectedConfigMap)

	retrievedConfigMap, err := clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), expectedConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expectedConfigMap.Name, retrievedConfigMap.Name)
	assert.Equal(t, expectedConfigMap.Data, retrievedConfigMap.Data)

	allNodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	nonDeletingNodes := 0
	for _, n := range allNodes.Items {
		if n.GetDeletionTimestamp() == nil {
			nonDeletingNodes++
		}
	}
	assert.Equal(t, expectedNodeCount, nonDeletingNodes, "Unexpected number of nodes after restore")
}

// DeleteConfigMapBeforeRestore waits until the ConfigMap is absent from the downstream cluster.
func DeleteConfigMapBeforeRestore(t *testing.T, clients *clients.Clients, c *v1.Cluster, expectedConfigMap corev1.ConfigMap) {
	t.Helper()

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	ns := configMapNamespace(expectedConfigMap)

	if err := clientset.CoreV1().ConfigMaps(ns).Delete(clients.Ctx, expectedConfigMap.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("deleting configmap %s/%s before restore: %v", ns, expectedConfigMap.Name, err)
	}

	err = wait.PollUntilContextTimeout(clients.Ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, getErr := clientset.CoreV1().ConfigMaps(ns).Get(ctx, expectedConfigMap.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) {
			return true, nil
		}
		if getErr != nil {
			return false, getErr
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("waiting for configmap %s/%s to be deleted before restore: %v", ns, expectedConfigMap.Name, err)
	}
}

// SnapshotKubernetesVersion returns the Kubernetes version stored in snapshot metadata.
func SnapshotKubernetesVersion(t *testing.T, snapshot *rkev1.ETCDSnapshot) string {
	t.Helper()

	spec, err := snapshotutil.ParseSnapshotClusterSpecOrError(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if spec.KubernetesVersion == "" {
		t.Fatal("snapshot is missing KubernetesVersion metadata")
	}
	return spec.KubernetesVersion
}

// AlternateKubernetesVersionForSnapshot returns a synthetic version for the same runtime. It must
// only be used in a restore request that replaces KubernetesVersion from snapshot metadata.
func AlternateKubernetesVersionForSnapshot(t *testing.T, snapshotKubernetesVersion string) string {
	t.Helper()

	snapshotRuntime := capr.GetRuntime(snapshotKubernetesVersion)

	var alternateVersion string
	switch snapshotRuntime {
	case capr.RuntimeRKE2:
		alternateVersion = alternateRKE2KubernetesVersion
	case capr.RuntimeK3S:
		alternateVersion = alternateK3sKubernetesVersion
	default:
		t.Fatalf("unsupported runtime %q for snapshot version %q", snapshotRuntime, snapshotKubernetesVersion)
	}

	if alternateVersion == snapshotKubernetesVersion {
		t.Fatalf("alternate Kubernetes version %q must differ from snapshot version %q", alternateVersion, snapshotKubernetesVersion)
	}

	return alternateVersion
}

// SetClusterAdditionalManifest updates the cluster's AdditionalManifest field.
func SetClusterAdditionalManifest(t *testing.T, clients *clients.Clients, c *v1.Cluster, value string) {
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
	if err != nil {
		t.Fatal(err)
	}
}
