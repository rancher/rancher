package user

import (
	"errors"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type pagedUserClient struct {
	wranglerv3.UserClient
	pager        *paginationtest.Pager[apiv3.User]
	deletedUser  string
	getErr       error
	updatedUsers []string
}

// Get returns the user from the pager items. It returns NotFound for the deleted user.
func (c *pagedUserClient) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	if name == c.deletedUser {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "users"}, name)
	}
	for i := range c.pager.Items {
		if c.pager.Items[i].Name == name {
			return &c.pager.Items[i], nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "users"}, name)
}

func (c *pagedUserClient) List(opts metav1.ListOptions) (*apiv3.UserList, error) {
	items, next, err := c.pager.Serve(opts)
	if err != nil {
		return nil, err
	}
	return &apiv3.UserList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
}

func (c *pagedUserClient) Update(u *apiv3.User) (*apiv3.User, error) {
	if u.Name == c.deletedUser {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "users"}, u.Name)
	}
	c.updatedUsers = append(c.updatedUsers, u.Name)
	return u, nil
}

type fakeSecretClient struct {
	wcorev1.SecretClient
	createdSecrets []string
	createErr      error
}

func (c *fakeSecretClient) Create(sec *corev1.Secret) (*corev1.Secret, error) {
	if c.createErr != nil {
		return nil, c.createErr
	}
	c.createdSecrets = append(c.createdSecrets, sec.Name)
	return sec, nil
}

func (c *fakeSecretClient) Update(sec *corev1.Secret) (*corev1.Secret, error) {
	return sec, nil
}

func TestPasswordMigrator_MigrateAll_Pagination(t *testing.T) {
	t.Parallel()

	user1 := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user1"},
		Password:   "pass1",
	}
	user2 := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user2"},
		Password:   "pass2",
	}
	user3 := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user3"},
		Password:   "pass3",
	}
	userNoPass := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user4"},
		Password:   "",
	}

	errListFailed := errors.New("list failed")
	errGetFailed := errors.New("get failed")
	// The secret create returns NotFound when the namespace of the password secrets does not exist.
	errNamespaceNotFound := apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "cattle-local-user-passwords")

	tests := []struct {
		name                 string
		users                []apiv3.User
		pageSize             int
		deletedUser          string
		failPage             int
		failErr              error
		secretCreateErr      error
		getErr               error
		wantTokens           []string
		expectedUpdatedUsers []string
		expectedSecrets      []string
		wantErr              error
	}{
		{
			name:                 "two-page traversal migrates all users across pages",
			users:                []apiv3.User{user1, user2, user3},
			pageSize:             2,
			wantTokens:           []string{"", "token-page-1"},
			expectedUpdatedUsers: []string{"user1", "user2", "user3"},
			expectedSecrets:      []string{"user1", "user2", "user3"},
		},
		{
			name:                 "user deleted concurrently on page 2 is safely skipped without aborting",
			users:                []apiv3.User{user1, user2, user3},
			pageSize:             2,
			deletedUser:          "user3",
			wantTokens:           []string{"", "token-page-1"},
			expectedUpdatedUsers: []string{"user1", "user2"},
			expectedSecrets:      []string{"user1", "user2", "user3"},
		},
		{
			name:                 "skips users with empty password without creating secrets",
			users:                []apiv3.User{user1, userNoPass, user3},
			pageSize:             2,
			wantTokens:           []string{"", "token-page-1"},
			expectedUpdatedUsers: []string{"user1", "user3"},
			expectedSecrets:      []string{"user1", "user3"},
		},
		{
			name:                 "expired token on page 2 resumes",
			users:                []apiv3.User{user1, user2, user3},
			pageSize:             2,
			failPage:             2,
			failErr:              paginationtest.ExpiredError("resume-token"),
			wantTokens:           []string{"", "token-page-1", "resume-token"},
			expectedUpdatedUsers: []string{"user1", "user2", "user3"},
			expectedSecrets:      []string{"user1", "user2", "user3"},
		},
		{
			name:                 "page 2 list failure aborts with the list error",
			users:                []apiv3.User{user1, user2, user3},
			pageSize:             2,
			failPage:             2,
			failErr:              errListFailed,
			wantTokens:           []string{"", "token-page-1"},
			expectedUpdatedUsers: []string{"user1", "user2"},
			expectedSecrets:      []string{"user1", "user2"},
			wantErr:              errListFailed,
		},
		{
			name:            "NotFound from the secret create is returned when the user exists",
			users:           []apiv3.User{user1, user2, user3},
			pageSize:        2,
			secretCreateErr: errNamespaceNotFound,
			wantTokens:      []string{""},
			wantErr:         errNamespaceNotFound,
		},
		{
			name:            "failed user check returns the migrate error",
			users:           []apiv3.User{user1, user2, user3},
			pageSize:        2,
			secretCreateErr: errNamespaceNotFound,
			getErr:          errGetFailed,
			wantTokens:      []string{""},
			wantErr:         errNamespaceNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pager := paginationtest.NewPager(tc.users, tc.pageSize).FailOnPage(tc.failPage, tc.failErr)
			pager.ExtraTokens = map[string]int{"resume-token": 1}
			userClient := &pagedUserClient{pager: pager, deletedUser: tc.deletedUser, getErr: tc.getErr}
			secretClient := &fakeSecretClient{createErr: tc.secretCreateErr}

			migrator := &PasswordMigrator{
				Users:   userClient,
				Secrets: secretClient,
			}

			err := migrator.MigrateAll()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.expectedUpdatedUsers, userClient.updatedUsers)
			assert.Equal(t, tc.expectedSecrets, secretClient.createdSecrets)
		})
	}
}
