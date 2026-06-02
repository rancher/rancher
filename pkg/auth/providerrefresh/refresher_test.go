package providerrefresh

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/rancher/norman/types"
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokens "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
)

func TestRefreshAttributes(t *testing.T) {
	var tokenUpdateCalled bool
	var tokenDeleteCalled bool

	userLocal := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		Username:   "admin",
		PrincipalIDs: []string{
			"local://user-abcde",
		},
	}

	userShibboleth := apiv3.User{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		Username:   "admin",
		PrincipalIDs: []string{
			"shibboleth_user://user1",
		},
	}

	attribsIn := apiv3.UserAttribute{
		ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]apiv3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
	}

	wantNoExtra := apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]apiv3.Principals{
			"local":      {},
			"shibboleth": {},
		},
		ExtraByProvider: map[string]map[string][]string{},
	}

	wantLocal := apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]apiv3.Principals{
			"local":      {},
			"shibboleth": {},
		},
		ExtraByProvider: map[string]map[string][]string{
			local.Name: {
				common.UserAttributePrincipalID: {"local://user-abcde"},
				common.UserAttributeUserName:    {"admin"},
			},
		},
	}

	wantShibboleth := apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]apiv3.Principals{
			"local":      {},
			"shibboleth": {},
		},
		ExtraByProvider: map[string]map[string][]string{
			saml.ShibbolethName: {
				common.UserAttributePrincipalID: {"shibboleth_user://user1"},
				common.UserAttributeUserName:    {"user1"},
			},
		},
	}

	loginTokenLocal := apiv3.Token{
		UserID:       "user-abcde",
		IsDerived:    false,
		AuthProvider: local.Name,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "local://user-abcde",
			},
			LoginName: "admin",
			Provider:  local.Name,
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: "local://user-abcde",
				common.UserAttributeUserName:    "admin",
			},
		},
	}

	derivedTokenLocal := loginTokenLocal
	derivedTokenLocal.IsDerived = true

	derivedTokenShibboleth := apiv3.Token{
		UserID:       "user-abcde",
		IsDerived:    true,
		AuthProvider: saml.ShibbolethName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shibboleth_user://user1",
			},
			LoginName: "user1",
			Provider:  saml.ShibbolethName,
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
			UserPrincipal: ext.TokenPrincipal{
				Name:      "local://user-abcde",
				Provider:  local.Name,
				LoginName: "admin",
				ExtraInfo: map[string]string{
					common.UserAttributePrincipalID: "local://user-abcde",
					common.UserAttributeUserName:    "admin",
				},
			},
		},
	}

	localPrincipalBytes, _ := json.Marshal(eLoginTokenLocal.Spec.UserPrincipal)
	eLoginSecretLocal := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-login-local"},
		Data: map[string][]byte{
			exttokens.FieldEnabled:        []byte("true"),
			exttokens.FieldHash:           []byte("kla9jkdmj"),
			exttokens.FieldKind:           []byte(exttokens.IsLogin),
			exttokens.FieldLastUpdateTime: []byte("13:00:05"),
			exttokens.FieldPrincipal:      localPrincipalBytes,
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
			UserPrincipal: ext.TokenPrincipal{
				Name:      "shibboleth_user://user1",
				Provider:  saml.ShibbolethName,
				LoginName: "user1",
				ExtraInfo: map[string]string{
					common.UserAttributePrincipalID: "shibboleth_user://user1",
					common.UserAttributeUserName:    "user1",
				},
			},
		},
	}

	shibbolethPrincipalBytes, _ := json.Marshal(eDerivedTokenShibboleth.Spec.UserPrincipal)
	eDerivedSecretShibboleth := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "user-abcde-derived-shibboleth"},
		Data: map[string][]byte{
			exttokens.FieldEnabled:        []byte("true"),
			exttokens.FieldHash:           []byte("kla9jkdmj"),
			exttokens.FieldKind:           []byte(""),
			exttokens.FieldLastUpdateTime: []byte("13:00:05"),
			exttokens.FieldPrincipal:      shibbolethPrincipalBytes,
			exttokens.FieldTTL:            []byte("4000"),
			exttokens.FieldUID:            []byte("2905498-kafld-lkad"),
			exttokens.FieldUserID:         []byte("user-abcde"),
		},
	}

	eTokenSetupEmpty := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{}, nil).
			AnyTimes()
	}

	eTokenSetupLoginLocal := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eLoginSecretLocal,
			}, nil).
			AnyTimes()
	}

	eTokenSetupDerivedLocal := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eDerivedSecretLocal,
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
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eLoginSecretLocal,
				&eDerivedSecretLocal,
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

	eTokenSetupLocalPatch := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			Patch("cattle-tokens",
				"user-abcde-login-local",
				ktypes.JSONPatchType,
				[]byte(`[{"op":"replace","path":"/data/enabled","value":"ZmFsc2U="}]`)).
			DoAndReturn(func(ns, name string, pt ktypes.PatchType, patch []byte, subresources ...string) (*corev1.Secret, error) {
				tokenUpdateCalled = true
				return nil, nil
			}).
			AnyTimes()
		secrets.EXPECT().
			Patch("cattle-tokens",
				"user-abcde-derived-local",
				ktypes.JSONPatchType,
				[]byte(`[{"op":"replace","path":"/data/enabled","value":"ZmFsc2U="}]`)).
			DoAndReturn(func(ns, name string, pt ktypes.PatchType, patch []byte, subresources ...string) (*corev1.Secret, error) {
				tokenUpdateCalled = true
				return nil, nil
			}).
			AnyTimes()
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eLoginSecretLocal,
				&eDerivedSecretLocal,
			}, nil).
			AnyTimes()
	}

	eTokenSetupDerivedLocalPatch := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		secrets.EXPECT().
			Patch("cattle-tokens",
				gomock.Any(),
				ktypes.JSONPatchType,
				[]byte(`[{"op":"replace","path":"/data/enabled","value":"ZmFsc2U="}]`)).
			DoAndReturn(func(ns, name string, pt ktypes.PatchType, patch []byte, subresources ...string) (*corev1.Secret, error) {
				tokenUpdateCalled = true
				return nil, nil
			}).
			AnyTimes()
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eDerivedSecretLocal,
			}, nil).
			AnyTimes()
	}

	eTokenSetupDerivedShibboleth := func(
		secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
		scache *fake.MockCacheInterface[*corev1.Secret]) {
		scache.EXPECT().
			List("cattle-tokens", gomock.Any()).
			Return([]*corev1.Secret{
				&eDerivedSecretShibboleth,
			}, nil).
			AnyTimes()
		scache.EXPECT().
			Get("cattle-tokens", gomock.Any()).
			Return(&eDerivedSecretShibboleth, nil).
			AnyTimes()
	}

	tests := []struct {
		name                  string
		user                  *apiv3.User
		attribs               *apiv3.UserAttribute // argument to refreshAttributes
		providerDisabled      bool
		providerDisabledError error
		tokens                []*apiv3.Token
		eTokens               []*ext.Token
		enabled               bool
		deleted               bool
		want                  *apiv3.UserAttribute // result expected from refreshAttributes
		eTokenSetup           func(
			secrets *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
			scache *fake.MockCacheInterface[*corev1.Secret])
	}{
		{
			name:        "local user no tokens",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*apiv3.Token{},
			eTokens:     []*ext.Token{},
			enabled:     true,
			want:        &wantNoExtra,
			eTokenSetup: eTokenSetupEmpty,
		},
		// from here on out test cases are pairs testing the same thing, one each for v3 and ext tokens
		{
			name:        "local user with login token",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*apiv3.Token{&loginTokenLocal},
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
			tokens:      []*apiv3.Token{&derivedTokenLocal},
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
			tokens:      []*apiv3.Token{&derivedTokenLocal},
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
			eTokenSetup: eTokenSetupDerivedLocalPatch,
		},
		{
			name:        "user with login and derived tokens",
			user:        &userLocal,
			attribs:     &attribsIn,
			tokens:      []*apiv3.Token{&loginTokenLocal, &derivedTokenLocal},
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
			tokens:      []*apiv3.Token{&derivedTokenShibboleth},
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
			tokens:           []*apiv3.Token{&loginTokenLocal, &derivedTokenLocal},
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
			eTokenSetup:      eTokenSetupLocalPatch,
			providerDisabled: true,
			deleted:          true,
			enabled:          false,
		},
		{
			name:                  "error in determining if provider is disabled, tokens left unchanged",
			user:                  &userLocal,
			attribs:               &attribsIn,
			tokens:                []*apiv3.Token{&loginTokenLocal, &derivedTokenLocal},
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

	for _, tt := range tests {
		tokenUpdateCalled = false
		tokenDeleteCalled = false
		t.Run(tt.name, func(t *testing.T) {
			providers.SetProviders(map[string]common.AuthProvider{
				local.Name: &mockLocalProvider{
					canAccess:   tt.enabled,
					disabled:    tt.providerDisabled,
					disabledErr: tt.providerDisabledError,
				},
				saml.ShibbolethName: &mockShibbolethProvider{},
			})

			ctrl := gomock.NewController(t)
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)

			users.EXPECT().Cache().Return(nil)
			secrets.EXPECT().Cache().Return(scache)

			// standard capture of delete and update events. See
			// also the `tokens` interface used by the refresher
			// below, same thing for the v3 tokens.
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

			tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
			tokenClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(token *apiv3.Token) (*apiv3.Token, error) {
				tokenUpdateCalled = true
				return token.DeepCopy(), nil
			}).AnyTimes()

			r := &refresher{
				tokenLister: &fakes.TokenListerMock{
					ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
						return tt.tokens, nil
					},
				},
				userLister: &fakes.UserListerMock{
					GetFunc: func(_, _ string) (*apiv3.User, error) {
						return tt.user, nil
					},
				},
				tokens: &fakes.TokenInterfaceMock{
					DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
						tokenDeleteCalled = true
						return nil
					},
				},
				tokenMGR: tokens.NewMockedManager(tokenClient, nil),
				extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
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
			user := apiv3.User{
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

func TestTriggerUserRefreshSkipsAnnotatedUser(t *testing.T) {
	t.Parallel()

	userID := "u-abcdef"
	attribs := &apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
			Annotations: map[string]string{
				common.ProviderRefreshErrorAnnotation: "user not found",
			},
		},
	}

	r := &refresher{
		ensureAndGetUserAttribute: func(userName string) (*apiv3.UserAttribute, bool, error) {
			return attribs.DeepCopy(), false, nil
		},
		userLister: &fakes.UserListerMock{
			GetFunc: func(namespace, name string) (*apiv3.User, error) {
				return &apiv3.User{
					ObjectMeta:   metav1.ObjectMeta{Name: userID},
					PrincipalIDs: []string{"azuread_user://some-guid"},
				}, nil
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{},
	}

	r.triggerUserRefresh(userID, true)

	assert.Empty(t, r.userAttributes.(*fakes.UserAttributeInterfaceMock).UpdateCalls(),
		"Update should not be called for annotated user attribute")
}

func TestTriggerUserRefreshNoopsWhenAlreadyPending(t *testing.T) {
	t.Parallel()

	userID := "u-abcdef"
	cachedAttribs := &apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
	}
	// Stale lister copy: NeedsRefresh=false. Fresh API copy: already true
	// (a concurrent trigger won the race). triggerUserRefresh should re-read
	// from the API, see NeedsRefresh already set, and skip Update.
	freshAttribs := &apiv3.UserAttribute{
		ObjectMeta:   metav1.ObjectMeta{Name: userID},
		NeedsRefresh: true,
	}

	r := &refresher{
		ensureAndGetUserAttribute: func(userName string) (*apiv3.UserAttribute, bool, error) {
			return cachedAttribs.DeepCopy(), false, nil
		},
		userLister: &fakes.UserListerMock{
			GetFunc: func(namespace, name string) (*apiv3.User, error) {
				return &apiv3.User{
					ObjectMeta:   metav1.ObjectMeta{Name: userID},
					PrincipalIDs: []string{"azuread_user://some-guid"},
				}, nil
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			GetFunc: func(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
				return freshAttribs.DeepCopy(), nil
			},
		},
	}

	r.triggerUserRefresh(userID, true)

	mock := r.userAttributes.(*fakes.UserAttributeInterfaceMock)
	assert.Len(t, mock.GetCalls(), 1, "Get should be called to re-read from the API server")
	assert.Empty(t, mock.UpdateCalls(),
		"Update should be skipped when the fresh copy already has NeedsRefresh=true")
}

func TestTriggerUserRefreshRetriesOnConflict(t *testing.T) {
	t.Parallel()

	userID := "u-abcdef"
	cachedAttribs := &apiv3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{Name: userID},
	}
	var updateAttempts int

	r := &refresher{
		ensureAndGetUserAttribute: func(userName string) (*apiv3.UserAttribute, bool, error) {
			return cachedAttribs.DeepCopy(), false, nil
		},
		userLister: &fakes.UserListerMock{
			GetFunc: func(namespace, name string) (*apiv3.User, error) {
				return &apiv3.User{
					ObjectMeta:   metav1.ObjectMeta{Name: userID},
					PrincipalIDs: []string{"azuread_user://some-guid"},
				}, nil
			},
		},
		userAttributes: &fakes.UserAttributeInterfaceMock{
			GetFunc: func(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
				return cachedAttribs.DeepCopy(), nil
			},
			UpdateFunc: func(ua *apiv3.UserAttribute) (*apiv3.UserAttribute, error) {
				updateAttempts++
				if updateAttempts == 1 {
					return nil, apierrors.NewConflict(
						schema.GroupResource{Group: "management.cattle.io", Resource: "userattributes"},
						userID, errors.New("the object has been modified"))
				}
				return ua, nil
			},
		},
	}

	r.triggerUserRefresh(userID, true)

	assert.Equal(t, 2, updateAttempts, "Update should retry after a 409 conflict and then succeed")
}

func TestRefreshAttributesNonTransientError(t *testing.T) {
	t.Parallel()

	const providerName = "testoidc"

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-abcde"},
		PrincipalIDs: []string{providerName + "_user://some-guid"},
	}

	attribs := &apiv3.UserAttribute{
		ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
		GroupPrincipals: map[string]apiv3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
	}

	loginToken := &apiv3.Token{
		UserID:       "user-abcde",
		IsDerived:    false,
		AuthProvider: providerName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://some-guid"},
			Provider:   providerName,
		},
	}

	providers.SetProviders(map[string]common.AuthProvider{
		providerName: &mockNonTransientProvider{},
	})

	ctrl := gomock.NewController(t)
	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)

	users.EXPECT().Cache().Return(nil)
	secrets.EXPECT().Cache().Return(scache)
	scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

	tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

	var tokenDeleteCalled, tokenUpdateCalled bool
	r := &refresher{
		tokenLister: &fakes.TokenListerMock{
			ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
				return []*apiv3.Token{loginToken}, nil
			},
		},
		userLister: &fakes.UserListerMock{
			GetFunc: func(_, _ string) (*apiv3.User, error) {
				return user, nil
			},
		},
		tokens: &fakes.TokenInterfaceMock{
			DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
				tokenDeleteCalled = true
				return nil
			},
		},
		tokenMGR: tokens.NewMockedManager(tokenClient, nil),
		extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
			exttokens.NewTimeHandler(),
			exttokens.NewHashHandler(),
			exttokens.NewAuthHandler()),
	}

	got, err := r.refreshAttributes(attribs)
	assert.Nil(t, got)
	assert.Error(t, err)

	var nte *common.NonTransientError
	assert.ErrorAs(t, err, &nte)
	assert.False(t, tokenDeleteCalled, "tokens should not be deleted when NonTransientError is returned")
	assert.False(t, tokenUpdateCalled, "tokens should not be updated when NonTransientError is returned")
}

