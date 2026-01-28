package user

import (
	"errors"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		currentpass string
		password    string
		expectsErr  bool
	}{
		{
			name:        "password too short",
			username:    "admin",
			currentpass: "currentpassword",
			password:    "tooshort",
			expectsErr:  true,
		},
		{
			name:        "username equals password min length",
			username:    "passwordpass",
			currentpass: "currentpassword",
			password:    "passwordpass",
			expectsErr:  true,
		},
		{
			name:        "username and password almost match",
			username:    "administrator",
			currentpass: "currentpassword",
			password:    "administrator1",
			expectsErr:  false,
		},
		{
			name:        "12 byte password, 6 runes",
			username:    "admin",
			currentpass: "currentpassword",
			password:    "пароль",
			expectsErr:  true,
		},
		{
			name:        "23 byte password, 12 runes",
			username:    "admin",
			currentpass: "currentpassword",
			password:    "абвгдеёжзий1",
			expectsErr:  false,
		},
		{
			name:        "username equals password min length unicode",
			username:    "абвгдеёжзий1",
			currentpass: "currentpassword",
			password:    "абвгдеёжзий1",
			expectsErr:  true,
		},
		{
			name:        "new password matches current password",
			username:    "admin",
			currentpass: "myfavoritepassword",
			password:    "myfavoritepassword",
			expectsErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.username, tt.currentpass, tt.password, 12)
			if err != nil && !tt.expectsErr {
				t.Errorf("Received unexpected error: %v", err)
			} else if err == nil && tt.expectsErr {
				t.Error("Expected error when non received")
			}
		})
	}

}

func TestUserFormatter(t *testing.T) {
	tests := map[string]struct {
		canRefresh           bool
		canUpdate            bool
		expectedActions      []string
		shouldHaveUpdateLink bool
	}{
		"user can refresh and update": {
			canRefresh:           true,
			canUpdate:            true,
			expectedActions:      []string{"setpassword", "refreshauthprovideraccess"},
			shouldHaveUpdateLink: true,
		},
		"user cannot refresh but can update": {
			canRefresh:           false,
			canUpdate:            true,
			expectedActions:      []string{"setpassword"},
			shouldHaveUpdateLink: true,
		},
		"user can refresh but cannot update": {
			canRefresh:           true,
			canUpdate:            false,
			expectedActions:      []string{"setpassword", "refreshauthprovideraccess"},
			shouldHaveUpdateLink: false,
		},
		"user cannot refresh or update": {
			canRefresh:           false,
			canUpdate:            false,
			expectedActions:      []string{"setpassword"},
			shouldHaveUpdateLink: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fakeAC := &fakeAccessControl{
				canDoFunc: func(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
					if verb == "create" && !tt.canRefresh {
						return errors.New("not allowed")
					}
					if verb == "update" && !tt.canUpdate {
						return errors.New("not allowed")
					}
					return nil
				},
			}

			schema := &types.Schema{
				Version: types.APIVersion{
					Group: v3.UserGroupVersionKind.Group,
				},
				ID:         v3.UserResource.Name,
				PluralName: "users",
			}

			apiContext := &types.APIContext{
				AccessControl: fakeAC,
				Schema:        schema,
				URLBuilder:    &stubURLBuilder{},
			}

			resource := &types.RawResource{
				ID:      "test-user",
				Schema:  schema,
				Actions: map[string]string{},
				Links: map[string]string{
					"update": "/v3/users/test-user",
				},
			}

			handler := &Handler{}
			handler.UserFormatter(apiContext, resource)

			for _, action := range tt.expectedActions {
				_, exists := resource.Actions[action]
				assert.True(t, exists, "Expected action %s to be present", action)
			}

			if !tt.canRefresh {
				_, exists := resource.Actions["refreshauthprovideraccess"]
				assert.False(t, exists, "Did not expect refreshauthprovideraccess action to be present")
			}

			_, hasUpdateLink := resource.Links["update"]
			assert.Equal(t, tt.shouldHaveUpdateLink, hasUpdateLink, "Update link presence mismatch")
		})
	}
}

// stubURLBuilder implements types.URLBuilder for testing
type stubURLBuilder struct{}

func (s *stubURLBuilder) Current() string {
	return "http://localhost:8080"
}

func (s *stubURLBuilder) RelativeToRoot(path string) string {
	return path
}

func (s *stubURLBuilder) Marker(marker string) string {
	return marker
}

func (s *stubURLBuilder) Link(linkName string, resource *types.RawResource) string {
	return "/v3/" + resource.Type + "/" + resource.ID + "/" + linkName
}

func (s *stubURLBuilder) Action(action string, resource *types.RawResource) string {
	return "/v3/users/" + resource.ID + "?action=" + action
}

func (s *stubURLBuilder) Collection(schema *types.Schema, version *types.APIVersion) string {
	return "/v3/" + schema.PluralName
}

func (s *stubURLBuilder) CollectionAction(schema *types.Schema, versionOverride *types.APIVersion, action string) string {
	return "/v3/" + schema.PluralName + "?action=" + action
}

func (s *stubURLBuilder) ResourceLink(resource *types.RawResource) string {
	return "/v3/" + resource.Type + "/" + resource.ID
}

func (s *stubURLBuilder) ResourceLinkByID(schema *types.Schema, id string) string {
	return "/v3/" + schema.PluralName + "/" + id
}

func (s *stubURLBuilder) FilterLink(schema *types.Schema, fieldName string, value string) string {
	return "/v3/" + schema.PluralName + "?" + fieldName + "=" + value
}

func (s *stubURLBuilder) Version(version types.APIVersion) string {
	return "/v3"
}

func (s *stubURLBuilder) SetSubContext(subContext string) {
	// no-op for stub
}

func (s *stubURLBuilder) SubContextCollection(subContext *types.Schema, contextName string, schema *types.Schema) string {
	return "/v3/" + schema.PluralName
}

func (s *stubURLBuilder) SchemaLink(schema *types.Schema) string {
	return "/v3/schemas/" + schema.ID
}

func (s *stubURLBuilder) ReverseSort(order types.SortOrder) string {
	return "reverse"
}

func (s *stubURLBuilder) Sort(field string) string {
	return "sort=" + field
}

func (s *stubURLBuilder) ActionLinkByID(schema *types.Schema, id string, action string) string {
	return "/v3/" + schema.PluralName + "/" + id + "?action=" + action
}

// fakeAccessControl implements types.AccessControl for testing
// All methods return nil/default values unless explicitly set
type fakeAccessControl struct {
	canDoFunc func(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error
}

func (f *fakeAccessControl) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	return nil
}

func (f *fakeAccessControl) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	return nil
}

func (f *fakeAccessControl) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	return nil
}

func (f *fakeAccessControl) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	return nil
}

func (f *fakeAccessControl) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	return nil
}

func (f *fakeAccessControl) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if f.canDoFunc != nil {
		return f.canDoFunc(apiGroup, resource, verb, apiContext, obj, schema)
	}
	return nil
}

func (f *fakeAccessControl) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	return obj
}

func (f *fakeAccessControl) FilterList(apiContext *types.APIContext, schema *types.Schema, objs []map[string]interface{}, context map[string]string) []map[string]interface{} {
	return objs
}
