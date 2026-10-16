//go:build !windows

package clean

import (
	"slices"
	"testing"

	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// pgSetDryRun sets the package variable dryRun for one test. Tests that call it must not run in parallel.
func pgSetDryRun(t *testing.T, value bool) {
	t.Helper()
	previous := dryRun
	dryRun = value
	t.Cleanup(func() { dryRun = previous })
}

var pgNormanLabels = map[string]string{"cattle.io/creator": "norman"}

// pgCleanupTarget describes one cleanup function that lists objects, deletes each of them, and returns []error.
type pgCleanupTarget struct {
	name      string
	resource  string
	namespace string // namespace of the objects, empty for cluster scoped objects
	selector  string
	run       func(kubernetes.Interface) []error
	build     func(names []string, next string) runtime.Object
}

func pgCleanupTargets() []pgCleanupTarget {
	const ns = "ns1"
	return []pgCleanupTarget{
		{
			name:     "cleanupClusterRoleBindings",
			resource: "clusterrolebindings",
			selector: "cattle.io/creator=norman",
			run:      cleanupClusterRoleBindings,
			build: func(names []string, next string) runtime.Object {
				return &rbacv1.ClusterRoleBindingList{ListMeta: pgListMeta(next), Items: pgMap(names, func(n string) rbacv1.ClusterRoleBinding {
					return rbacv1.ClusterRoleBinding{ObjectMeta: pgMeta(n, "", pgNormanLabels)}
				})}
			},
		},
		{
			name:      "cleanupRoleBindings",
			resource:  "rolebindings",
			namespace: ns,
			selector:  "cattle.io/creator=norman",
			run:       cleanupRoleBindings,
			build: func(names []string, next string) runtime.Object {
				return &rbacv1.RoleBindingList{ListMeta: pgListMeta(next), Items: pgMap(names, func(n string) rbacv1.RoleBinding {
					return rbacv1.RoleBinding{ObjectMeta: pgMeta(n, ns, pgNormanLabels)}
				})}
			},
		},
		{
			name:     "cleanupClusterRoles",
			resource: "clusterroles",
			selector: "cattle.io/creator=norman",
			run:      cleanupClusterRoles,
			build: func(names []string, next string) runtime.Object {
				return &rbacv1.ClusterRoleList{ListMeta: pgListMeta(next), Items: pgMap(names, func(n string) rbacv1.ClusterRole {
					return rbacv1.ClusterRole{ObjectMeta: pgMeta(n, "", pgNormanLabels)}
				})}
			},
		},
		{
			name:      "cleanupRoles",
			resource:  "roles",
			namespace: ns,
			selector:  "cattle.io/creator=norman",
			run:       cleanupRoles,
			build: func(names []string, next string) runtime.Object {
				return &rbacv1.RoleList{ListMeta: pgListMeta(next), Items: pgMap(names, func(n string) rbacv1.Role {
					return rbacv1.Role{ObjectMeta: pgMeta(n, ns, pgNormanLabels)}
				})}
			},
		},
		{
			name:      "cleanUpImagePullSecrets",
			resource:  "secrets",
			namespace: ns,
			selector:  cluster.CopiedPullSecretLabel + "=true",
			run:       cleanUpImagePullSecrets,
			build: func(names []string, next string) runtime.Object {
				labels := map[string]string{cluster.CopiedPullSecretLabel: "true"}
				return pgSecretList(pgMap(names, func(n string) corev1.Secret {
					return corev1.Secret{ObjectMeta: pgMeta(n, ns, labels)}
				}), next)
			},
		},
	}
}

func TestCleanupListAndDeletePagination(t *testing.T) {
	// The tests change the package variable dryRun, so they must not run in parallel.
	// Page layout with two items per page: [0 1] [2 3] [4].
	all := pgNames(5)

	tests := []pgScenario{
		{
			name:          "deletes the items of every page",
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantDeleted:   all,
		},
		{
			name:          "dry run pages through all items and deletes nothing",
			dryRun:        true,
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantDeleted:   []string{},
		},
		{
			name:          "resumes from the continue token of an expired error",
			listErrs:      map[int]error{1: paginationtest.ExpiredError("new-token")},
			extraTokens:   map[string]int{"new-token": 1},
			wantContinues: []string{"", "token-page-1", "new-token", "token-page-2"},
			wantDeleted:   all,
		},
		{
			name:          "returns the list error of the second page",
			listErrs:      map[int]error{1: errPgList},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"item-0", "item-1"},
			wantErrs:      []error{errPgList},
		},
		{
			name:          "keeps the delete errors of earlier pages when a later list call fails",
			listErrs:      map[int]error{1: errPgList},
			deleteErrs:    map[string]error{"item-1": errPgDelete},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"item-0", "item-1"},
			wantErrs:      []error{errPgDelete, errPgList},
		},
		{
			name:          "tolerates NotFound on delete",
			deleteErrs:    map[string]error{"item-1": pgNotFound(), "item-3": pgNotFound()},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantDeleted:   all,
		},
		{
			name:          "collects other delete errors and keeps paging",
			deleteErrs:    map[string]error{"item-1": errPgDelete, "item-3": errPgDelete},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantDeleted:   all,
			wantErrs:      []error{errPgDelete, errPgDelete},
		},
	}

	for _, target := range pgCleanupTargets() {
		for _, tc := range tests {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				pgSetDryRun(t, tc.dryRun)

				pager := newPgPager(all, 2, tc.listErrs, tc.extraTokens)
				client := fake.NewClientset()
				pgServeList(&client.Fake, target.resource, pager, target.build)
				pgFailOn(&client.Fake, "delete", target.resource, pgPrefixKeys(target.namespace, tc.deleteErrs))

				errs := target.run(client)

				pgAssertErrs(t, errs, tc.wantErrs)
				pgAssertCalls(t, pager.Calls, target.selector, tc.wantContinues)
				assert.Equal(t, pgKeys(target.namespace, tc.wantDeleted), pgActionKeys(&client.Fake, "delete", target.resource))
			})
		}
	}
}

