package operations

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
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

func RunSnapshotCreateTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, configMap corev1.ConfigMap, targetNode string) *rkev1.ETCDSnapshot {
	var dumpDebugData = true
	defer func() {
		if dumpDebugData {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			logrus.Errorf("cluster %s etcd snapshot creation operation failed", c.Name)
			logrus.Errorf("cluster %s test data bundle: \n%s", c.Name, data)
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
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot creation", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotCreatePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		dumpDebugData = false
		t.Fatal(err)
	}

	var snapshot *rkev1.ETCDSnapshot
	// Get the etcd snapshot object
	if err := retry.OnError(wait.Backoff{
		Steps:    10,
		Duration: 30 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		if apierrors.IsForbidden(err) {
			return false
		}
		return true
	},
		func() error {
			snapshotsList, err := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, c.Name)})
			if err != nil {
				return err
			}
			var snapshots []*rkev1.ETCDSnapshot
			for _, s := range snapshotsList.Items {
				if s.SnapshotFile.NodeName == targetNode && s.SnapshotFile.Size > 0 {
					snapshots = append(snapshots, s.DeepCopy())
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
	dumpDebugData = false
	return snapshot
}

func RunSnapshotRestoreTest(t *testing.T, clients *clients.Clients, c *v1.Cluster, snapshotName string, expectedConfigMap corev1.ConfigMap, expectedNodeCount int) {
	var dumpDebugData = true
	defer func() {
		if dumpDebugData {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			logrus.Errorf("cluster %s etcd snapshot restore operation failed", c.Name)
			logrus.Errorf("cluster %s test data bundle: \n%s", c.Name, data)
		}
	}()

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
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot restore", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotRestorePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	ns := corev1.NamespaceDefault

	if expectedConfigMap.Namespace != "" {
		ns = expectedConfigMap.Namespace
	}

	// Check for the configmap!
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
	assert.Equal(t, expectedNodeCount, len(allNodes.Items))
}
