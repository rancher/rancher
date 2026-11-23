package user

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/impersonation"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type actionTokenManager struct {
	original *apiv3.Token
	created  int
}

type actionFailingBindings struct{}

func (actionFailingBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return nil, errors.New("bindings unavailable")
}

func (m *actionTokenManager) NewLoginToken(userID string, principal apiv3.Principal, _ []apiv3.Principal, _ string, _ int64, _ string) (*apiv3.Token, string, error) {
	m.created++
	return &apiv3.Token{ObjectMeta: metav1.ObjectMeta{Name: "effective"}, UserID: userID, UserPrincipal: principal}, "secret", nil
}

func (m *actionTokenManager) UpdateToken(token *apiv3.Token) (*apiv3.Token, error) {
	return token, nil
}

func (m *actionTokenManager) GetToken(value string) (accessor.TokenAccessor, int, error) {
	if value == "original" {
		return m.original, http.StatusOK, nil
	}
	return nil, http.StatusUnauthorized, errors.New("token not found")
}

func (m *actionTokenManager) GetTokenForRecovery(string) (*apiv3.Token, bool, error) {
	return nil, false, errors.New("not implemented")
}

func (m *actionTokenManager) DeleteTokenByName(string) (int, error) {
	return http.StatusOK, nil
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		currentpass  string
		password     string
		expectsErr   bool
		expectErrMsg string
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
		{
			name:         "new password matches username",
			username:     "administrativeuser",
			currentpass:  "myfavoritepassword",
			password:     "administrativeuser",
			expectsErr:   true,
			expectErrMsg: "Password cannot be the same as the username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.username, tt.currentpass, tt.password, 12)
			switch {
			case tt.expectsErr && tt.expectErrMsg != "":
				assert.EqualError(t, err, tt.expectErrMsg)
				return
			case tt.expectsErr:
				assert.Error(t, err, "Expected an error but got nil")
				return
			case !tt.expectsErr:
				assert.NoError(t, err, "Expected no error but got one")
				return
			}
		})
	}

}

func TestImpersonationActionReturnsErrorWithoutFirstLoginSideEffect(t *testing.T) {
	disabled := false
	users := formatterUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}},
	}
	manager := &actionTokenManager{original: &apiv3.Token{UserID: "u-admin"}}
	impersonator := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		manager,
	)

	previous := settings.FirstLogin.Get()
	require.NoError(t, settings.FirstLogin.Set("true"))
	t.Cleanup(func() { _ = settings.FirstLogin.Set(previous) })

	req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target?action=impersonate", nil)
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
	response := httptest.NewRecorder()
	apiContext := &types.APIContext{ID: "u-target", Request: req, Response: response}

	err := (&Handler{Impersonator: impersonator}).Actions("impersonate", nil, apiContext)

	require.Error(t, err)
	apiErr, ok := err.(*httperror.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, apiErr.Code.Status)
	assert.Equal(t, "true", settings.FirstLogin.Get())
	assert.Equal(t, 0, manager.created)
	assert.Empty(t, response.Result().Cookies())
	assert.Equal(t, http.StatusOK, response.Code)
}

func TestImpersonationActionRunsFirstLoginSideEffectOnlyAfterSuccess(t *testing.T) {
	enabled := true
	users := formatterUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
	}
	manager := &actionTokenManager{original: &apiv3.Token{UserID: "u-admin"}}
	impersonator := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		manager,
	)
	previous := settings.FirstLogin.Get()
	require.NoError(t, settings.FirstLogin.Set("true"))
	t.Cleanup(func() { _ = settings.FirstLogin.Set(previous) })

	req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target?action=impersonate", nil)
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
	response := httptest.NewRecorder()

	err := (&Handler{Impersonator: impersonator}).Actions("impersonate", nil, &types.APIContext{
		ID: "u-target", Request: req, Response: response,
	})

	require.NoError(t, err)
	assert.Equal(t, "false", settings.FirstLogin.Get())
	assert.Equal(t, 1, manager.created)
	assert.Len(t, response.Result().Cookies(), 2)
	assert.Contains(t, response.Body.String(), `"impersonating":true`)
}

