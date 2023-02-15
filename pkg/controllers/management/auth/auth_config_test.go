package auth

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	azuread "github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockUsers := newMockUserLister()
			config := &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        azuread.Name,
					Annotations: map[string]string{CleanupAnnotation: test.annotationValue},
				},
				Enabled: test.configEnabled,
			}
			var service cleanupService
			controller := authConfigController{
				cleanup:                 &service,
				authConfigsUnstructured: newMockAuthConfigClient(config),
				users:                   &mockUsers,
			}

			authConfig, err := controller.sync("test", config)
			acObject := authConfig.(*v3.AuthConfig)
			require.NoError(t, err)
			assert.Equal(t, test.newAnnotationValue, acObject.Annotations[CleanupAnnotation])
			assert.Equal(t, test.expectCleanup, service.cleanupCalled)
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

type mockUnstructuredAuthConfig struct {
	config *v3.AuthConfig
}

func (m mockUnstructuredAuthConfig) GetObjectKind() schema.ObjectKind {
	return nil
}

func (m mockUnstructuredAuthConfig) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) NewEmptyInstance() runtime.Unstructured {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) UnstructuredContent() map[string]interface{} {
	var out map[string]any
	b, err := json.Marshal(m.config)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(b, &out)
	if err != nil {
		return nil
	}
	return out
}

func (m mockUnstructuredAuthConfig) SetUnstructuredContent(content map[string]interface{}) {
	b, err := json.Marshal(content)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, m.config)
	if err != nil {
		return
	}
}

func (m mockUnstructuredAuthConfig) IsList() bool {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) EachListItem(f func(runtime.Object) error) error {
	//TODO implement me
	panic("implement me")
}

type mockAuthConfigClient struct {
	config mockUnstructuredAuthConfig
}

func newMockAuthConfigClient(authConfig *v3.AuthConfig) objectclient.GenericClient {
	return mockAuthConfigClient{config: mockUnstructuredAuthConfig{authConfig}}
}

func (m mockAuthConfigClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	o := unstructured.Unstructured{}
	o.SetUnstructuredContent(map[string]any{"Object": m.config})
	return &o, nil
}

func (m mockAuthConfigClient) Update(name string, o runtime.Object) (runtime.Object, error) {
	return o, nil
}

func (m mockAuthConfigClient) UnstructuredClient() objectclient.GenericClient {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Create(o runtime.Object) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) UpdateStatus(name string, o runtime.Object) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Delete(name string, opts *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectFactory() objectclient.ObjectFactory {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectClient() *objectclient.ObjectClient {
	//TODO implement me
	panic("implement me")
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