func TestDeleteJobPagination(t *testing.T) {
	// The tests change the package variable dryRun, so they must not run in parallel.
	const namespace = "default"

	// Page layout with two items per page: [0 1] [2 3] [4 5] [6].
	names := []string{
		"cattle-cleanup-a", "other-1",
		"other-2", "cattle-cleanup-b",
		"cattle-cleanup-c", "keep-3",
		"cattle-cleanup-d",
	}
	cleanupJobs := []string{"cattle-cleanup-a", "cattle-cleanup-b", "cattle-cleanup-c", "cattle-cleanup-d"}

	tests := []pgScenario{
		{
			name:          "deletes only cleanup jobs on every page",
			wantContinues: []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantDeleted:   cleanupJobs,
		},
		{
			name:          "dry run pages through all jobs and deletes nothing",
			dryRun:        true,
			wantContinues: []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantDeleted:   []string{},
		},
		{
			name:          "resumes from the continue token of an expired error",
			listErrs:      map[int]error{1: paginationtest.ExpiredError("new-token")},
			extraTokens:   map[string]int{"new-token": 1},
			wantContinues: []string{"", "token-page-1", "new-token", "token-page-2", "token-page-3"},
			wantDeleted:   cleanupJobs,
		},
		{
			name:          "returns the list error of the second page",
			listErrs:      map[int]error{1: errPgList},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"cattle-cleanup-a"},
			wantErrs:      []error{errPgList},
		},
		{
			name:          "tolerates NotFound on delete",
			deleteErrs:    map[string]error{"cattle-cleanup-b": pgNotFound()},
			wantContinues: []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantDeleted:   cleanupJobs,
		},
		{
			name:          "returns other delete errors and stops paging",
			deleteErrs:    map[string]error{"cattle-cleanup-b": errPgDelete},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"cattle-cleanup-a", "cattle-cleanup-b"},
			wantErrs:      []error{errPgDelete},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pgSetDryRun(t, tc.dryRun)

			pager := newPgPager(names, 2, tc.listErrs, tc.extraTokens)
			client := fake.NewClientset()
			pgServeList(&client.Fake, "jobs", pager, func(names []string, next string) runtime.Object {
				return &batchv1.JobList{ListMeta: pgListMeta(next), Items: pgMap(names, func(n string) batchv1.Job {
					return batchv1.Job{ObjectMeta: pgMeta(n, namespace, pgNormanLabels)}
				})}
			})
			pgFailOn(&client.Fake, "delete", "jobs", pgPrefixKeys(namespace, tc.deleteErrs))

			err := deleteJob(client)

			if len(tc.wantErrs) == 0 {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErrs[0])
			}
			pgAssertCalls(t, pager.Calls, "cattle.io/creator=norman", tc.wantContinues)
			assert.Equal(t, pgKeys(namespace, tc.wantDeleted), pgActionKeys(&client.Fake, "delete", "jobs"))

			for _, action := range client.Actions() {
				if deleteAction, ok := action.(k8stesting.DeleteActionImpl); ok {
					require.NotNil(t, deleteAction.DeleteOptions.PropagationPolicy)
					assert.Equal(t, metav1.DeletePropagationBackground, *deleteAction.DeleteOptions.PropagationPolicy)
				}
			}
		})
	}
}