func TestImpersonationActionDoesNotReturnSetterErrorAfterResponse(t *testing.T) {
	enabled := true
	users := formatterUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
	}
	manager := &actionTokenManager{original: &apiv3.Token{UserID: "u-admin"}}
	impersonator := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		manager,
	)
	previous := settings.FirstLogin.Get()
	require.NoError(t, settings.FirstLogin.Set("true"))
	t.Cleanup(func() { _ = settings.FirstLogin.Set(previous) })

	req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target?action=impersonate", nil)
	req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
	req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
	response := httptest.NewRecorder()
	actionHandler := &Handler{
		Impersonator: impersonator,
		firstLoginSetter: func(string) error {
			return errors.New("settings unavailable")
		},
	}

	err := actionHandler.Actions("impersonate", nil, &types.APIContext{
		ID: "u-target", Request: req, Response: response,
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"impersonating":true`)
	assert.Equal(t, 1, manager.created)
	assert.Len(t, response.Result().Cookies(), 2)
}

func TestImpersonationActionStatusMatrix(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name       string
		auth       bool
		target     *apiv3.User
		nested     bool
		failing    bool
		tokenUser  string
		wantStatus int
	}{
		{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
		{name: "unauthorized target", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}, wantStatus: http.StatusForbidden},
		{name: "nested", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, nested: true, wantStatus: http.StatusConflict},
		{name: "lookup failure", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, failing: true, wantStatus: http.StatusInternalServerError},
		{name: "forged caller", auth: true, target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, tokenUser: "u-other", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			users := formatterUsers{"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}}}
			if test.target != nil {
				users["u-target"] = test.target
			}
			tokenUser := test.tokenUser
			if tokenUser == "" {
				tokenUser = "u-admin"
			}
			manager := &actionTokenManager{original: &apiv3.Token{UserID: tokenUser}}
			handler := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
				func(string) impersonation.GlobalRoleBindingLister {
					if test.failing {
						return actionFailingBindings{}
					}
					return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
				},
				func(string) impersonation.GlobalRoleGetter {
					return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
				manager,
			)
			req := httptest.NewRequest(http.MethodPost, "/v3/users/u-target?action=impersonate", nil)
			req.AddCookie(&http.Cookie{Name: tokens.CookieName, Value: "original"})
			if test.nested {
				req.AddCookie(&http.Cookie{Name: tokens.OriginalSessionCookie, Value: "nested"})
			}
			if test.auth {
				req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
			}
			response := httptest.NewRecorder()
			err := (&Handler{Impersonator: handler}).Actions("impersonate", nil, &types.APIContext{
				ID: "u-target", Request: req, Response: response,
			})

			require.Error(t, err)
			apiErr, ok := err.(*httperror.APIError)
			require.True(t, ok)
			assert.Equal(t, test.wantStatus, apiErr.Code.Status)
			assert.Empty(t, response.Result().Cookies())
			assert.Equal(t, 0, manager.created)
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

type formatterUsers map[string]*apiv3.User

func (u formatterUsers) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	user, ok := u[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("user"), name)
	}
	return user, nil
}

type formatterAttributes struct{}

func (formatterAttributes) Get(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
	return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), name)
}

type formatterAttributeValues map[string]*apiv3.UserAttribute

func (a formatterAttributeValues) Get(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
	attribute, ok := a[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("userattribute"), name)
	}
	return attribute, nil
}

type formatterBindings []apiv3.GlobalRoleBinding

func (b formatterBindings) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	return &apiv3.GlobalRoleBindingList{Items: b}, nil
}

type formatterRoles map[string]*apiv3.GlobalRole

func (r formatterRoles) Get(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
	role, ok := r[name]
	if !ok {
		return nil, apierrors.NewNotFound(apiv3.Resource("globalrole"), name)
	}
	return role, nil
}

func TestUserFormatterUsesImpersonationEligibilityInsteadOfUpdate(t *testing.T) {
	t.Parallel()

	enabled := true
	users := formatterUsers{
		"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-user":  {ObjectMeta: metav1.ObjectMeta{Name: "u-user"}},
	}
	impersonator := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		nil,
	)
	schema := &types.Schema{
		Version: types.APIVersion{Group: v3.UserGroupVersionKind.Group},
		ID:      v3.UserResource.Name,
	}

	tests := []struct {
		name       string
		caller     string
		wantAction bool
	}{
		{name: "admin without update", caller: "u-admin", wantAction: true},
		{name: "non admin", caller: "u-user", wantAction: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			apiContext := &types.APIContext{
				AccessControl: &fakeAccessControl{canDoFunc: func(_, _, verb string, _ *types.APIContext, _ map[string]interface{}, _ *types.Schema) error {
					if verb == "update" {
						return errors.New("update denied")
					}
					return nil
				}},
				Schema:     schema,
				URLBuilder: &stubURLBuilder{},
				Request: func() *http.Request {
					req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
					return req.WithContext(request.WithUser(req.Context(), httptestUser(test.caller)))
				}(),
			}
			resource := &types.RawResource{
				ID: "u-target", Schema: schema, Actions: map[string]string{}, Links: map[string]string{},
				Values: map[string]interface{}{
					"enabled":      enabled,
					"principalIds": []string{"local://target"},
				},
			}
			handler := &Handler{Impersonator: impersonator}

			handler.UserFormatter(apiContext, resource)

			_, gotAction := resource.Actions["impersonate"]
			assert.Equal(t, test.wantAction, gotAction)
		})
	}
}

func TestUserFormatterImpersonationDenyMatrix(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false
	tests := []struct {
		name      string
		caller    string
		target    *apiv3.User
		bindings  formatterBindings
		anonymous bool
		nested    bool
	}{
		{name: "disabled target", caller: "u-admin", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &disabled, PrincipalIDs: []string{"local://target"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "system target", caller: "u-admin", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"system:serviceaccount"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "target without principal", caller: "u-admin", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "administrator target", caller: "u-admin", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}, {UserName: "u-target", GlobalRoleName: "admin"}}},
		{name: "unauthenticated caller", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}, anonymous: true},
		{name: "caller lookup error", caller: "u-missing", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "target lookup error", caller: "u-admin", bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}},
		{name: "nested impersonation", caller: "u-admin", target: &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}}, bindings: formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}, nested: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			users := formatterUsers{
				"u-admin": {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
			}
			if test.target != nil {
				users["u-target"] = test.target
			}
			impersonator := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
				func(string) impersonation.GlobalRoleBindingLister { return test.bindings },
				func(string) impersonation.GlobalRoleGetter {
					return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
				},
				nil,
			)
			apiContext := &types.APIContext{
				AccessControl: &fakeAccessControl{canDoFunc: func(_, _, verb string, _ *types.APIContext, _ map[string]interface{}, _ *types.Schema) error {
					if verb == "update" {
						return errors.New("update denied")
					}
					return nil
				}},
				Schema:     &types.Schema{Version: types.APIVersion{Group: v3.UserGroupVersionKind.Group}, ID: v3.UserResource.Name},
				URLBuilder: &stubURLBuilder{},
			}
			req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
			if !test.anonymous {
				req = req.WithContext(request.WithUser(req.Context(), httptestUser(test.caller)))
			}
			if test.nested {
				req.AddCookie(&http.Cookie{Name: "R_IMPERSONATOR_SESS", Value: "nested"})
			}
			apiContext.Request = req
			resource := &types.RawResource{ID: "u-target", Schema: apiContext.Schema, Actions: map[string]string{}, Links: map[string]string{}}

			(&Handler{Impersonator: impersonator}).UserFormatter(apiContext, resource)

			assert.NotContains(t, resource.Actions, "impersonate")
		})
	}
}

func TestUserFormatterMatchesSharedEligibilityDecision(t *testing.T) {
	t.Parallel()

	enabled := true
	users := formatterUsers{
		"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
		"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
	}
	impersonator := impersonation.NewHandlerForTest(
		func(string) impersonation.UserGetter { return users },
		func(string) impersonation.UserAttributeGetter { return formatterAttributes{} },
		func(string) impersonation.GlobalRoleBindingLister {
			return formatterBindings{{UserName: "u-admin", GlobalRoleName: "admin"}}
		},
		func(string) impersonation.GlobalRoleGetter {
			return formatterRoles{"admin": {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true}}
		},
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
	req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
	apiContext := &types.APIContext{
		AccessControl: &fakeAccessControl{canDoFunc: func(string, string, string, *types.APIContext, map[string]interface{}, *types.Schema) error {
			return errors.New("not allowed")
		}},
		Schema:     &types.Schema{ID: v3.UserResource.Name},
		URLBuilder: &stubURLBuilder{},
		Request:    req,
	}
	resource := &types.RawResource{ID: "u-target", Schema: apiContext.Schema, Actions: map[string]string{}, Links: map[string]string{}}

	eligible, err := impersonator.CanImpersonateRequest(req, resource.ID)
	require.NoError(t, err)
	(&Handler{Impersonator: impersonator}).UserFormatter(apiContext, resource)
	_, advertised := resource.Actions["impersonate"]
	assert.Equal(t, eligible, advertised)
}

func TestUserFormatterRejectsAllAdminTargetBindingKinds(t *testing.T) {
	t.Parallel()

	enabled := true
	tests := []struct {
		name       string
		binding    apiv3.GlobalRoleBinding
		attributes impersonation.UserAttributeGetter
		role       *apiv3.GlobalRole
	}{
		{
			name:    "custom full access user binding",
			binding: apiv3.GlobalRoleBinding{UserName: "u-target", GlobalRoleName: "custom-admin"},
			role: &apiv3.GlobalRole{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-admin"},
				Rules: []rbacv1.PolicyRule{
					{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
					{NonResourceURLs: []string{"*"}, Verbs: []string{"*"}},
				},
			},
		},
		{
			name:    "user principal binding",
			binding: apiv3.GlobalRoleBinding{UserPrincipalName: "local://target", GlobalRoleName: "admin"},
			role:    &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
		},
		{
			name:    "group principal binding",
			binding: apiv3.GlobalRoleBinding{GroupPrincipalName: "ldap_group://admins", GlobalRoleName: "admin"},
			attributes: formatterAttributeValues{"u-target": {
				GroupPrincipals: map[string]apiv3.Principals{
					"ldap": {Items: []apiv3.Principal{{ObjectMeta: metav1.ObjectMeta{Name: "ldap_group://admins"}}}},
				},
			}},
			role: &apiv3.GlobalRole{ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			users := formatterUsers{
				"u-admin":  {ObjectMeta: metav1.ObjectMeta{Name: "u-admin"}},
				"u-target": {ObjectMeta: metav1.ObjectMeta{Name: "u-target"}, Enabled: &enabled, PrincipalIDs: []string{"local://target"}},
			}
			attributes := test.attributes
			if attributes == nil {
				attributes = formatterAttributes{}
			}
			impersonator := impersonation.NewHandlerForTest(
				func(string) impersonation.UserGetter { return users },
				func(string) impersonation.UserAttributeGetter { return attributes },
				func(string) impersonation.GlobalRoleBindingLister {
					return formatterBindings{
						{UserName: "u-admin", GlobalRoleName: "admin"},
						test.binding,
					}
				},
				func(string) impersonation.GlobalRoleGetter {
					return formatterRoles{
						"admin":        {ObjectMeta: metav1.ObjectMeta{Name: "admin"}, Builtin: true},
						"custom-admin": test.role,
					}
				},
				nil,
			)
			req := httptest.NewRequest(http.MethodGet, "/v3/users/u-target", nil)
			req = req.WithContext(request.WithUser(req.Context(), httptestUser("u-admin")))
			apiContext := &types.APIContext{
				AccessControl: &fakeAccessControl{canDoFunc: func(string, string, string, *types.APIContext, map[string]interface{}, *types.Schema) error {
					return errors.New("not allowed")
				}},
				Schema:     &types.Schema{ID: v3.UserResource.Name},
				URLBuilder: &stubURLBuilder{},
				Request:    req,
			}
			resource := &types.RawResource{ID: "u-target", Schema: apiContext.Schema, Actions: map[string]string{}, Links: map[string]string{}}

			(&Handler{Impersonator: impersonator}).UserFormatter(apiContext, resource)

			assert.NotContains(t, resource.Actions, "impersonate")
		})
	}
}

func httptestUser(name string) *k8suser.DefaultInfo {
	return &k8suser.DefaultInfo{Name: name}
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

type fakePasswordUpdater struct {
	err      error
	user     *apiv3.User
	password string
}

func (f *fakePasswordUpdater) VerifyAndUpdatePassword(string, string, string) error {
	return nil
}

func (f *fakePasswordUpdater) SetPassword(user *apiv3.User, newPassword string) error {
	f.user = user
	f.password = newPassword
	return f.err
}

type fakeResponseWriter struct {
	code int
	obj  interface{}
}

func (f *fakeResponseWriter) Write(_ *types.APIContext, code int, obj interface{}) {
	f.code = code
	f.obj = obj
}

func TestSetPassword(t *testing.T) {
	t.Parallel()

	userID := "u-abc"
	newPassword := "fake-new-password"
	user := &apiv3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
			UID:  "fake-uid",
		},
		Username: "fake-username",
	}

	tests := []struct {
		name         string
		pwdUpdater   *fakePasswordUpdater
		userGetErr   error
		wantErr      string
		wantNotFound bool
	}{
		{
			name:         "user not found",
			pwdUpdater:   &fakePasswordUpdater{},
			userGetErr:   apierrors.NewNotFound(schema.GroupResource{Resource: "users"}, userID),
			wantErr:      "not found",
			wantNotFound: true,
		},
		{
			name:       "password is set",
			pwdUpdater: &fakePasswordUpdater{},
		},
		{
			name:       "error setting password",
			pwdUpdater: &fakePasswordUpdater{err: errors.New("unexpected error")},
			wantErr:    "unexpected error",
		},
		{
			name:       "error getting user",
			pwdUpdater: &fakePasswordUpdater{},
			userGetErr: errors.New("unexpected error"),
			wantErr:    "unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := &mockStore{
				byIDResults: []map[string]interface{}{{
					types.ResourceFieldID:    userID,
					client.UserFieldUsername: user.Username,
				}},
			}
			userClient := &fakes.UserInterfaceMock{
				GetFunc: func(name string, _ metav1.GetOptions) (*apiv3.User, error) {
					assert.Equal(t, userID, name)
					return user, tt.userGetErr
				},
			}
			h := &Handler{
				UserClient: userClient,
				PwdChanger: tt.pwdUpdater,
			}

			body, _ := json.Marshal(map[string]string{"newPassword": newPassword})
			req := httptest.NewRequest("POST", "/v3/users/"+userID+"?action=setpassword", bytes.NewReader(body))
			rw := &fakeResponseWriter{}
			apiCtx := &types.APIContext{
				Request:        req,
				ResponseWriter: rw,
				ID:             userID,
				Schema:         &types.Schema{Store: store},
			}

			err := h.setPassword(apiCtx)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, tt.wantNotFound, httperror.IsNotFound(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, user, tt.pwdUpdater.user)
			assert.Equal(t, newPassword, tt.pwdUpdater.password)
			assert.Equal(t, false, store.updateData[client.UserFieldMustChangePassword])
			assert.Equal(t, http.StatusOK, rw.code)
		})
	}
}
