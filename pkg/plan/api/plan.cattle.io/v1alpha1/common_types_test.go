package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mapperForTests returns a RESTMapper seeded with the GroupKinds these tests use. Scope for
// management.cattle.io Cluster is Root (cluster-scoped) so the namespace-override rule can be
// verified independently of any real k8s API server. The defaultGroupVersions argument to
// NewDefaultRESTMapper is what lets RESTMapping(gk) (with no version) resolve to a mapping.
func mapperForTests() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "cluster.x-k8s.io", Version: "v1beta2"},
		{Group: "provisioning.cattle.io", Version: "v1"},
		{Group: "management.cattle.io", Version: "v3"},
	})
	m.Add(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Machine"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "Cluster"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "Node"}, meta.RESTScopeNamespace)
	return m
}

// typedObj embeds ObjectMeta and TypeMeta so ObjTo*LifecycleLabels can read a non-empty GVK from
// obj.GetObjectKind() (cache-fetched typed objects have empty TypeMeta in production).
type typedObj struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (t *typedObj) DeepCopyObject() runtime.Object { return t }

func TestObjToMachineLifecycleLabels(t *testing.T) {
	obj := &typedObj{
		TypeMeta:   metav1.TypeMeta{APIVersion: "cluster.x-k8s.io/v1beta2", Kind: "Machine"},
		ObjectMeta: metav1.ObjectMeta{Name: "m0", Namespace: "fleet-default"},
	}
	got, err := ObjToMachineLifecycleLabels(obj)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		MachineLifecycleGroupLabel: "cluster.x-k8s.io",
		MachineLifecycleKindLabel:  "Machine",
		MachineLifecycleNameLabel:  "m0",
	}, got)
}

func TestObjToClusterLifecycleLabels(t *testing.T) {
	obj := &typedObj{
		TypeMeta:   metav1.TypeMeta{APIVersion: "management.cattle.io/v3", Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "c-abc123"},
	}
	got, err := ObjToClusterLifecycleLabels(obj)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		ClusterLifecycleGroupLabel: "management.cattle.io",
		ClusterLifecycleKindLabel:  "Cluster",
		ClusterLifecycleNameLabel:  "c-abc123",
	}, got)
}

func TestHasMachineLifecycleLabels(t *testing.T) {
	full := map[string]string{
		MachineLifecycleGroupLabel: "cluster.x-k8s.io",
		MachineLifecycleKindLabel:  "Machine",
		MachineLifecycleNameLabel:  "m0",
	}
	for _, tt := range []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{"nil labels", nil, false},
		{"missing group", map[string]string{MachineLifecycleKindLabel: "Machine", MachineLifecycleNameLabel: "m0"}, false},
		{"missing kind", map[string]string{MachineLifecycleGroupLabel: "cluster.x-k8s.io", MachineLifecycleNameLabel: "m0"}, false},
		{"missing name", map[string]string{MachineLifecycleGroupLabel: "cluster.x-k8s.io", MachineLifecycleKindLabel: "Machine"}, false},
		{"all present", full, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: tt.labels}}
			assert.Equal(t, tt.want, HasMachineLifecycleLabels(obj))
		})
	}
}

func TestClusterLifecycleLabelsToObjectReference(t *testing.T) {
	mapper := mapperForTests()

	tests := []struct {
		name             string
		labels           map[string]string
		contextNamespace string
		wantAPIVersion   string
		wantKind         string
		wantName         string
		wantNamespace    string
		wantErr          bool
	}{
		{
			name: "namespace-scoped kind uses context namespace",
			labels: map[string]string{
				ClusterLifecycleGroupLabel: "cluster.x-k8s.io",
				ClusterLifecycleKindLabel:  "Cluster",
				ClusterLifecycleNameLabel:  "capi-cluster-0",
			},
			contextNamespace: "capi-ns",
			wantAPIVersion:   "cluster.x-k8s.io/v1beta2",
			wantKind:         "Cluster",
			wantName:         "capi-cluster-0",
			wantNamespace:    "capi-ns",
		},
		{
			name: "cluster-scoped kind returns empty namespace regardless of context",
			labels: map[string]string{
				ClusterLifecycleGroupLabel: "management.cattle.io",
				ClusterLifecycleKindLabel:  "Cluster",
				ClusterLifecycleNameLabel:  "c-abc123",
			},
			contextNamespace: "c-abc123",
			wantAPIVersion:   "management.cattle.io/v3",
			wantKind:         "Cluster",
			wantName:         "c-abc123",
			wantNamespace:    "",
		},
		{
			name: "provisioning.cattle.io Cluster is namespaced",
			labels: map[string]string{
				ClusterLifecycleGroupLabel: "provisioning.cattle.io",
				ClusterLifecycleKindLabel:  "Cluster",
				ClusterLifecycleNameLabel:  "my-cluster",
			},
			contextNamespace: "fleet-default",
			wantAPIVersion:   "provisioning.cattle.io/v1",
			wantKind:         "Cluster",
			wantName:         "my-cluster",
			wantNamespace:    "fleet-default",
		},
		{
			name: "unknown group returns error",
			labels: map[string]string{
				ClusterLifecycleGroupLabel: "unregistered.example.com",
				ClusterLifecycleKindLabel:  "Cluster",
				ClusterLifecycleNameLabel:  "x",
			},
			contextNamespace: "fleet-default",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "plan", Namespace: tt.contextNamespace, Labels: tt.labels}}
			ref, err := ClusterLifecycleLabelsToObjectReference(obj, tt.contextNamespace, mapper)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAPIVersion, ref.APIVersion)
			assert.Equal(t, tt.wantKind, ref.Kind)
			assert.Equal(t, tt.wantName, ref.Name)
			assert.Equal(t, tt.wantNamespace, ref.Namespace)
		})
	}
}