// pgDirtyNamespaces returns namespaces that cleanupNamespaces must change, except for "item-2", which is already clean.
// Page layout with two items per page: [0 1] [2 3] [4 5].
func pgDirtyNamespaces() []corev1.Namespace {
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "item-0", Finalizers: []string{"controller.cattle.io/namespace-auth", "kubernetes"}}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:        "item-1",
			Labels:      map[string]string{nslabels.ProjectIDFieldLabel: "p-1", "keep": "label"},
			Annotations: map[string]string{"cattle.io/status": "x", "field.cattle.io/creatorId": "u-1", "keep": "annotation"},
		}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-2", Labels: map[string]string{"keep": "label"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-3", Annotations: map[string]string{"field.cattle.io/resourceQuotaTemplateId": "q"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-4", Finalizers: []string{"controller.cattle.io/namespace-auth"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "item-5", Finalizers: []string{"controller.cattle.io/namespace-auth"}}},
	}
	return namespaces
}

func pgNamespaceIsClean(ns *corev1.Namespace) bool {
	if slices.Contains(ns.Finalizers, "controller.cattle.io/namespace-auth") {
		return false
	}
	for _, label := range nsLabels {
		if _, ok := ns.Labels[label]; ok {
			return false
		}
	}
	for _, annotation := range nsAnnotations {
		if _, ok := ns.Annotations[annotation]; ok {
			return false
		}
	}
	return true
}

// pgStoredNamespace reads a namespace from the object tracker, which bypasses the injected get errors.
func pgStoredNamespace(t *testing.T, client *fake.Clientset, name string) *corev1.Namespace {
	t.Helper()
	obj, err := client.Tracker().Get(corev1.SchemeGroupVersion.WithResource("namespaces"), "", name)
	require.NoError(t, err)
	ns, ok := obj.(*corev1.Namespace)
	require.True(t, ok, "unexpected type %T", obj)
	return ns
}

func pgNamespaceList(items []corev1.Namespace, next string) runtime.Object {
	return &corev1.NamespaceList{ListMeta: pgListMeta(next), Items: items}
}

