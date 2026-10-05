package usercontrollers

import (
	"testing"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type roleTemplateBindingsMocks struct {
	crtbs        *fake.MockControllerInterface[*v32.ClusterRoleTemplateBinding, *v32.ClusterRoleTemplateBindingList]
	crtbCache    *fake.MockCacheInterface[*v32.ClusterRoleTemplateBinding]
	prtbs        *fake.MockControllerInterface[*v32.ProjectRoleTemplateBinding, *v32.ProjectRoleTemplateBindingList]
	prtbCache    *fake.MockCacheInterface[*v32.ProjectRoleTemplateBinding]
	projectCache *fake.MockCacheInterface[*v32.Project]
}

// newRoleTemplateBindingsTest returns role template binding clients that fail the test on any call that
// isn't expected.
func newRoleTemplateBindingsTest(t *testing.T) (*roleTemplateBindings, *roleTemplateBindingsMocks) {
	ctrl := gomock.NewController(t)
	m := &roleTemplateBindingsMocks{
		crtbs:        fake.NewMockControllerInterface[*v32.ClusterRoleTemplateBinding, *v32.ClusterRoleTemplateBindingList](ctrl),
		crtbCache:    fake.NewMockCacheInterface[*v32.ClusterRoleTemplateBinding](ctrl),
		prtbs:        fake.NewMockControllerInterface[*v32.ProjectRoleTemplateBinding, *v32.ProjectRoleTemplateBindingList](ctrl),
		prtbCache:    fake.NewMockCacheInterface[*v32.ProjectRoleTemplateBinding](ctrl),
		projectCache: fake.NewMockCacheInterface[*v32.Project](ctrl),
	}
	return &roleTemplateBindings{crtbs: m.crtbs, crtbCache: m.crtbCache, prtbs: m.prtbs, prtbCache: m.prtbCache, projectCache: m.projectCache}, m
}

// newConnectedImportedCluster returns an imported cluster whose removal started removedFor ago.
func newConnectedImportedCluster(removedFor time.Duration) *v32.Cluster {
	started := metav1.NewTime(time.Now().Add(-removedFor))
	return &v32.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1", DeletionTimestamp: &started},
		Status:     v32.ClusterStatus{Driver: v32.ClusterDriverImported, APIEndpoint: "https://10.0.0.1", CACert: "ca"},
	}
}

func deletingMeta(namespace, name string) metav1.ObjectMeta {
	now := metav1.Now()
	return metav1.ObjectMeta{Namespace: namespace, Name: name, DeletionTimestamp: &now}
}

func TestRemoveRoleTemplateBindingsIsNotRequiredForClustersTheirResourcesGoAwayWith(t *testing.T) {
	for name, cluster := range map[string]*v32.Cluster{
		"created by a provisioning cluster": func() *v32.Cluster {
			c := newConnectedImportedCluster(0)
			c.Annotations = map[string]string{"provisioning.cattle.io/administrated": "true"}
			return c
		}(),
		"never connected": func() *v32.Cluster {
			c := newConnectedImportedCluster(0)
			c.Status.APIEndpoint = ""
			return c
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			bindings, _ := newRoleTemplateBindingsTest(t)

			removal, err := bindings.removeRoleTemplateBindings(cluster)

			require.NoError(t, err)
			assert.True(t, removal.concluded)
			assert.Equal(t, coreV1.ConditionTrue, removal.status)
			assert.Equal(t, reasonNotRequired, removal.reason)
		})
	}
}

