package system

import (
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func helmPod(name string, terminated *v1.ContainerStateTerminated) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "cattle-system", ResourceVersion: "1"},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{{
				Name:  "helm",
				State: v1.ContainerState{Terminated: terminated},
			}},
		},
	}
}

// closedWatch returns a watcher whose result channel is already closed, i.e. the API server
// ended the watch without the pod having completed.
func closedWatch() watch.Interface {
	w := watch.NewFake()
	w.Stop()
	return w
}

// completingWatch returns a watcher that delivers one event carrying a completed pod.
func completingWatch(pod *v1.Pod) watch.Interface {
	w := watch.NewFakeWithChanSize(1, false)
	w.Modify(pod)
	w.Stop()
	return w
}

func setOperationTimeout(t *testing.T, value string) {
	t.Helper()
	original := settings.SystemManagedChartsOperationTimeout.Get()
	require.NoError(t, settings.SystemManagedChartsOperationTimeout.Set(value))
	t.Cleanup(func() {
		require.NoError(t, settings.SystemManagedChartsOperationTimeout.Set(original))
	})
}

func TestWaitPodDone(t *testing.T) {
	op := &catalog.Operation{
		Status: catalog.OperationStatus{
			PodNamespace: "cattle-system",
			PodName:      "helm-operation-abc12",
			Chart:        "system-upgrade-controller",
		},
	}

	t.Run("pod already complete needs no watch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pods := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
		pods.EXPECT().Get("cattle-system", "helm-operation-abc12", metav1.GetOptions{}).
			Return(helmPod("helm-operation-abc12", &v1.ContainerStateTerminated{ExitCode: 0}), nil)

		m := &Manager{pods: pods}
		assert.NoError(t, m.waitPodDone(op))
	})

	// The regression: the API server routinely closes a watch before a slow first install
	// finishes. Treating that as a failure made installCharts drop the chart until the next
	// ClusterRepo event, up to an hour later.
	t.Run("watch closing early is retried, not reported as failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pods := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
		running := helmPod("helm-operation-abc12", nil)
		pods.EXPECT().Get("cattle-system", "helm-operation-abc12", metav1.GetOptions{}).Return(running, nil)

		gomock.InOrder(
			pods.EXPECT().Watch("cattle-system", gomock.Any()).Return(closedWatch(), nil),
			pods.EXPECT().Watch("cattle-system", gomock.Any()).
				Return(completingWatch(helmPod("helm-operation-abc12", &v1.ContainerStateTerminated{ExitCode: 0})), nil),
		)

		m := &Manager{pods: pods}
		assert.NoError(t, m.waitPodDone(op))
	})

	t.Run("a failed helm container is reported", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pods := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
		pods.EXPECT().Get("cattle-system", "helm-operation-abc12", metav1.GetOptions{}).
			Return(helmPod("helm-operation-abc12", nil), nil)
		pods.EXPECT().Watch("cattle-system", gomock.Any()).
			Return(completingWatch(helmPod("helm-operation-abc12", &v1.ContainerStateTerminated{ExitCode: 1})), nil)

		m := &Manager{pods: pods}
		err := m.waitPodDone(op)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exited 1")
	})

	t.Run("gives up once the operation timeout elapses", func(t *testing.T) {
		setOperationTimeout(t, "1ms")

		ctrl := gomock.NewController(t)
		pods := fake.NewMockClientInterface[*v1.Pod, *v1.PodList](ctrl)
		pods.EXPECT().Get("cattle-system", "helm-operation-abc12", metav1.GetOptions{}).
			Return(helmPod("helm-operation-abc12", nil), nil)
		pods.EXPECT().Watch("cattle-system", gomock.Any()).Return(closedWatch(), nil).AnyTimes()

		m := &Manager{pods: pods}
		err := m.waitPodDone(op)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timed out after")
	})
}

func TestOperationTimeout(t *testing.T) {
	t.Run("uses the setting", func(t *testing.T) {
		setOperationTimeout(t, "42s")
		assert.Equal(t, 42*time.Second, operationTimeout())
	})

	t.Run("falls back when unparseable", func(t *testing.T) {
		setOperationTimeout(t, "not-a-duration")
		assert.Equal(t, 5*time.Minute, operationTimeout())
	})
}