func TestCleanupNamespacesPagination(t *testing.T) {
	// The tests change the package variable dryRun, so they must not run in parallel.
	dirty := []string{"item-0", "item-1", "item-3", "item-4", "item-5"}

	tests := []struct {
		name          string
		dryRun        bool
		listErrs      map[int]error
		extraTokens   map[string]int
		getErrs       map[string]error
		updateErrs    map[string]error
		wantContinues []string
		wantUpdated   []string // names of the namespaces that get an update call, in order
		wantCleaned   []string // names of the namespaces that are clean after the run
		wantErrs      []error
	}{
		{
			name:          "cleans the namespaces of every page",
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantUpdated:   dirty,
			wantCleaned:   dirty,
		},
		{
			name:          "dry run pages through all namespaces and updates nothing",
			dryRun:        true,
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantUpdated:   []string{},
			wantCleaned:   []string{},
		},
		{
			name:          "resumes from the continue token of an expired error",
			listErrs:      map[int]error{1: paginationtest.ExpiredError("new-token")},
			extraTokens:   map[string]int{"new-token": 1},
			wantContinues: []string{"", "token-page-1", "new-token", "token-page-2"},
			wantUpdated:   dirty,
			wantCleaned:   dirty,
		},
		{
			name:          "returns the list error of the second page",
			listErrs:      map[int]error{1: errPgList},
			wantContinues: []string{"", "token-page-1"},
			wantUpdated:   []string{"item-0", "item-1"},
			wantCleaned:   []string{"item-0", "item-1"},
			wantErrs:      []error{errPgList},
		},
		{
			name:          "tolerates NotFound on get",
			getErrs:       map[string]error{"item-3": pgNotFound()},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantUpdated:   []string{"item-0", "item-1", "item-4", "item-5"},
			wantCleaned:   []string{"item-0", "item-1", "item-4", "item-5"},
		},
		{
			name:          "tolerates NotFound on update",
			updateErrs:    map[string]error{"item-3": pgNotFound()},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantUpdated:   dirty,
			wantCleaned:   []string{"item-0", "item-1", "item-4", "item-5"},
		},
		{
			name:          "collects other get and update errors and keeps paging",
			getErrs:       map[string]error{"item-1": errPgList},
			updateErrs:    map[string]error{"item-4": errPgUpdate},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantUpdated:   []string{"item-0", "item-3", "item-4", "item-5"},
			wantCleaned:   []string{"item-0", "item-3", "item-5"},
			wantErrs:      []error{errPgList, errPgUpdate},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pgSetDryRun(t, tc.dryRun)

			namespaces := pgDirtyNamespaces()
			objects := make([]runtime.Object, 0, len(namespaces))
			for i := range namespaces {
				objects = append(objects, &namespaces[i])
			}
			client := fake.NewClientset(objects...)
			pager := newPgPager(pgDirtyNamespaces(), 2, tc.listErrs, tc.extraTokens)
			pgServeList(&client.Fake, "namespaces", pager, pgNamespaceList)
			pgFailOn(&client.Fake, "get", "namespaces", tc.getErrs)
			pgFailOn(&client.Fake, "update", "namespaces", tc.updateErrs)

			errs := cleanupNamespaces(client)

			pgAssertErrs(t, errs, tc.wantErrs)
			pgAssertCalls(t, pager.Calls, "", tc.wantContinues)
			assert.Equal(t, pgKeys("", tc.wantUpdated), pgActionKeys(&client.Fake, "update", "namespaces"))

			for _, name := range dirty {
				ns := pgStoredNamespace(t, client, name)
				assert.Equal(t, slices.Contains(tc.wantCleaned, name), pgNamespaceIsClean(ns), "namespace %s clean state", name)
			}

			// Values that cleanupNamespaces must not remove.
			if slices.Contains(tc.wantCleaned, "item-0") {
				ns := pgStoredNamespace(t, client, "item-0")
				assert.Equal(t, []string{"kubernetes"}, ns.Finalizers)
			}
			if slices.Contains(tc.wantCleaned, "item-1") {
				ns := pgStoredNamespace(t, client, "item-1")
				assert.Equal(t, map[string]string{"keep": "label"}, ns.Labels)
				assert.Equal(t, map[string]string{"keep": "annotation"}, ns.Annotations)
			}
		})
	}
}

func TestGetProjectMonitoringNamespacesPagination(t *testing.T) {
	t.Parallel()

	// Page layout with two items per page: [0 1] [2 3] [4].
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-prometheus-p-a", Labels: map[string]string{nslabels.ProjectIDFieldLabel: "p-a"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-prometheus-p-b", Labels: map[string]string{nslabels.ProjectIDFieldLabel: "p-b"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{nslabels.ProjectIDFieldLabel: "p-c"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "plain"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-prometheus-p-d", Labels: map[string]string{nslabels.ProjectIDFieldLabel: "p-d"}}},
	}

	tests := []struct {
		name          string
		listErrs      map[int]error
		extraTokens   map[string]int
		wantContinues []string
		want          []string
		wantErr       error
	}{
		{
			name:          "returns matching namespaces from every page",
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			want:          []string{"cattle-prometheus-p-a", "cattle-prometheus-p-b", "cattle-prometheus-p-d"},
		},
		{
			name:          "resumes from the continue token of an expired error",
			listErrs:      map[int]error{1: paginationtest.ExpiredError("new-token")},
			extraTokens:   map[string]int{"new-token": 1},
			wantContinues: []string{"", "token-page-1", "new-token", "token-page-2"},
			want:          []string{"cattle-prometheus-p-a", "cattle-prometheus-p-b", "cattle-prometheus-p-d"},
		},
		{
			name:          "returns the list error of the second page",
			listErrs:      map[int]error{1: errPgList},
			wantContinues: []string{"", "token-page-1"},
			wantErr:       errPgList,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pager := newPgPager(namespaces, 2, tc.listErrs, tc.extraTokens)
			client := fake.NewClientset()
			pgServeList(&client.Fake, "namespaces", pager, pgNamespaceList)

			got, err := getProjectMonitoringNamespaces(client)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
			pgAssertCalls(t, pager.Calls, "", tc.wantContinues)
		})
	}
}