type mockNonTransientProvider struct {
	mockLocalProvider
}

func (p *mockNonTransientProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	return nil, &common.NonTransientError{Err: fmt.Errorf("oauth2: invalid_grant: Session not active")}
}

func (p *mockNonTransientProvider) IsDisabledProvider() (bool, error) {
	return false, nil
}

type mockLocalProvider struct {
	canAccess      bool
	canAccessErr   error
	getPrincipalFn func(string, accessor.TokenAccessor) (apiv3.Principal, error)
	disabled       bool
	disabledErr    error
}

func (p *mockLocalProvider) IsDisabledProvider() (bool, error) {
	return p.disabled, p.disabledErr
}

func (p *mockLocalProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLocalProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockLocalProvider) GetName() string {
	panic("not implemented")
}

func (p *mockLocalProvider) AuthenticateUser(http.ResponseWriter, *http.Request, any) (apiv3.Principal, []apiv3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]apiv3.Principal, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	if p.getPrincipalFn != nil {
		return p.getPrincipalFn(principalID, token)
	}
	return token.GetUserPrincipal(), nil
}

func (p *mockLocalProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockLocalProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	panic("not implemented")
}

func (p *mockLocalProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	return []apiv3.Principal{}, nil
}

func (p *mockLocalProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	return p.canAccess, p.canAccessErr
}

