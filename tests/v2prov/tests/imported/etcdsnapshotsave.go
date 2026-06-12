package imported

import (
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RunETCDSnapshotSaveOperationTest creates an ETCDSnapshotSave operation targeting the given
// clusterRef and waits for it to reach the Succeeded phase. This tests the operation.cattle.io/v1alpha1
// ETCDSnapshotSave controller which manages snapshot lifecycle through beacons and plan secrets.
func RunETCDSnapshotSaveOperationTest(t *testing.T, clients *clients.Clients, namespace string, clusterRef corev1.ObjectReference) *opv1alpha1.ETCDSnapshotSave {
	t.Helper()

	op := &opv1alpha1.ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-snapshot-",
			Namespace:    namespace,
		},
		Spec: opv1alpha1.ETCDSnapshotSaveSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &clusterRef,
				TTL:        60,
			},
		},
	}

	op, err := clients.Operation.ETCDSnapshotSave().Create(op)
	if err != nil {
		t.Fatal(err)
	}

	err = wait.ObjectWithTimeout(clients.Ctx, 5*time.Minute, clients.Operation.ETCDSnapshotSave().Watch, op, func(obj runtime.Object) (bool, error) {
		op = obj.(*opv1alpha1.ETCDSnapshotSave)
		if op.Status.Phase == opv1alpha1.OperationPhaseFailed {
			return false, fmt.Errorf("etcd snapshot create operation failed at step %q", op.Status.Step)
		}
		return op.Status.Phase == opv1alpha1.OperationPhaseSucceeded, nil
	})
	if err != nil {
		handleError(t, clients, clusterRef.Name, err)
	}

	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, op.Status.Phase)
	return op
}
