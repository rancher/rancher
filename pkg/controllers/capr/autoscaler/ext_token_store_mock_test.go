package autoscaler

import (
	"sync"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/accessor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockExtTokenStore is a lightweight test double for the extTokenStore
// interface used by the autoscaler handler. It records calls and returns
// pre-programmed responses so tests can exercise the ext-token-aware code
// paths in ensureUserToken and cleanupRBAC without pulling in the full
// *exttokenstore.SystemStore.
type mockExtTokenStore struct {
	mu sync.Mutex

	// listForUserFn, when non-nil, is invoked in place of the default
	// behaviour of returning an empty list.
	listForUserFn func(userName string) (*ext.TokenList, error)
	// deleteFn, when non-nil, is invoked in place of the default no-op
	// delete.
	deleteFn func(name string, options *metav1.DeleteOptions) error
	// fetchFn, when non-nil, is invoked in place of the default no-op
	// fetch.
	fetchFn func(tokenID string) (accessor.TokenAccessor, error)

	listForUserCalls []string
	deleteCalls      []string
}

func (m *mockExtTokenStore) Fetch(tokenID string) (accessor.TokenAccessor, error) {
	if m.fetchFn != nil {
		return m.fetchFn(tokenID)
	}
	return nil, nil
}

func (m *mockExtTokenStore) ListForUser(userName string) (*ext.TokenList, error) {
	m.mu.Lock()
	m.listForUserCalls = append(m.listForUserCalls, userName)
	m.mu.Unlock()
	if m.listForUserFn != nil {
		return m.listForUserFn(userName)
	}
	return &ext.TokenList{}, nil
}

func (m *mockExtTokenStore) Delete(name string, options *metav1.DeleteOptions) error {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, name)
	m.mu.Unlock()
	if m.deleteFn != nil {
		return m.deleteFn(name, options)
	}
	return nil
}