func (p *mockLocalProvider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockLocalProvider) UsesUserSecrets() bool      { return false }
func (p *mockLocalProvider) CanRefreshPrincipals() bool { return true }

func (p *mockLocalProvider) CleanupResources(*apiv3.AuthConfig) error {
	return nil
}

type mockShibbolethProvider struct {
	enabled    bool
	enabledErr error
}

func (p *mockShibbolethProvider) IsDisabledProvider() (bool, error) {
	return p.enabled, p.enabledErr
}

func (p *mockShibbolethProvider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockShibbolethProvider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	panic("not implemented")
}

func (p *mockShibbolethProvider) GetName() string {
	panic("not implemented")
}

func (p *mockShibbolethProvider) AuthenticateUser(http.ResponseWriter, *http.Request, any) (apiv3.Principal, []apiv3.Principal, string, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]apiv3.Principal, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	return token.GetUserPrincipal(), nil
}

func (p *mockShibbolethProvider) CustomizeSchema(schema *types.Schema) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	panic("not implemented")
}

func (p *mockShibbolethProvider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	return []apiv3.Principal{}, errors.New("Not implemented")
}

func (p *mockShibbolethProvider) CanAccessWithGroupProviders(userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	return true, nil
}

func (p *mockShibbolethProvider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return map[string][]string{
		common.UserAttributePrincipalID: {userPrincipal.ExtraInfo[common.UserAttributePrincipalID]},
		common.UserAttributeUserName:    {userPrincipal.ExtraInfo[common.UserAttributeUserName]},
	}
}