func TestRemoveRoleTemplateBindingsDeletesThemAndWaits(t *testing.T) {
	bindings, m := newRoleTemplateBindingsTest(t)
	m.crtbCache.EXPECT().List("c-m-test", gomock.Any()).Return([]*v32.ClusterRoleTemplateBinding{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "crtb-a"}},
		{ObjectMeta: deletingMeta("c-m-test", "crtb-b")},
	}, nil)
	m.crtbs.EXPECT().Delete("c-m-test", "crtb-a", gomock.Any()).Return(nil)
	m.projectCache.EXPECT().List("c-m-test", gomock.Any()).Return([]*v32.Project{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "p-1"}, Status: v32.ProjectStatus{BackingNamespace: "c-m-test-p-1"}},
	}, nil)
	m.prtbCache.EXPECT().List("c-m-test-p-1", gomock.Any()).Return([]*v32.ProjectRoleTemplateBinding{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test-p-1", Name: "prtb-x"}},
	}, nil)
	m.prtbs.EXPECT().Delete("c-m-test-p-1", "prtb-x", gomock.Any()).Return(nil)
	// No call deletes the project itself: that would delete the namespaces Rancher created for it downstream.

	removal, err := bindings.removeRoleTemplateBindings(newConnectedImportedCluster(0))

	require.NoError(t, err)
	assert.False(t, removal.concluded, "three bindings are still there")
	assert.Equal(t, coreV1.ConditionUnknown, removal.status)
	assert.Equal(t, reasonRemoving, removal.reason)
}

func TestRemoveRoleTemplateBindingsConcludesOnceTheyAreGone(t *testing.T) {
	bindings, m := newRoleTemplateBindingsTest(t)
	m.crtbCache.EXPECT().List("c-m-test", gomock.Any()).Return(nil, nil)
	m.projectCache.EXPECT().List("c-m-test", gomock.Any()).Return(nil, nil)

	removal, err := bindings.removeRoleTemplateBindings(newConnectedImportedCluster(0))

	require.NoError(t, err)
	assert.True(t, removal.concluded)
	assert.Equal(t, coreV1.ConditionTrue, removal.status)
	assert.Equal(t, reasonRemoved, removal.reason)
}

func TestRemoveRoleTemplateBindingsMovesOnAfterTheTimeout(t *testing.T) {
	bindings, m := newRoleTemplateBindingsTest(t)
	m.crtbCache.EXPECT().List("c-m-test", gomock.Any()).Return([]*v32.ClusterRoleTemplateBinding{
		{ObjectMeta: deletingMeta("c-m-test", "crtb-b")},
	}, nil)
	m.projectCache.EXPECT().List("c-m-test", gomock.Any()).Return(nil, nil)

	removal, err := bindings.removeRoleTemplateBindings(newConnectedImportedCluster(roleTemplateBindingsRemovalTimeout + time.Minute))

	require.NoError(t, err)
	assert.True(t, removal.concluded, "like the agent uninstall, the removal moves on")
	assert.Equal(t, coreV1.ConditionFalse, removal.status)
	assert.Equal(t, reasonTimedOut, removal.reason)
	assert.Contains(t, removal.message, "1 role template bindings")
}

func TestRemoveWaitsForTheRoleTemplateBindingsBeforeUninstallingTheAgent(t *testing.T) {
	cluster := newConnectedImportedCluster(0)
	lifecycle, env := newRemoveTestEnv(cluster.DeepCopy())
	bindings, m := newRoleTemplateBindingsTest(t)
	m.crtbCache.EXPECT().List("c-m-test", gomock.Any()).Return([]*v32.ClusterRoleTemplateBinding{
		{ObjectMeta: deletingMeta("c-m-test", "crtb-b")},
	}, nil)
	m.projectCache.EXPECT().List("c-m-test", gomock.Any()).Return(nil, nil)
	lifecycle.roleTemplateBindings = bindings

	_, err := lifecycle.Remove(cluster)

	assert.ErrorIs(t, err, generic.ErrSkip)
	assert.True(t, v32.ClusterConditionRoleTemplateBindingsRemoved.IsUnknown(env.stored))
	assert.Empty(t, v32.ClusterConditionAgentUninstallScheduled.GetStatus(env.stored), "the agent must still be there while the bindings are removed")
	assert.Len(t, env.controller.EnqueueAfterCalls(), 1)
}
