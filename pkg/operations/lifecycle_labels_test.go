package operations

import (
	"errors"
	"testing"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func labeledPlanSecret(name string) corev1.Secret {
	return corev1.Secret{Name: name, Namespace: "fleet-default", Labels: map[string]string{
		planv1alpha1.ClusterLifecycleGroupLabel: "management.cattle.io",
		planv1alpha1.ClusterLifecycleKindLabel:  "Cluster",
		planv1alpha1.ClusterLifecycleNameLabel:  "c-abc",
		planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
		planv1alpha1.MachineLifecycleKindLabel:  "Node",
		planv1alpha1.MachineLifecycleNameLabel:  "m-1",
	}}
}

func TestLifecycleLabelsProblem(t *testing.T) {
	ustr := &unstructured.Unstructured{}
	ustr.SetAPIVersion("management.cattle.io/v3")
	ustr.SetKind("Cluster")
	ustr.SetName("c-abc")

	t.Run("labeled secrets pass", func(t *testing.T) {
		a, b := labeledPlanSecret("a"), labeledPlanSecret("b")
		assert.Empty(t, LifecycleLabelsProblem(ustr, []*corev1.Secret{&a, &b}))
	})

	t.Run("no secrets pass", func(t *testing.T) {
		assert.Empty(t, LifecycleLabelsProblem(ustr, nil))
	})

	t.Run("only the group of the clusterRef's apiVersion is compared", func(t *testing.T) {
		secret := labeledPlanSecret("a")
		other := *ustr
		other.SetAPIVersion("management.cattle.io/v4")
		assert.Empty(t, LifecycleLabelsProblem(&other, []*corev1.Secret{&secret}))
	})

	for name, tc := range map[string]struct {
		mislabel func(map[string]string)
		want     string
	}{
		"cluster group missing": {func(l map[string]string) { delete(l, planv1alpha1.ClusterLifecycleGroupLabel) }, "has no plan.cattle.io/cluster-group label"},
		"cluster group differs": {func(l map[string]string) { l[planv1alpha1.ClusterLifecycleGroupLabel] = "cluster.x-k8s.io" }, `has plan.cattle.io/cluster-group="cluster.x-k8s.io", but the operation's cluster is "management.cattle.io"`},
		"cluster kind empty":    {func(l map[string]string) { l[planv1alpha1.ClusterLifecycleKindLabel] = "" }, `has plan.cattle.io/cluster-kind="", but the operation's cluster is "Cluster"`},
		"cluster name differs":  {func(l map[string]string) { l[planv1alpha1.ClusterLifecycleNameLabel] = "c-other" }, `has plan.cattle.io/cluster-name="c-other"`},
		"machine group missing": {func(l map[string]string) { delete(l, planv1alpha1.MachineLifecycleGroupLabel) }, "has no plan.cattle.io/machine-group label"},
		"machine kind empty":    {func(l map[string]string) { l[planv1alpha1.MachineLifecycleKindLabel] = "" }, "has no plan.cattle.io/machine-kind label"},
		"machine name missing":  {func(l map[string]string) { delete(l, planv1alpha1.MachineLifecycleNameLabel) }, "has no plan.cattle.io/machine-name label"},
		"no labels at all":      {func(l map[string]string) { clear(l) }, "has no plan.cattle.io/cluster-group label"},
	} {
		t.Run(name, func(t *testing.T) {
			good, bad := labeledPlanSecret("good"), labeledPlanSecret("bad")
			tc.mislabel(bad.Labels)

			problem := LifecycleLabelsProblem(ustr, []*corev1.Secret{&good, &bad})
			assert.Contains(t, problem, "machine-plan secret fleet-default/bad ")
			assert.Contains(t, problem, tc.want)
		})
	}

	t.Run("the clusterRef must be usable", func(t *testing.T) {
		secret := labeledPlanSecret("a")
		assert.NotEmpty(t, LifecycleLabelsProblem(nil, []*corev1.Secret{&secret}))
		ustr := &unstructured.Unstructured{}
		ustr.SetAPIVersion("a/b/c")
		assert.Contains(t, LifecycleLabelsProblem(ustr, []*corev1.Secret{&secret}), "invalid apiVersion")
	})
}

func TestCheckLifecycleLabels(t *testing.T) {
	t.Run("checks every secret of the cluster", func(t *testing.T) {
		good, bad := labeledPlanSecret("good"), labeledPlanSecret("bad")
		delete(bad.Labels, planv1alpha1.MachineLifecycleNameLabel)

		problem, err := CheckLifecycleLabels(&fakeSecrets{items: []corev1.Secret{good, bad}}, testCluster(), "fleet-default")
		require.NoError(t, err)
		assert.Contains(t, problem, "fleet-default/bad")
	})

	t.Run("passes a labeled cluster", func(t *testing.T) {
		problem, err := CheckLifecycleLabels(&fakeSecrets{items: []corev1.Secret{labeledPlanSecret("a")}}, testCluster(), "fleet-default")
		require.NoError(t, err)
		assert.Empty(t, problem)
	})

	// A failure to reach the secrets may pass on retry, so it is returned rather than rejecting the
	// operation.
	t.Run("returns a listing failure", func(t *testing.T) {
		_, err := CheckLifecycleLabels(&fakeSecrets{listErr: errors.New("boom")}, testCluster(), "fleet-default")
		assert.Error(t, err)
	})
}