func (p *mockShibbolethProvider) UsesUserSecrets() bool      { return false }
func (p *mockShibbolethProvider) CanRefreshPrincipals() bool { return false }

func (p *mockShibbolethProvider) CleanupResources(*apiv3.AuthConfig) error {
	return nil
}

type mockGitHubAppProvider struct {
	mockLocalProvider
	refetchCalled   bool
	refetchSecret   string
	groupPrincipals []apiv3.Principal
}

func (p *mockGitHubAppProvider) RefetchGroupPrincipals(principalID, secret string) ([]apiv3.Principal, error) {
	p.refetchCalled = true
	p.refetchSecret = secret
	return p.groupPrincipals, nil
}

func (p *mockGitHubAppProvider) IsDisabledProvider() (bool, error) {
	return false, nil
}

func TestRefreshAttributesNoPerUserSecrets(t *testing.T) {
	const providerName = "githubapp"

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-ghapp"},
		PrincipalIDs: []string{providerName + "_user://12345"},
	}

	attribs := &apiv3.UserAttribute{
		ObjectMeta:      metav1.ObjectMeta{Name: "user-ghapp"},
		GroupPrincipals: map[string]apiv3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
	}

	loginToken := &apiv3.Token{
		UserID:       "user-ghapp",
		IsDerived:    false,
		AuthProvider: providerName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://12345"},
			Provider:   providerName,
			LoginName:  "testuser",
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: providerName + "_user://12345",
				common.UserAttributeUserName:    "testuser",
			},
		},
	}

	wantGroupPrincipals := []apiv3.Principal{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: providerName + "_team://org:team1"},
			PrincipalType: "group",
			Provider:      providerName,
		},
	}

	mockProvider := &mockGitHubAppProvider{
		mockLocalProvider: mockLocalProvider{canAccess: true},
		groupPrincipals:   wantGroupPrincipals,
	}

	providers.SetProviders(map[string]common.AuthProvider{
		providerName: mockProvider,
	})

	ctrl := gomock.NewController(t)
	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)

	users.EXPECT().Cache().Return(nil)
	secrets.EXPECT().Cache().Return(scache)
	scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

	tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

	r := &refresher{
		tokenLister: &fakes.TokenListerMock{
			ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
				return []*apiv3.Token{loginToken}, nil
			},
		},
		userLister: &fakes.UserListerMock{
			GetFunc: func(_, _ string) (*apiv3.User, error) {
				return user, nil
			},
		},
		tokens:   &fakes.TokenInterfaceMock{},
		tokenMGR: tokens.NewMockedManager(tokenClient, nil),
		extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
			exttokens.NewTimeHandler(),
			exttokens.NewHashHandler(),
			exttokens.NewAuthHandler()),
	}

	got, err := r.refreshAttributes(attribs)
	assert.NoError(t, err)
	assert.True(t, mockProvider.refetchCalled, "RefetchGroupPrincipals should be called for providers without per-user secrets")
	assert.Empty(t, mockProvider.refetchSecret, "secret should be empty for providers without per-user secrets")
	assert.Equal(t, wantGroupPrincipals, got.GroupPrincipals[providerName].Items)
}

