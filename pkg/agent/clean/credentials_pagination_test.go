package clean

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/pagination"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// The helpers with the "pg" prefix simulate an API server that serves pages on top of the client-go fake clientset.
// The fake clientset ignores Limit and Continue, so paginationtest.Pager serves the pages itself and records every call.

var (
	errPgList    = errors.New("list failed")
	errPgDelete  = errors.New("delete failed")
	errPgUpdate  = errors.New("update failed")
	errPgExpired = paginationtest.ExpiredError()
)

// newPgPager returns a pager for a copy of items.
func newPgPager[T any](items []T, perPage int, listErrs map[int]error) *paginationtest.Pager[T] {
	p := paginationtest.NewPager(items, perPage)
	p.ListErrs = listErrs
	return p
}

// pgServeList makes the fake clientset answer list calls for resource from the pager.
// build converts one page and the next continue token into a typed list.
func pgServeList[T any](f *k8stesting.Fake, resource string, p *paginationtest.Pager[T], build func(items []T, next string) runtime.Object) {
	f.PrependReactor("list", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		listAction, ok := action.(k8stesting.ListActionImpl)
		if !ok {
			return true, nil, fmt.Errorf("unexpected action type %T", action)
		}
		items, next, err := p.Serve(listAction.ListOptions)
		if err != nil {
			return true, nil, err
		}
		return true, build(items, next), nil
	})
}

// pgFailOn makes the fake clientset return the error for the given verb and object.
// The map key is "namespace/name", or "name" for cluster scoped objects.
func pgFailOn(f *k8stesting.Fake, verb, resource string, errs map[string]error) {
	f.PrependReactor(verb, resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		if err, ok := errs[pgKey(action.GetNamespace(), pgActionName(action))]; ok {
			return true, nil, err
		}
		return false, nil, nil
	})
}

func pgActionName(action k8stesting.Action) string {
	switch a := action.(type) {
	case k8stesting.DeleteActionImpl:
		return a.Name
	case k8stesting.GetActionImpl:
		return a.Name
	case k8stesting.UpdateActionImpl:
		if accessor, err := meta.Accessor(a.Object); err == nil {
			return accessor.GetName()
		}
	}
	return ""
}

func pgKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "/" + name
}

// pgKeys returns the "namespace/name" keys for names. The result is never nil.
func pgKeys(namespace string, names []string) []string {
	keys := make([]string, 0, len(names))
	for _, name := range names {
		keys = append(keys, pgKey(namespace, name))
	}
	return keys
}

func pgNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("item-%d", i)
	}
	return names
}

// pgActionKeys returns the keys of all recorded actions with the given verb and resource. The result is never nil.
func pgActionKeys(f *k8stesting.Fake, verb, resource string) []string {
	keys := []string{}
	for _, action := range f.Actions() {
		if action.GetVerb() != verb || action.GetResource().Resource != resource {
			continue
		}
		keys = append(keys, pgKey(action.GetNamespace(), pgActionName(action)))
	}
	return keys
}

// pgAssertCalls checks that every list call has the page size limit and the selector,
// and that call N+1 sends the continue token from the response to call N.
func pgAssertCalls(t *testing.T, calls []metav1.ListOptions, wantSelector string, wantContinues []string) {
	t.Helper()
	gotContinues := make([]string, 0, len(calls))
	for i, call := range calls {
		gotContinues = append(gotContinues, call.Continue)
		assert.Equal(t, pagination.PageSize, call.Limit, "limit of list call %d", i)
		assert.Equal(t, wantSelector, call.LabelSelector, "label selector of list call %d", i)
	}
	assert.Equal(t, wantContinues, gotContinues, "continue tokens of the list calls")
}

func pgAssertErrs(t *testing.T, got, want []error) {
	t.Helper()
	if !assert.Len(t, got, len(want)) {
		return
	}
	for i := range want {
		assert.ErrorIs(t, got[i], want[i])
	}
}

func pgNotFound() error {
	return apierrors.NewNotFound(schema.GroupResource{Resource: "test"}, "gone")
}

func pgListMeta(next string) metav1.ListMeta {
	return metav1.ListMeta{Continue: next}
}

func pgMeta(name, namespace string, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}
}

func pgMap[T any](names []string, f func(name string) T) []T {
	items := make([]T, 0, len(names))
	for _, name := range names {
		items = append(items, f(name))
	}
	return items
}

// pgScenario describes one table entry. Not every test uses every field.
type pgScenario struct {
	name          string
	dryRun        bool
	listErrs      map[int]error
	deleteErrs    map[string]error // item name -> error that the delete call returns
	wantContinues []string
	wantDeleted   []string // names of the items that get a delete call, in order
	wantErrs      []error
}

func pgSecretList(items []corev1.Secret, next string) runtime.Object {
	return &corev1.SecretList{ListMeta: pgListMeta(next), Items: items}
}

