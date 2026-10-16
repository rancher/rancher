package cleanup

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const paginationTestProvider = "azuread"

// paginationTestEnv records the calls that deleteUsers makes.
type paginationTestEnv struct {
	pager   *paginationtest.Pager[v3.User]
	deleted []string
	updated []*v3.User
}

type paginationTestConfig struct {
	// pages holds the users of each page.
	pages [][]v3.User
	// listErrAtCall maps a zero-based list call index to the error that the call returns.
	listErrAtCall map[int]error
	// deleteErrs maps a user name to the error that Delete returns for it.
	deleteErrs map[string]error
	// secrets holds the names of users that have a password secret.
	secrets map[string]bool
}

func newPaginationTestService(t *testing.T, cfg paginationTestConfig) (*Service, *paginationTestEnv) {
	t.Helper()
	ctrl := gomock.NewController(t)
	env := &paginationTestEnv{pager: paginationtest.NewPagesPager(cfg.pages...)}
	env.pager.ListErrs = cfg.listErrAtCall

	userClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
	userClient.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*v3.UserList, error) {
		items, next, err := env.pager.Serve(opts)
		if err != nil {
			return nil, err
		}
		return &v3.UserList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
	}).AnyTimes()
	userClient.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ *metav1.DeleteOptions) error {
		env.deleted = append(env.deleted, name)
		return cfg.deleteErrs[name]
	}).AnyTimes()

	impClient := fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl)
	impClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(u *v3.User) (*v3.User, error) {
		env.updated = append(env.updated, u.DeepCopy())
		return u, nil
	}).AnyTimes()
	userClient.EXPECT().WithImpersonation(gomock.Any()).Return(impClient, nil).AnyTimes()

	secretsCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretsCache.EXPECT().Get(pbkdf2.LocalUserPasswordsNamespace, gomock.Any()).DoAndReturn(func(_, name string) (*corev1.Secret, error) {
		if cfg.secrets[name] {
			return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}).AnyTimes()

	return &Service{userClient: userClient, secretsCache: secretsCache}, env
}

func paginationTestExternalUser(name string) v3.User {
	return v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		PrincipalIDs: []string{paginationTestProvider + "_user://" + name},
	}
}

func paginationTestHybridUser(name, password string) v3.User {
	return v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Password:     password,
		PrincipalIDs: []string{paginationTestProvider + "_user://" + name, "local://" + name},
	}
}

func paginationTestOtherProviderUser(name string) v3.User {
	return v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		PrincipalIDs: []string{"okta_user://" + name},
	}
}

func paginationTestLocalUser(name string) v3.User {
	return v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		PrincipalIDs: []string{"local://" + name},
	}
}

func paginationTestUserPages() [][]v3.User {
	return [][]v3.User{
		{paginationTestExternalUser("ext-1"), paginationTestOtherProviderUser("okta-1"), paginationTestHybridUser("pwd-1", "hash")},
		{paginationTestExternalUser("ext-2"), paginationTestHybridUser("secret-1", ""), paginationTestLocalUser("local-1")},
		{paginationTestExternalUser("ext-3"), paginationTestHybridUser("pwd-2", "hash")},
	}
}

func TestDeleteUsersPagination(t *testing.T) {
	t.Parallel()

	listErr := errors.New("list failed")
	expiredErr := paginationtest.ExpiredError()
	deleteErr := errors.New("delete failed")
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "users"}, "ext-1")

	tests := []struct {
		name        string
		cfg         paginationTestConfig
		wantErr     error
		wantTokens  []string
		wantDeleted []string
		wantReset   []string
	}{
		{
			name: "processes all pages",
			cfg: paginationTestConfig{
				pages:   paginationTestUserPages(),
				secrets: map[string]bool{"secret-1": true},
			},
			wantTokens:  []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{"ext-1", "ext-2", "ext-3"},
			wantReset:   []string{"pwd-1", "secret-1", "pwd-2"},
		},
		{
			name: "delete NotFound is tolerated",
			cfg: paginationTestConfig{
				pages:      paginationTestUserPages(),
				secrets:    map[string]bool{"secret-1": true},
				deleteErrs: map[string]error{"ext-1": notFound},
			},
			wantTokens:  []string{"", "token-page-1", "token-page-2"},
			wantDeleted: []string{"ext-1", "ext-2", "ext-3"},
			wantReset:   []string{"pwd-1", "secret-1", "pwd-2"},
		},
		{
			name: "delete error aborts and later pages are not listed",
			cfg: paginationTestConfig{
				pages:      paginationTestUserPages(),
				secrets:    map[string]bool{"secret-1": true},
				deleteErrs: map[string]error{"ext-2": deleteErr},
			},
			wantErr:     deleteErr,
			wantTokens:  []string{"", "token-page-1"},
			wantDeleted: []string{"ext-1", "ext-2"},
			wantReset:   []string{"pwd-1"},
		},
		{
			name: "list error on page 2 is returned",
			cfg: paginationTestConfig{
				pages:         paginationTestUserPages(),
				secrets:       map[string]bool{"secret-1": true},
				listErrAtCall: map[int]error{1: listErr},
			},
			wantErr:     listErr,
			wantTokens:  []string{"", "token-page-1"},
			wantDeleted: []string{"ext-1"},
			wantReset:   []string{"pwd-1"},
		},
		{
			name: "expired token is returned and the list does not resume",
			cfg: paginationTestConfig{
				pages:         paginationTestUserPages(),
				secrets:       map[string]bool{"secret-1": true},
				listErrAtCall: map[int]error{1: expiredErr},
			},
			wantErr:     expiredErr,
			wantTokens:  []string{"", "token-page-1"},
			wantDeleted: []string{"ext-1"},
			wantReset:   []string{"pwd-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, env := newPaginationTestService(t, tt.cfg)

			err := svc.deleteUsers(paginationTestProvider)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantTokens, env.pager.Tokens())
			env.pager.AssertLimit(t)
			assert.Equal(t, tt.wantDeleted, env.deleted)

			var reset []string
			for _, u := range env.updated {
				reset = append(reset, u.Name)
				assert.Equal(t, []string{"local://" + u.Name}, u.PrincipalIDs)
			}
			assert.Equal(t, tt.wantReset, reset)
		})
	}
}