type mockSecretProvider struct {
	mockLocalProvider
	refetchErr    error
	refetchGroups []apiv3.Principal
	refetchCalled bool
}

func (p *mockSecretProvider) UsesUserSecrets() bool      { return true }
func (p *mockSecretProvider) CanRefreshPrincipals() bool { return true }

func (p *mockSecretProvider) RefetchGroupPrincipals(principalID, secret string) ([]apiv3.Principal, error) {
	p.refetchCalled = true
	return p.refetchGroups, p.refetchErr
}

func (p *mockSecretProvider) IsDisabledProvider() (bool, error) {
	return p.disabled, p.disabledErr
}

type mockRefetchErrorProvider struct {
	mockLocalProvider
	refetchErr error
}

func (p *mockRefetchErrorProvider) RefetchGroupPrincipals(principalID, secret string) ([]apiv3.Principal, error) {
	return nil, p.refetchErr
}

func (p *mockRefetchErrorProvider) IsDisabledProvider() (bool, error) {
	return false, nil
}

func TestRefreshAttributesEarlyErrors(t *testing.T) {
	t.Run("user lister error", func(t *testing.T) {
		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) {
					return nil, fmt.Errorf("user lookup failed")
				},
			},
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
			GroupPrincipals: map[string]apiv3.Principals{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "error getting user")
	})

	t.Run("token lister error", func(t *testing.T) {
		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) {
					return &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"}}, nil
				},
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return nil, fmt.Errorf("token list failed")
				},
			},
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
			GroupPrincipals: map[string]apiv3.Principals{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "error listing tokens")
	})

	t.Run("ext token list error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return(nil, fmt.Errorf("cache error")).AnyTimes()

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) {
					return &apiv3.User{ObjectMeta: metav1.ObjectMeta{Name: "user-abcde"}}, nil
				},
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{}, nil
				},
			},
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-abcde"},
			GroupPrincipals: map[string]apiv3.Principals{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "error listing ext tokens")
	})
}

