package auth

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSync(t *testing.T) {
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
			name:                    "refresh user belonging to on auth provider but not another",
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
			controller := authConfigController{users: &mockUsers, authRefresher: &mockRefresher}
			config := v3.AuthConfig{
				ObjectMeta: v1.ObjectMeta{Name: testConfigName},
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
		ObjectMeta:   v1.ObjectMeta{Name: username},
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
