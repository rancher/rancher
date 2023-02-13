package auth

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	azuread "github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func TestCleanupRuns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		configEnabled      bool
		annotationValue    string
		expectCleanup      bool
		newAnnotationValue string
	}{
		{
			name:               "cleanup runs in disabled unlocked auth config",
			configEnabled:      false,
			annotationValue:    CleanupUnlocked,
			expectCleanup:      true,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in disabled auth config without annotation",
			configEnabled:      false,
			annotationValue:    "",
			expectCleanup:      false,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in enabled auth config without annotation",
			configEnabled:      true,
			annotationValue:    "",
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in disabled rancher_locked auth config",
			configEnabled:      false,
			annotationValue:    CleanupRancherLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in disabled user_locked auth config",
			configEnabled:      false,
			annotationValue:    CleanupUserLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUserLocked,
		},
		{
			name:               "no cleanup in enabled unlocked auth config",
			configEnabled:      true,
			annotationValue:    CleanupUnlocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in enabled rancher_locked auth config",
			configEnabled:      true,
			annotationValue:    CleanupRancherLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in enabled user_locked auth config",
			configEnabled:      true,
			annotationValue:    CleanupUserLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUserLocked,
		},
		{
			name:               "no cleanup in disabled auth config with invalid annotation",
			configEnabled:      false,
			annotationValue:    "bad",
			expectCleanup:      false,
			newAnnotationValue: "bad",
		},
	}

	mockAuthConfig := newMockAuthConfigClient()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockUsers := newMockUserLister()
			var service cleanupService
			controller := authConfigController{
				cleanup:          &service,
				authConfigClient: mockAuthConfig,
				users:            &mockUsers,
			}
			config := &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        azuread.Name,
					Annotations: map[string]string{CleanupAnnotation: test.annotationValue},
				},
				Enabled: test.configEnabled,
			}

			obj, err := controller.sync("test", config)
			require.NoError(t, err)
			assert.Equal(t, test.expectCleanup, service.cleanupCalled)
			assert.Equal(t, test.newAnnotationValue, obj.(*v3.AuthConfig).Annotations[CleanupAnnotation])
		})
	}
}

func TestAuthConfigSync(t *testing.T) {
	tests := []struct {
		name                    string
		usernamesForTestConfig  []string
		usernamesForOtherConfig []string
		listUsersErr            error
		errExpected             bool
	}{
		{
			name:                    "basic test case - refresh single user",
			usernamesForTestConfig:  []string{"tUser"},
			usernamesForOtherConfig: []string{},
			listUsersErr:            nil,
			errExpected:             false,
		},
		{
			name:                    "refresh user belonging to one auth provider but not another",
			usernamesForTestConfig:  []string{"tUser"},
			usernamesForOtherConfig: []string{"oUser"},
			listUsersErr:            nil,
			errExpected:             false,
		},
		{
			name:                    "refresh multiple users, some in the auth config, others not",
			usernamesForTestConfig:  []string{"tUser", "sUser", "newUser"},
			usernamesForOtherConfig: []string{"oUser", "configUser", "otherConfigUser"},
			listUsersErr:            nil,
			errExpected:             false,
		},
		{
			name:                    "error when listing users - expect an error",
			usernamesForTestConfig:  []string{"tUser", "sUser", "newUser"},
			usernamesForOtherConfig: []string{"oUser", "configUser", "otherConfigUser"},
			listUsersErr:            fmt.Errorf("error when listing users"),
			errExpected:             true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			const testConfigName = "testConfig"
			const otherConfigName = "otherConfig"

			mockUsers := newMockUserLister()
			for _, username := range test.usernamesForTestConfig {
				mockUsers.AddUser(username, testConfigName)
			}

			for _, username := range test.usernamesForOtherConfig {
				mockUsers.AddUser(username, otherConfigName)
			}

			if test.listUsersErr != nil {
				mockUsers.AddListUserError(test.listUsersErr)
			}

			mockRefresher := newMockAuthProvider()
			controller := authConfigController{users: &mockUsers, authRefresher: &mockRefresher, cleanup: &fakeCleanupService{}}
			config := v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: testConfigName,
					Annotations: map[string]string{
						CleanupAnnotation: CleanupUnlocked,
					},
				},
				Enabled: true,
			}
			_, err := controller.sync("test", &config)
			if test.errExpected {
				assert.Error(t, err, "Expected error but none was provided")
			} else {
				assert.NoError(t, err, "Expected no error")
				for _, username := range test.usernamesForTestConfig {
					assert.Contains(t, mockRefresher.refreshedUsers, username, "Expected user to be refreshed")
				}
				for _, username := range test.usernamesForOtherConfig {
					assert.NotContains(t, mockRefresher.refreshedUsers, username, "Did not expect user to be refreshed")
				}
			}
		})
	}
}

type mockUserLister struct {
	users        []*v3.User
	listUsersErr error
}

func newMockUserLister() mockUserLister {
	return mockUserLister{
		users: []*v3.User{},
	}
}

func (m *mockUserLister) List(namespace string, selector labels.Selector) (ret []*v3.User, err error) {
	if m.listUsersErr != nil {
		return nil, m.listUsersErr
	}
	return m.users, nil
}
func (m *mockUserLister) Get(namespace, name string) (*v3.User, error) {
	for _, user := range m.users {
		if user.Name == name {
			return user, nil
		}
	}
	return nil, apierror.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "user"}, name)
}

func (m *mockUserLister) AddUser(username string, provider string) {
	principalIds := []string{
		fmt.Sprintf("local://%s", username),
		fmt.Sprintf("%s_user://%s", provider, username),
	}
	newUser := v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: username},
		PrincipalIDs: principalIds,
	}
	found := false
	for idx, user := range m.users {
		if user.Name == newUser.Name {
			m.users[idx] = &newUser
			found = true
		}
	}
	if !found {
		m.users = append(m.users, &newUser)
	}
}

func (m *mockUserLister) AddListUserError(err error) {
	m.listUsersErr = err
}

type cleanupService struct {
	cleanupCalled bool
}

func (s *cleanupService) Run(_ *v3.AuthConfig) error {
	s.cleanupCalled = true
	return nil
}

type mockAuthConfigClient struct {
}

func (m mockAuthConfigClient) Create(_ *v3.AuthConfig) (*v3.AuthConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Update(config *v3.AuthConfig) (*v3.AuthConfig, error) {
	return config, nil
}

func (m mockAuthConfigClient) Delete(_ string, _ *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Get(_ string, _ metav1.GetOptions) (*v3.AuthConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) List(_ metav1.ListOptions) (*v3.AuthConfigList, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *v3.AuthConfig, err error) {
	//TODO implement me
	panic("implement me")
}

func newMockAuthConfigClient() controllers.AuthConfigClient {
	return mockAuthConfigClient{}
}

type fakeCleanupService struct{}

func (f *fakeCleanupService) Run(config *v3.AuthConfig) error {
	return nil
}

type mockAuthProvider struct {
	allUsersRefreshed bool
	refreshedUsers    map[string]bool
}

func newMockAuthProvider() mockAuthProvider {
	return mockAuthProvider{
		allUsersRefreshed: false,
		refreshedUsers:    map[string]bool{},
	}
}

func (m *mockAuthProvider) TriggerAllUserRefresh() {
	m.allUsersRefreshed = true
}

func (m *mockAuthProvider) TriggerUserRefresh(username string, force bool) {
	m.refreshedUsers[username] = force
}