func TestRefreshAttributesPerUserSecrets(t *testing.T) {
	const providerName = "github"

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-secret"},
		PrincipalIDs: []string{providerName + "_user://12345"},
	}

	existingGroups := []apiv3.Principal{
		{ObjectMeta: metav1.ObjectMeta{Name: providerName + "_team://org:existing"}},
	}

	t.Run("secret not found preserves state", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
		mgrSecretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		mgrSecretCache.EXPECT().
			Get("cattle-system", "user-secret-secret").
			Return(nil, apierrors.NewNotFound(corev1.Resource("secrets"), "user-secret-secret")).
			AnyTimes()

		mockProvider := &mockSecretProvider{
			mockLocalProvider: mockLocalProvider{canAccess: true},
		}
		providers.SetProviders(map[string]common.AuthProvider{
			providerName: mockProvider,
		})

		derivedToken := &apiv3.Token{
			UserID:       "user-secret",
			IsDerived:    true,
			AuthProvider: providerName,
			UserPrincipal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://12345"},
				Provider:   providerName,
			},
		}

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{derivedToken}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, mgrSecretCache),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "user-secret"},
			GroupPrincipals: map[string]apiv3.Principals{
				providerName: {Items: existingGroups},
			},
			ExtraByProvider: map[string]map[string][]string{
				providerName: {"key": {"existing-value"}},
			},
		}

		got, err := r.refreshAttributes(attribs)
		assert.NoError(t, err)
		assert.False(t, mockProvider.refetchCalled, "RefetchGroupPrincipals should not be called when secret is missing")
		assert.Equal(t, existingGroups, got.GroupPrincipals[providerName].Items, "existing group principals should be preserved")
		assert.Equal(t, map[string][]string{"key": {"existing-value"}}, got.ExtraByProvider[providerName], "existing extra attributes should be preserved")
	})

	t.Run("secret hard error returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)
		mgrSecretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		mgrSecretCache.EXPECT().
			Get("cattle-system", "user-secret-secret").
			Return(nil, fmt.Errorf("connection refused")).
			AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockSecretProvider{
				mockLocalProvider: mockLocalProvider{canAccess: true},
			},
		})

		loginToken := &apiv3.Token{
			UserID:       "user-secret",
			IsDerived:    false,
			AuthProvider: providerName,
			UserPrincipal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://12345"},
				Provider:   providerName,
			},
		}

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, mgrSecretCache),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-secret"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.Error(t, err)
	})
}

