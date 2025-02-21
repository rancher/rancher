package machineprovisioning

import (
	"context"
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/managementuser/snapshotbackpopulate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"os/exec"
	"strings"
	"testing"
	"time"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_Operation_SetB_MP_EtcdSnapshotCleanup
func Test_Operation_SetB_MP_EtcdSnapshotCleanup(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	newNs, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	osInfo, err := objectstore.GetObjectStore(clients, newNs.Name, "store0", "s3snapshots")
	if err != nil {
		t.Fatal(err)
	}

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp-etcd-snapshot-cleanup",
			Namespace: newNs.Name,
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{
				RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
						S3: &rkev1.ETCDSnapshotS3{
							Endpoint:            osInfo.Endpoint,
							EndpointCA:          osInfo.Cert,
							Bucket:              osInfo.Bucket,
							CloudCredentialName: osInfo.CloudCredentialName,
							Folder:              "testfolder",
						},
					},
				},
				MachinePools: []provisioningv1.RKEMachinePool{
					{
						EtcdRole:         true,
						ControlPlaneRole: true,
						WorkerRole:       true,
						Quantity:         &defaults.One,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}

	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "s3")
	assert.NotNil(t, snapshot)

	// check snapshot in s3 and local

	// list s3 snapshots for cluster
	// delete snapshot from s3
	// wait until snapshot is removed from local cluster
	snapshotList, err := clients.RKE.ETCDSnapshot().List(newNs.Name, metav1.ListOptions{})
	assert.NoError(t, err)

	for _, snapshot := range snapshotList.Items {
		assert.NotNil(t, snapshot.Annotations)
		assert.NotEqual(t, snapshot.Annotations[snapshotbackpopulate.StorageAnnotationKey], "")

		storage := snapshotbackpopulate.Storage(snapshot.Annotations[snapshotbackpopulate.StorageAnnotationKey])
		switch storage {
		case snapshotbackpopulate.Local:
			machines, err := clients.CAPI.Machine().List(newNs.Name, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", capr.MachineIDLabel, snapshot.Labels[capr.MachineIDLabel])})
			assert.NoError(t, err)

			assert.Equal(t, 1, len(machines.Items))
			machine := machines.Items[0]

			podMachine, err := clients.Dynamic.Resource(schema.GroupVersionResource{
				Group:    machine.Spec.InfrastructureRef.GroupVersionKind().Group,
				Version:  machine.Spec.InfrastructureRef.GroupVersionKind().Version,
				Resource: strings.ToLower(fmt.Sprintf("%ss", machine.Spec.InfrastructureRef.GroupVersionKind().Kind)),
			}).Namespace(machine.Spec.InfrastructureRef.Namespace).Get(context.TODO(), machine.Spec.InfrastructureRef.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, podMachine)

			podName := strings.ReplaceAll(podMachine.GetName(), ".", "-")

			// delete snapshot from pod
			cmd := exec.Command("kubectl",
				[]string{
					"-n",
					newNs.Name,
					"exec",
					podName,
					"--",
					"rm",
					"-f",
					strings.TrimPrefix(snapshot.SnapshotFile.Location, "file://"),
				}...)
			err = cmd.Run()
			assert.NoError(t, err)
		case snapshotbackpopulate.S3:
			// do nothing
		}
	}

	// delete all first and then check each, realistically only the first one should matter since once the list is
	// refreshed, they should all be deleted, but we could get unlucky and have a refresh between deletions.
	for _, snapshot := range snapshotList.Items {
		if snapshot.SnapshotFile.NodeName == "s3" {
			continue
		}
		if err := retry.OnError(wait.Backoff{
			Duration: 1 * time.Minute,
			Factor:   1.0,
			Jitter:   0.1,
			Steps:    11,
		}, func(err error) bool {
			return true
		}, func() error {
			// wait until the plansecret controller deletes the snapshot
			_, err := clients.RKE.ETCDSnapshot().Get(newNs.Name, snapshot.Name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			} else if err != nil {
				return err
			}
			return errors.New("waiting for snapshot object to delete")
		}); err != nil {
			assert.FailNow(t, err.Error())
		}
	}

}