func TestUnusedPullSecretsPagination(t *testing.T) {
	t.Parallel()

	const namespace = "cattle-system"
	selector := cluster.AgentPullSecretLabel + "=true"
	labels := map[string]string{cluster.AgentPullSecretLabel: "true"}
	names := pgNames(6)

	tests := []struct {
		name        string
		configured  []string
		listErrs    map[int]error
		deleteErrs  map[string]error
		wantCalls   []string
		wantDeleted []string
	}{
		{
			name:        "removes unused secrets from every page and keeps configured ones",
			configured:  []string{"item-1", "item-4"},
			wantCalls:   []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{"item-0", "item-2", "item-3", "item-5"},
		},
		{
			name:        "removes all secrets when none is configured",
			configured:  nil,
			wantCalls:   []string{"", "token-page-1", "token-page-2"},
			wantDeleted: names,
		},
		{
			name:        "keeps all secrets when all are configured",
			configured:  names,
			wantCalls:   []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{},
		},
		{
			name:        "deletes nothing when listing the second page fails",
			configured:  []string{"item-1", "item-4"},
			listErrs:    map[int]error{1: errPgList},
			wantCalls:   []string{"", "token-page-1"},
			wantDeleted: []string{},
		},
		{
			name:        "returns the expired error of the second page and does not resume",
			configured:  []string{"item-1", "item-4"},
			listErrs:    map[int]error{1: errPgExpired},
			wantCalls:   []string{"", "token-page-1"},
			wantDeleted: []string{},
		},
		{
			name:        "tolerates NotFound on delete and deletes the remaining secrets",
			configured:  []string{"item-1", "item-4"},
			deleteErrs:  map[string]error{"item-2": pgNotFound()},
			wantCalls:   []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{"item-0", "item-2", "item-3", "item-5"},
		},
		{
			name:        "logs other delete errors and deletes the remaining secrets",
			configured:  []string{"item-1", "item-4"},
			deleteErrs:  map[string]error{"item-2": errPgDelete},
			wantCalls:   []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{"item-0", "item-2", "item-3", "item-5"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pager := newPgPager(pgMap(names, func(n string) corev1.Secret {
				return corev1.Secret{ObjectMeta: pgMeta(n, namespace, labels)}
			}), 2, tc.listErrs)
			client := fake.NewClientset()
			pgServeList(&client.Fake, "secrets", pager, pgSecretList)
			pgFailOn(&client.Fake, "delete", "secrets", pgPrefixKeys(namespace, tc.deleteErrs))

			unusedPullSecrets(t.Context(), client.CoreV1().Secrets(namespace), tc.configured)

			pgAssertCalls(t, pager.Calls, selector, tc.wantCalls)
			assert.Equal(t, pgKeys(namespace, tc.wantDeleted), pgActionKeys(&client.Fake, "delete", "secrets"))
		})
	}
}

// pgPrefixKeys converts a map keyed by name to a map keyed by "namespace/name".
func pgPrefixKeys(namespace string, in map[string]error) map[string]error {
	out := make(map[string]error, len(in))
	for name, err := range in {
		out[pgKey(namespace, name)] = err
	}
	return out
}

func TestRemoveUnusedCattleCredentialsFromPagination(t *testing.T) {
	// t.Setenv forbids t.Parallel for this test and its subtests.
	const (
		namespace = "cattle-system"
		current   = "cattle-credentials-current"
	)
	t.Setenv("CATTLE_CREDENTIAL_NAME", current)

	now := time.Now()
	old := metav1.NewTime(now.Add(-2 * time.Hour))
	recent := metav1.NewTime(now.Add(-10 * time.Minute))

	// Page layout with two items per page: [0 1] [2 3] [4 5] [6].
	secrets := []corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-credentials-old0", Namespace: namespace, CreationTimestamp: old}},
		{ObjectMeta: metav1.ObjectMeta{Name: "unrelated-secret", Namespace: namespace, CreationTimestamp: old}},
		{ObjectMeta: metav1.ObjectMeta{Name: current, Namespace: namespace, CreationTimestamp: old}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-credentials-recent", Namespace: namespace, CreationTimestamp: recent}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-credentials-old4", Namespace: namespace, CreationTimestamp: old}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other-secret", Namespace: namespace, CreationTimestamp: old}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cattle-credentials-old6", Namespace: namespace, CreationTimestamp: old}},
	}

	tests := []pgScenario{
		{
			name:          "deletes only old unused credentials on every page",
			wantContinues: []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantDeleted:   []string{"cattle-credentials-old0", "cattle-credentials-old4", "cattle-credentials-old6"},
		},
		{
			name:          "returns the list error of the second page",
			listErrs:      map[int]error{1: errPgList},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"cattle-credentials-old0"},
			wantErrs:      []error{errPgList},
		},
		{
			name:          "returns the expired error of the second page and does not resume",
			listErrs:      map[int]error{1: errPgExpired},
			wantContinues: []string{"", "token-page-1"},
			wantDeleted:   []string{"cattle-credentials-old0"},
			wantErrs:      []error{errPgExpired},
		},
		{
			name:          "tolerates NotFound on delete and continues",
			deleteErrs:    map[string]error{"cattle-credentials-old4": pgNotFound()},
			wantContinues: []string{"", "token-page-1", "token-page-2", "token-page-3"},
			wantDeleted:   []string{"cattle-credentials-old0", "cattle-credentials-old4", "cattle-credentials-old6"},
		},
		{
			name:          "returns other delete errors and stops paging",
			deleteErrs:    map[string]error{"cattle-credentials-old4": errPgDelete},
			wantContinues: []string{"", "token-page-1", "token-page-2"},
			wantDeleted:   []string{"cattle-credentials-old0", "cattle-credentials-old4"},
			wantErrs:      []error{errPgDelete},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pager := newPgPager(secrets, 2, tc.listErrs)
			client := fake.NewClientset()
			pgServeList(&client.Fake, "secrets", pager, pgSecretList)
			pgFailOn(&client.Fake, "delete", "secrets", pgPrefixKeys(namespace, tc.deleteErrs))

			err := removeUnusedCattleCredentialsFrom(t.Context(), client.CoreV1().Secrets(namespace))

			if len(tc.wantErrs) == 0 {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErrs[0])
			}
			pgAssertCalls(t, pager.Calls, "", tc.wantContinues)
			assert.Equal(t, pgKeys(namespace, tc.wantDeleted), pgActionKeys(&client.Fake, "delete", "secrets"))
		})
	}
}