func TestRefreshAttributesRefetchErrors(t *testing.T) {
	const providerName = "testprovider"

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-refetch"},
		PrincipalIDs: []string{providerName + "_user://abc"},
	}

	existingGroups := []apiv3.Principal{
		{ObjectMeta: metav1.ObjectMeta{Name: providerName + "_team://existing-group"}},
	}

	loginToken := &apiv3.Token{
		UserID:       "user-refetch",
		IsDerived:    false,
		AuthProvider: providerName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://abc"},
			Provider:   providerName,
			LoginName:  "testuser",
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: providerName + "_user://abc",
				common.UserAttributeUserName:    "testuser",
			},
		},
	}

	t.Run("transient error preserves groups", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{canAccess: true},
				refetchErr:        fmt.Errorf("connection timeout"),
			},
		})

		var tokenDeleteCalled bool
		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens: &fakes.TokenInterfaceMock{
				DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
					tokenDeleteCalled = true
					return nil
				},
			},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "user-refetch"},
			GroupPrincipals: map[string]apiv3.Principals{
				providerName: {Items: existingGroups},
			},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.NoError(t, err)
		assert.Equal(t, existingGroups, got.GroupPrincipals[providerName].Items, "existing groups should be preserved on transient error")
		assert.False(t, tokenDeleteCalled, "login tokens should not be deleted when errorConfirmingLogins is set")
	})

	t.Run("no access blanks principal and deletes tokens", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{canAccess: false},
				refetchErr:        errors.New("no access"),
			},
		})

		derivedToken := &apiv3.Token{
			UserID:       "user-refetch",
			IsDerived:    true,
			AuthProvider: providerName,
			UserPrincipal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://abc"},
				Provider:   providerName,
			},
		}

		var tokenDeleteCalled, tokenUpdateCalled bool
		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken, derivedToken}, nil
				},
			},
			tokens: &fakes.TokenInterfaceMock{
				DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
					tokenDeleteCalled = true
					return nil
				},
			},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		tokenClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(token *apiv3.Token) (*apiv3.Token, error) {
			tokenUpdateCalled = true
			return token.DeepCopy(), nil
		}).AnyTimes()

		attribs := &apiv3.UserAttribute{
			ObjectMeta: metav1.ObjectMeta{Name: "user-refetch"},
			GroupPrincipals: map[string]apiv3.Principals{
				providerName: {Items: existingGroups},
			},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.NoError(t, err)
		assert.Nil(t, got.GroupPrincipals[providerName].Items, "group principals should be cleared on no access")
		assert.True(t, tokenDeleteCalled, "login tokens should be deleted when user has no access")
		assert.True(t, tokenUpdateCalled, "derived tokens should be disabled when user has no access")
	})
}

func TestRefreshAttributesAccessAndPrincipalErrors(t *testing.T) {
	const providerName = "testprovider"

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-err"},
		PrincipalIDs: []string{providerName + "_user://abc"},
	}

	loginToken := &apiv3.Token{
		UserID:       "user-err",
		IsDerived:    false,
		AuthProvider: providerName,
		UserPrincipal: apiv3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: providerName + "_user://abc"},
			Provider:   providerName,
			LoginName:  "testuser",
			ExtraInfo: map[string]string{
				common.UserAttributePrincipalID: providerName + "_user://abc",
				common.UserAttributeUserName:    "testuser",
			},
		},
	}

	t.Run("can access error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{
					canAccess:    false,
					canAccessErr: fmt.Errorf("ldap connection failed"),
				},
				refetchErr: nil,
			},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-err"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.Error(t, err)
	})

	t.Run("get principal error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{
					canAccess: true,
					getPrincipalFn: func(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
						return apiv3.Principal{}, fmt.Errorf("principal fetch failed")
					},
				},
			},
			local.Name: &mockLocalProvider{
				canAccess: true,
				getPrincipalFn: func(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
					return apiv3.Principal{}, fmt.Errorf("local fallback also failed")
				},
			},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-err"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "principal fetch failed")
	})

	t.Run("extra attributes with nil ExtraByProvider", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{canAccess: true},
			},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-err"},
			GroupPrincipals: map[string]apiv3.Principals{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.NoError(t, err)
		assert.NotNil(t, got.ExtraByProvider, "ExtraByProvider should be initialized when nil")
		assert.Contains(t, got.ExtraByProvider, providerName)
	})

	t.Run("login token delete not found is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{canAccess: false},
			},
		})

		tokenClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(token *apiv3.Token) (*apiv3.Token, error) {
			return token.DeepCopy(), nil
		}).AnyTimes()

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens: &fakes.TokenInterfaceMock{
				DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
					return apierrors.NewNotFound(corev1.Resource("tokens"), "token")
				},
			},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-err"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.NoError(t, err, "not-found errors during login token deletion should be ignored")
		assert.NotNil(t, got)
	})

	t.Run("login token delete error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{}, nil).AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockRefetchErrorProvider{
				mockLocalProvider: mockLocalProvider{canAccess: false},
			},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{loginToken}, nil
				},
			},
			tokens: &fakes.TokenInterfaceMock{
				DeleteFunc: func(_ string, _ *metav1.DeleteOptions) error {
					return fmt.Errorf("delete failed")
				},
			},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-err"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.Error(t, err)
	})
}

