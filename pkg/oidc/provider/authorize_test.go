package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"github.com/rancher/rancher/pkg/settings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	providermocks "github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/oidc/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAuthEndpoint(t *testing.T) {
	const (
		fakeTokenName   = "fake-token-name"
		fakeTokenValue  = "fake-token-value"
		fakeUserID      = "fake-user-id"
		fakeCode        = "fake-code"
		fakeRedirectUri = "https://www.rancher.com"
		fakeClientID    = "client-id"
		fakeClientName  = "client-name"
		fakeServerUrl   = "https://www.fake.com"
	)
	type mockParams struct {
		extTokenStore   *exttokenstore.SystemStore
		extSecrets      *fake.MockCacheInterface[*corev1.Secret]
		tokenCache      *fake.MockNonNamespacedCacheInterface[*v3.Token]
		userLister      *fake.MockNonNamespacedCacheInterface[*v3.User]
		oidcClientCache *fake.MockNonNamespacedCacheInterface[*v3.OIDCClient]
		codeCreator     *mocks.MockcodeCreator
		sessionAdder    *mocks.MocksessionAdder
	}
	_ = settings.ServerURL.Set(fakeServerUrl)
	fakeTime := time.Unix(0, 0)
	ctrl := gomock.NewController(t)
	tests := map[string]struct {
		req                                func() *http.Request
		mockSetup                          func(mockParams)
		wantRedirect                       string
		wantHttpCode                       int
		wantError                          string
		wantAccessControlAllowOriginHeader string
	}{
		"redirect with code when Rancher token is present": {
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.sessionAdder.EXPECT().Add(fakeCode, session.Session{
					ClientID:      fakeClientID,
					TokenName:     fakeTokenName,
					Scope:         []string{"openid"},
					CodeChallenge: "code-challenge",
					CreatedAt:     fakeTime,
				})
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
				m.codeCreator.EXPECT().GenerateCode().Return(fakeCode, nil)
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?code=fake-code",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"redirect with code when ext token is present": {
			mockSetup: func(m mockParams) {
				// no legacy token
				m.tokenCache.EXPECT().
					Get(fakeTokenName).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "token not found"))
				// but an ext token
				fakePrincipal := ext.TokenPrincipal{
					Name:        "world",
					Provider:    "local", // ext token reader checks for provider, cannot be empty
					DisplayName: "myself",
					LoginName:   "hello",
				}
				fakePrincipalBytes, _ := json.Marshal(fakePrincipal)
				fakeTokenHash, _ := hashers.GetHasher().CreateHash(fakeTokenValue)
				fakeSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.NewTime(time.Now()), // required for ex token expiration check
						Name:              fakeTokenName,
						Labels: map[string]string{
							exttokenstore.UserIDLabel:     fakeUserID,
							exttokenstore.SecretKindLabel: exttokenstore.SecretKindLabelValue,
						},
						UID: "bombastic",
					},
					Data: map[string][]byte{
						exttokenstore.FieldDescription:    []byte(""),
						exttokenstore.FieldEnabled:        []byte("true"),
						exttokenstore.FieldHash:           []byte(fakeTokenHash),
						exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
						exttokenstore.FieldLastUpdateTime: []byte("13:00:05"),
						exttokenstore.FieldPrincipal:      fakePrincipalBytes,
						exttokenstore.FieldTTL:            []byte("4000"),
						exttokenstore.FieldUID:            []byte("2905498-kafld-lkad"),
						exttokenstore.FieldUserID:         []byte(fakeUserID),
					},
				}
				m.extSecrets.EXPECT().
					Get(exttokenstore.TokenNamespace, fakeTokenName).
					Return(&fakeSecret, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.sessionAdder.EXPECT().Add(fakeCode, session.Session{
					ClientID:      fakeClientID,
					TokenName:     "ext/" + fakeTokenName,
					Scope:         []string{"openid"},
					CodeChallenge: "code-challenge",
					CreatedAt:     fakeTime,
				})
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
				m.codeCreator.EXPECT().GenerateCode().Return(fakeCode, nil)
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?code=fake-code",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"redirect to login page if Rancher token is not present": {
			req: func() *http.Request {
				return &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&code_challenge=code-challenge&response_type=code&redirect_uri=https://www.rancher.com&scope=openid&client_id=client-id",
					},
					Method: http.MethodGet,
				}
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeServerUrl + "/dashboard/auth/login?client_id=client-id&code_challenge=code-challenge&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fwww.rancher.com&response_type=code&scope=openid",
		},
		"redirect to login page if Rancher token does not match": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&code_challenge=code-challenge&response_type=code&redirect_uri=https://www.rancher.com&scope=openid&client_id=client-id",
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  "invalid",
					UserID: fakeUserID,
				}, nil)
			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeServerUrl + "/dashboard/auth/login?client_id=client-id&code_challenge=code-challenge&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fwww.rancher.com&response_type=code&scope=openid",
		},
		"redirect to login page if ext token does not match": {
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&code_challenge=code-challenge&response_type=code&redirect_uri=https://www.rancher.com&scope=openid&client_id=client-id",
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				// no legacy token
				m.tokenCache.EXPECT().
					Get(fakeTokenName).
					Return(nil, errors.NewNotFound(schema.GroupResource{}, "token not found"))
				// but an ext token, with an invalid hash
				fakePrincipal := ext.TokenPrincipal{
					Name:        "world",
					Provider:    "local", // ext token reader checks for provider, cannot be empty
					DisplayName: "myself",
					LoginName:   "hello",
				}
				fakePrincipalBytes, _ := json.Marshal(fakePrincipal)
				invalidTokenHash, _ := hashers.GetHasher().CreateHash("invalid")
				fakeSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.NewTime(time.Now()), // required for ex token expiration check
						Name:              fakeTokenName,
						Labels: map[string]string{
							exttokenstore.UserIDLabel:     fakeUserID,
							exttokenstore.SecretKindLabel: exttokenstore.SecretKindLabelValue,
						},
						UID: "bombastic",
					},
					Data: map[string][]byte{
						exttokenstore.FieldDescription:    []byte(""),
						exttokenstore.FieldEnabled:        []byte("true"),
						exttokenstore.FieldHash:           []byte(invalidTokenHash),
						exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
						exttokenstore.FieldLastUpdateTime: []byte("13:00:05"),
						exttokenstore.FieldPrincipal:      fakePrincipalBytes,
						exttokenstore.FieldTTL:            []byte("4000"),
						exttokenstore.FieldUID:            []byte("2905498-kafld-lkad"),
						exttokenstore.FieldUserID:         []byte(fakeUserID),
					},
				}
				m.extSecrets.EXPECT().
					Get(exttokenstore.TokenNamespace, fakeTokenName).
					Return(&fakeSecret, nil)

			},
			wantHttpCode: http.StatusFound,
			wantRedirect: fakeServerUrl + "/dashboard/auth/login?client_id=client-id&code_challenge=code-challenge&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fwww.rancher.com&response_type=code&scope=openid",
		},
		"response type not supported": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=none&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?error=unsupported_response_type&error_description=response+type+not+supported",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"code challenge method not supported": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=plain&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?error=invalid_request&error_description=challenge_method+not+supported%2C+only+S256+is+supported",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"invalid scope with no configured scopes": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid+invalidscope&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?error=invalid_scope&error_description=invalid+scope%3A+invalidscope",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"invalid scope with configured scopes": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid+invalidscope&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
							Scopes:       []string{"openid", "profile", "rancher:test"},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?error=invalid_scope&error_description=invalid+scope%3A+invalidscope",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"configured scopes": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid+offline_access+profile+rancher:test&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.sessionAdder.EXPECT().Add(fakeCode, session.Session{
					ClientID:      fakeClientID,
					TokenName:     fakeTokenName,
					Scope:         []string{"openid", "offline_access", "profile", "rancher:test"},
					CodeChallenge: "code-challenge",
					CreatedAt:     fakeTime,
				})
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
							Scopes:       []string{"openid", "profile", "rancher:test", "offline_access"},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
				m.codeCreator.EXPECT().GenerateCode().Return(fakeCode, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?code=fake-code",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"missing code challenge": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{fakeRedirectUri},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			wantHttpCode:                       http.StatusFound,
			wantRedirect:                       fakeRedirectUri + "?error=invalid_request&error_description=missing+code_challenge",
			wantAccessControlAllowOriginHeader: fakeRedirectUri,
		},
		"missing redirect uri": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid",
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"missing redirect_uri"}`,
		},
		"redirect uri same as host uri": {
			// check is done after token verification. kind of token does not matter
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=https://rancher.com",
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Times(0)
			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"redirect_uri can't be the same as the host uri"}`,
		},
		"oidc client not registered": {
			// check is done after token verification. kind of token does not matter
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return(nil, errors.NewNotFound(schema.GroupResource{}, fakeClientID))
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"error retrieving OIDC client:  \"client-id\" not found"}`,
		},
		"redirect uri not registered": {
			// check is done after token verification. kind of token does not matter
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeTokenName,
					},
					Token:  fakeTokenValue,
					UserID: fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{
					ObjectMeta: metav1.ObjectMeta{
						Name: fakeUserID,
					},
				}, nil)
				m.oidcClientCache.EXPECT().GetByIndex(OIDCClientByIDIndex, fakeClientID).Return([]*v3.OIDCClient{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: fakeClientName,
						},
						Spec: v3.OIDCClientSpec{
							RedirectURIs: []string{"anotherurl"},
						},
						Status: v3.OIDCClientStatus{
							ClientID: fakeClientID,
						},
					},
				}, nil)
			},
			req: func() *http.Request {
				req := &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=" + fakeRedirectUri,
					},
					Method: http.MethodGet,
				}
				req.Header = map[string][]string{
					"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
				}

				return req
			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"redirect_uri is not registered"}`,
		},
		"invalid redirect url": {
			// check is done after token verification. kind of token does not matter
			mockSetup: func(m mockParams) {
				m.tokenCache.EXPECT().Get(fakeTokenName).Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{Name: fakeTokenName},
					Token:      fakeTokenValue,
					UserID:     fakeUserID,
				}, nil)
				m.userLister.EXPECT().Get(fakeUserID).Return(&v3.User{ObjectMeta: metav1.ObjectMeta{Name: fakeUserID}}, nil)
			},
			req: func() *http.Request {
				return &http.Request{
					URL: &url.URL{
						Scheme:   "https",
						Host:     "rancher.com",
						RawQuery: "code_challenge_method=S256&response_type=code&code_challenge=code-challenge&client_id=client-id&scope=openid&redirect_uri=%3A%2F%2F",
					},
					Method: http.MethodGet,
					Header: map[string][]string{
						"Cookie": {"R_SESS=" + fakeTokenName + ":" + fakeTokenValue},
					},
				}

			},
			wantHttpCode: http.StatusBadRequest,
			wantError:    `{"error":"invalid_request","error_description":"invalid redirect_uri"}`,
		},
	}

	// register auth provider
	mockProvider := providermocks.NewMockAuthProvider(ctrl)
	mockProvider.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()
	providers.SetProviders(map[string]common.AuthProvider{"local": mockProvider})
	t.Cleanup(func() { providers.SetProviders(nil) })

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// no t.Parallel() because of shared controller, provider map
			tc := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
			sc := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			scc := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
			uc := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
			sc.EXPECT().Cache().Return(scc)
			uc.EXPECT().Cache().Return(nil)
			ets := exttokenstore.NewSystem(nil, nil, sc, uc, tc, nil, nil, nil, nil, nil)
			m := mockParams{
				tokenCache:      tc,
				extTokenStore:   ets,
				extSecrets:      scc,
				userLister:      fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl),
				oidcClientCache: fake.NewMockNonNamespacedCacheInterface[*v3.OIDCClient](ctrl),
				sessionAdder:    mocks.NewMocksessionAdder(ctrl),
				codeCreator:     mocks.NewMockcodeCreator(ctrl),
			}
			if test.mockSetup != nil {
				test.mockSetup(m)
			}
			h := newAuthorizeHandler(m.extTokenStore, m.userLister, m.sessionAdder, m.codeCreator, m.oidcClientCache)
			h.now = func() time.Time {
				return fakeTime
			}
			rec := httptest.NewRecorder()

			h.authEndpoint(rec, test.req())

			assert.Equal(t, test.wantHttpCode, rec.Code)
			if test.wantRedirect != "" {
				assert.Equal(t, test.wantRedirect, rec.Header().Get("Location"))
			}
			assert.Equal(t, test.wantAccessControlAllowOriginHeader, rec.Header().Get("Access-Control-Allow-Origin"))
			if test.wantError != "" {
				assert.JSONEq(t, test.wantError, strings.TrimSpace(rec.Body.String()))
			}
		})
	}
}
