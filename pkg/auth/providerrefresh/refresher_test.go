package providerrefresh

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func Test_refreshAttributes(t *testing.T) {
	// common structures

	userLocal := v3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		Username:   "admin",
		PrincipalIDs: []string{
			"local://user-abcde",
		},
	}

	userShibboleth := v3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		Username:   "admin",
		PrincipalIDs: []string{
			"shibboleth_user://user1",
		},
	}

	attribsIn := v3.UserAttribute{
		ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]v3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
	}

	wantNoExtra := v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]v3.Principals{
			"local":      v3.Principals{},
			"shibboleth": v3.Principals{},
		},
		ExtraByProvider: map[string]map[string][]string{},
	}

	wantLocal := v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]v3.Principals{
			"local":      v3.Principals{},
			"shibboleth": v3.Principals{},
		},
		ExtraByProvider: map[string]map[string][]string{
			providers.LocalProvider: map[string][]string{
				common.UserAttributePrincipalID: []string{"local://user-abcde"},
				common.UserAttributeUserName:    []string{"admin"},
			},
		},
	}

	wantShibboleth := v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]v3.Principals{
			"local":      v3.Principals{},
			"shibboleth": v3.Principals{},
		},
		ExtraByProvider: map[string]map[string][]string{
			saml.ShibbolethName: map[string][]string{
				common.UserAttributePrincipalID: []string{"shibboleth_user://user1"},
				common.UserAttributeUserName:    []string{"user1"},
			},
		},
	}

	loginTokenLocal := v3.Token{
		UserID:       "user-abcde",
		IsDerived:    false,
		AuthProvider: providers.LocalProvider,
		UserPrincipal: v3.Principal{
			Provider: providers.LocalProvider,
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: "local://user-abcde",
				common.UserAttributeUserName:    "admin",
			},
		},
	}

	derivedTokenLocal := loginTokenLocal
	derivedTokenLocal.IsDerived = true

	derivedTokenShibboleth := v3.Token{
		UserID:       "user-abcde",
		IsDerived:    true,
		AuthProvider: saml.ShibbolethName,
		UserPrincipal: v3.Principal{
			Provider: saml.ShibbolethName,
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: "shibboleth_user://user1",
				common.UserAttributeUserName:    "user1",
			},
		},
	}

	// BEWARE: for the ext tokens we see here into the internals, in
	// particular into the field structure of the backing secret.  Using the
	// exported constants for field names should help detecting changes
	// breaking things.

	eLoginTokenLocal := ext.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-login-local"},
		Spec: ext.TokenSpec{
			UserID: "user-abcde",
			Kind:   exttokens.IsLogin,
		},
		Status: ext.TokenStatus{
			AuthProvider: providers.LocalProvider,
			LoginName:    "admin",
			PrincipalID:  "local://user-abcde",
		},
	}

	eLoginSecretLocal := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-login-local"},
		Data: map[string][]byte{
			exttokens.FieldAnnotations:    []byte("null"),
			exttokens.FieldAuthProvider:   []byte(providers.LocalProvider),
			exttokens.FieldDisplayName:    []byte(""),
			exttokens.FieldEnabled:        []byte("true"),
			exttokens.FieldHash:           []byte("kla9jkdmj"),
			exttokens.FieldKind:           []byte(exttokens.IsLogin),
			exttokens.FieldLabels:         []byte("null"),
			exttokens.FieldLastUpdateTime: []byte("13:00:05"),
			exttokens.FieldLoginName:      []byte("admin"),
			exttokens.FieldPrincipalID:    []byte("local://user-abcde"),
			exttokens.FieldTTL:            []byte("4000"),
			exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
			exttokens.FieldUserID:         []byte("user-abcde"),
		},
	}

	eDerivedTokenLocal := eLoginTokenLocal
	eDerivedTokenLocal.ObjectMeta.Name = "user-abcde-derived-local"
	eDerivedTokenLocal.Spec.Kind = ""

	eDerivedSecretLocal := *eLoginSecretLocal.DeepCopy() // copy the map
	eDerivedSecretLocal.ObjectMeta.Name = "user-abcde-derived-local"
	eDerivedSecretLocal.Data[exttokens.FieldKind] = []byte("")

	eDerivedTokenShibboleth := ext.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-derived-shibboleth"},
		Spec: ext.TokenSpec{
			UserID: "user-abcde",
			Kind:   "",
		},
		Status: ext.TokenStatus{
			AuthProvider: saml.ShibbolethName,
			LoginName:    "user1",
			PrincipalID:  "shibboleth_user://user1",
		},
	}

	eDerivedSecretShibboleth := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-derived-shibboleth"},
		Data: map[string][]byte{
			exttokens.FieldAnnotations:    []byte("null"),
			exttokens.FieldAuthProvider:   []byte(saml.ShibbolethName),
			exttokens.FieldDisplayName:    []byte(""),
			exttokens.FieldEnabled:        []byte("true"),
			exttokens.FieldHash:           []byte("kla9jkdmj"),
			exttokens.FieldKind:           []byte(""),
			exttokens.FieldLabels:         []byte("null"),
			exttokens.FieldLastUpdateTime: []byte("13:00:05"),
			exttokens.FieldLoginName:      []byte("user1"),
			exttokens.FieldPrincipalID:    []byte("shibboleth_user://user1"),
			exttokens.FieldTTL:            []byte("4000"),
			exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
			exttokens.FieldUserID:         []byte("user-abcde"),
		},
	}

	eTokenSetupEmpty := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return(&corev1.SecretList{}, nil).
			AnyTimes()
	}

	eTokenSetupLoginLocal := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return(&corev1.SecretList{
				Items: []corev1.Secret{
					eLoginSecretLocal,
				},
			}, nil).
			AnyTimes()
	}

	eTokenSetupDerivedLocal := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return(&corev1.SecretList{
				Items: []corev1.Secret{
					eDerivedSecretLocal,
				},
			}, nil).
			AnyTimes()
		scache.EXPECT().
			Get("cattle-tokens", gomock.Any()).
			Return(&eDerivedSecretLocal, nil).
			AnyTimes()
	}

	eTokenSetupLocal := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return(&corev1.SecretList{
				Items: []corev1.Secret{
					eLoginSecretLocal,
					eDerivedSecretLocal,
				},
			}, nil).
			AnyTimes()
		scache.EXPECT().
			Get("cattle-tokens", "user-abcde-login-local").
			Return(&eLoginSecretLocal, nil).
			AnyTimes()
		scache.EXPECT().
			Get("cattle-tokens", "user-abcde-derived-local").
			Return(&eDerivedSecretLocal, nil).
			AnyTimes()
	}

	eTokenSetupDerivedShibboleth := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return(&corev1.SecretList{
				Items: []corev1.Secret{
					eDerivedSecretShibboleth,
				},
			}, nil).
			AnyTimes()
		scache.EXPECT().
			Get("cattle-tokens", gomock.Any()).
			Return(&eDerivedSecretShibboleth, nil).
			AnyTimes()
	}

	tests := []struct {
		name                  string
		user                  *v3.User
		attribs               *v3.UserAttribute // argument to refreshAttributes
		providerDisabled      bool
		providerDisabledError error
		tokens                []*v3.Token
		eTokens               []*ext.Token
		enabled               bool
		deleted               bool
		want                  *v3.UserAttribute // result expected from refreshAttributes
		eTokenSetup           func(
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			scache *fake.MockCacheInterface[*corev1.Secret])
	}{
		{
			name:        "local user no tokens",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{},
			eTokens:     []*ext.Token{},
			enabled:     true,
			want:        &wantNoExtra,
			eTokenSetup: eTokenSetupEmpty,
		},
		// from here on out test cases are pairs testing the same thing, one each for legacy and ext tokens
		{
			name:        "local user with login token",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{&loginTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupEmpty,
		},
		{
			name:        "local user with ext login token",
			user:        &userLocal,
			attribs:     &attribsIn,
			eTokens:     []*ext.Token{&eLoginTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupLoginLocal,
		},
		{
			name:        "local user with derived token",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{&derivedTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupEmpty,
		},
		{
			name:        "local user with derived ext token",
			user:        &userLocal,
			attribs:     &attribsIn,
			eTokens:     []*ext.Token{&eDerivedTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupDerivedLocal,
		},
		{
			name:        "user with derived token disabled in provider",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{&derivedTokenLocal},
			enabled:     false,
			want:        &wantNoExtra,
			eTokenSetup: eTokenSetupEmpty,
		},
		{
			name:        "user with derived ext token disabled in provider",
			user:        &userLocal,
			attribs:     &attribsIn,
			eTokens:     []*ext.Token{&eDerivedTokenLocal},
			enabled:     false,
			want:        &wantNoExtra,
			eTokenSetup: eTokenSetupDerivedLocal,
		},
		{
			name:        "user with login and derived tokens",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{&loginTokenLocal, &derivedTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupEmpty,
		},
		{
			name:        "user with ext login and derived tokens",
			user:        &userLocal,
			attribs:     &attribsIn,
			eTokens:     []*ext.Token{&eLoginTokenLocal, &eDerivedTokenLocal},
			enabled:     true,
			want:        &wantLocal,
			eTokenSetup: eTokenSetupLocal,
		},
		{
			name:        "shibboleth user",
			user:        &userShibboleth,
			attribs:     &attribsIn,
			tokens:      []*v3.Token{&derivedTokenShibboleth},
			enabled:     true,
			want:        &wantShibboleth,
			eTokenSetup: eTokenSetupEmpty,
		},
		{
			name:        "shibboleth user, ext",
			user:        &userShibboleth,
			attribs:     &attribsIn,
			eTokens:     []*ext.Token{&eDerivedTokenShibboleth},
			enabled:     true,
			want:        &wantShibboleth,
			eTokenSetup: eTokenSetupDerivedShibboleth,
		},
		{
			name:             "disabled provider, disabled/deleted tokens",
			user:             &userLocal,
			attribs:          &attribsIn,
			tokens:           []*v3.Token{&loginTokenLocal, &derivedTokenLocal},
			want:             &wantNoExtra,
			eTokenSetup:      eTokenSetupEmpty,
			providerDisabled: true,
			deleted:          true,
			enabled:          false,
		},
		{
			name:             "disabled provider, disabled/deleted ext tokens",
			user:             &userLocal,
			attribs:          &attribsIn,
			eTokens:          []*ext.Token{&eLoginTokenLocal, &eDerivedTokenLocal},
			want:             &wantNoExtra,
			eTokenSetup:      eTokenSetupLocal,
			providerDisabled: true,
			deleted:          true,
			enabled:          false,
		},
		{
			name:                  "error in determining if provider is disabled, tokens left unchanged",
			user:                  &userLocal,
			attribs:               &attribsIn,
			tokens:                []*v3.Token{&loginTokenLocal, &derivedTokenLocal},
			want:                  &wantLocal,
			eTokenSetup:           eTokenSetupEmpty,
			providerDisabled:      true,
			providerDisabledError: fmt.Errorf("unable to determine if provider was disabled"),
			deleted:               false,
			enabled:               true,
		},
		{
			name:                  "error in determining if provider is disabled, ext tokens left unchanged",
			user:                  &userLocal,
			attribs:               &attribsIn,
			eTokens:               []*ext.Token{&eLoginTokenLocal, &eDerivedTokenLocal},
			want:                  &wantLocal,
			eTokenSetup:           eTokenSetupLocal,
			providerDisabled:      true,
			providerDisabledError: fmt.Errorf("unable to determine if provider was disabled"),
			deleted:               false,
			enabled:               true,
		},
	}

	providers.ProviderNames = map[string]bool{
		providers.LocalProvider: true,
		saml.ShibbolethName:     true,
	}

	for _, tt := range tests {
		tokenUpdateCalled := false
		tokenDeleteCalled := false
		t.Run(tt.name, func(t *testing.T) {
			providers.Providers = map[string]common.AuthProvider{
				providers.LocalProvider: &mockLocalProvider{
					canAccess:   tt.enabled,
					disabled:    tt.providerDisabled,
					disabledErr: tt.providerDisabledError,
				},
				saml.ShibbolethName: &mockShibbolethProvider{},
			}

			ctrl := gomock.NewController(t)
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(scache)

			// standard capture of delete and update events. See
			// also the `tokens` interface used by the refresher
			// below, same thing for the legacy tokens.
			secrets.EXPECT().
				Update(gomock.Any()).
				DoAndReturn(func(secret *corev1.Secret) (*corev1.Secret, error) {
					tokenUpdateCalled = true
					return secret, nil
				}).AnyTimes()
			secrets.EXPECT().
				Delete("cattle-tokens", gomock.Any(), gomock.Any()).
				DoAndReturn(func(space, name string, opts *metav1.DeleteOptions) error {
					tokenDeleteCalled = true
					return nil
				}).AnyTimes()

			// additional ext token setup
			if tt.eTokenSetup != nil {
				tt.eTokenSetup(secrets, scache)
			}

			r := &refresher{
				tokenLister: &fakes.TokenListerMock{
					ListFunc: func(_ string, _ labels.Selector) ([]*v3.Token, error) {
						return tt.tokens, nil
					},
				},
				userLister: &fakes.UserListerMock{
					GetFunc: func(_, _ string) (*v3.User, error) {
						return tt.user, nil
					},
				},
				tokens: &fakes.TokenInterfaceMock{
					DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
						tokenDeleteCalled = true
						return nil
					},
				},
				tokenMGR: tokens.NewMockedManager(&fakes.TokenInterfaceMock{
					UpdateFunc: func(_ *v3.Token) (*v3.Token, error) {
						tokenUpdateCalled = true
						return nil, nil
					},
				}),
				extTokenStore: exttokens.NewSystem(nil, secrets, users, nil,
					exttokens.NewTimeHandler(),
					exttokens.NewHashHandler(),
					exttokens.NewAuthHandler()),
			}
			got, err := r.refreshAttributes(tt.attribs)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
			assert.NotEqual(t, tt.enabled, tokenUpdateCalled)
			assert.Equal(t, tt.deleted, tokenDeleteCalled)
		})
	}
}

func TestGetPrincipalIDForProvider(t *testing.T) {
	const testUserUsername = "tUser"
	tests := []struct {
		name               string
		userPrincipalIds   []string
		providerName       string
		desiredPrincipalId string
	}{
		{
			name:               "basic test",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name:               "no principal for provider",
			userPrincipalIds:   []string{fmt.Sprintf("azure_user://%s", testUserUsername)},
			providerName:       "not-a-provider",
			desiredPrincipalId: "",
		},
		{
			name:               "local provider principal",
			userPrincipalIds:   []string{fmt.Sprintf("local://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: fmt.Sprintf("local://%s", testUserUsername),
		},
		{
			name:               "local provider missing principal",
			userPrincipalIds:   []string{fmt.Sprintf("local_user://%s", testUserUsername)},
			providerName:       "local",
			desiredPrincipalId: "",
		},
		{
			name: "multiple providers, correct one (first) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
		{
			name: "multiple providers, correct one (last) chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("azure_user://%s", testUserUsername),
			},
			providerName:       "azure",
			desiredPrincipalId: fmt.Sprintf("azure_user://%s", testUserUsername),
		},
		{
			name: "multiple correct providers, first one chosen",
			userPrincipalIds: []string{
				fmt.Sprintf("ldap_user://%s", testUserUsername),
				fmt.Sprintf("ldap_user://%s", "tUser2"),
			},
			providerName:       "ldap",
			desiredPrincipalId: fmt.Sprintf("ldap_user://%s", testUserUsername),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			user := v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: testUserUsername,
				},
				PrincipalIDs: test.userPrincipalIds,
			}
			outputPrincipalID := GetPrincipalIDForProvider(test.providerName, &user)
			assert.Equal(t, test.desiredPrincipalId, outputPrincipalID, "got a different principal id than expected")
		})
	}
}

type mockLocalProvider struct {
	canAccess   bool
	disabled    bool
	disabledErr error
}

func (p *mockLocalProvider) IsDisabledProvider() (bool, error) {
	return p.disabled, p.disabledErr
}

func (p *mockLocalProvider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLocalProvider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLocalProvider) GetName() string {
	panic("not implemented")
}

func (p *mockLocalProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error) {
	return token.GetUserPrincipal(), nil
}

func (p *mockLocalProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockLocalProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, nil
}

func (p *mockLocalProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return p.canAccess, nil
}

func (p *mockLocalProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: []string{userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    []string{userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockLocalProvider) CleanupResources(*v3.AuthConfig) error {
	return nil
}

type mockShibbolethProvider struct {
	enabled    bool
	enabledErr error
}

func (p *mockShibbolethProvider) IsDisabledProvider() (bool, error) {
	return p.enabled, p.enabledErr
}

func (p *mockShibbolethProvider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockShibbolethProvider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockShibbolethProvider) GetName() string {
	panic("not implemented")
}

func (p *mockShibbolethProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]v3.Principal, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error) {
	return token.GetUserPrincipal(), nil
}

func (p *mockShibbolethProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return []v3.Principal{}, errors.New("Not implemented")
}

func (p *mockShibbolethProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error) {
	return true, nil
}

func (p *mockShibbolethProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: []string{userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    []string{userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockShibbolethProvider) CleanupResources(*v3.AuthConfig) error {
	return nil
}