func TestRefreshAttributesExtTokenDisable(t *testing.T) {
	const providerName = local.Name

	user := &apiv3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "user-ext"},
		PrincipalIDs: []string{"local://user-ext"},
	}

	eDerivedToken := ext.Token{
		ObjectMeta: metav1.ObjectMeta{Name: "user-ext-derived"},
		Spec: ext.TokenSpec{
			UserID: "user-ext",
			Kind:   "",
			UserPrincipal: ext.TokenPrincipal{
				Name:     "local://user-ext",
				Provider: providerName,
			},
		},
	}

	principalBytes, _ := json.Marshal(eDerivedToken.Spec.UserPrincipal)
	eDerivedSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "user-ext-derived",
			Labels: map[string]string{tokens.UserIDLabel: "user-ext"},
		},
		Data: map[string][]byte{
			exttokens.FieldEnabled:        []byte("true"),
			exttokens.FieldHash:           []byte("somehash"),
			exttokens.FieldKind:           []byte(""),
			exttokens.FieldLastUpdateTime: []byte("13:00:05"),
			exttokens.FieldPrincipal:      principalBytes,
			exttokens.FieldTTL:            []byte("4000"),
			exttokens.FieldUID:            []byte("uid-123"),
			exttokens.FieldUserID:         []byte("user-ext"),
		},
	}

	t.Run("ext derived tokens disabled on lost access", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{&eDerivedSecret}, nil).AnyTimes()
		scache.EXPECT().Get("cattle-tokens", gomock.Any()).Return(&eDerivedSecret, nil).AnyTimes()

		var patchCalled bool
		secrets.EXPECT().
			Patch("cattle-tokens", "user-ext-derived", ktypes.JSONPatchType, gomock.Any()).
			DoAndReturn(func(ns, name string, pt ktypes.PatchType, patch []byte, subresources ...string) (*corev1.Secret, error) {
				patchCalled = true
				return nil, nil
			}).
			AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockLocalProvider{canAccess: false},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-ext"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		_, err := r.refreshAttributes(attribs)
		assert.NoError(t, err)
		assert.True(t, patchCalled, "ext derived token should be disabled via secret patch")
	})

	t.Run("disable token error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
		scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
		users := fake.NewMockNonNamespacedControllerInterface[*apiv3.User, *apiv3.UserList](ctrl)
		tokenClient := fake.NewMockNonNamespacedClientInterface[*apiv3.Token, *apiv3.TokenList](ctrl)

		users.EXPECT().Cache().Return(nil)
		secrets.EXPECT().Cache().Return(scache)
		scache.EXPECT().List("cattle-tokens", gomock.Any()).Return([]*corev1.Secret{&eDerivedSecret}, nil).AnyTimes()
		scache.EXPECT().Get("cattle-tokens", gomock.Any()).Return(&eDerivedSecret, nil).AnyTimes()

		secrets.EXPECT().
			Patch("cattle-tokens", gomock.Any(), ktypes.JSONPatchType, gomock.Any()).
			Return(nil, fmt.Errorf("patch failed")).
			AnyTimes()

		providers.SetProviders(map[string]common.AuthProvider{
			providerName: &mockLocalProvider{canAccess: false},
		})

		r := &refresher{
			userLister: &fakes.UserListerMock{
				GetFunc: func(_, _ string) (*apiv3.User, error) { return user, nil },
			},
			tokenLister: &fakes.TokenListerMock{
				ListFunc: func(_ string, _ labels.Selector) ([]*apiv3.Token, error) {
					return []*apiv3.Token{}, nil
				},
			},
			tokens:   &fakes.TokenInterfaceMock{},
			tokenMGR: tokens.NewMockedManager(tokenClient, nil),
			extTokenStore: exttokens.NewSystem(nil, nil, secrets, users, nil, nil,
				exttokens.NewTimeHandler(),
				exttokens.NewHashHandler(),
				exttokens.NewAuthHandler()),
		}

		attribs := &apiv3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: "user-ext"},
			GroupPrincipals: map[string]apiv3.Principals{},
			ExtraByProvider: map[string]map[string][]string{},
		}

		got, err := r.refreshAttributes(attribs)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "error disabling token")
	})
}
