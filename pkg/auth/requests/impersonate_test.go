package requests

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAuthenticateImpersonation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	userInfo := &user.DefaultInfo{
		Name:   "user",
		UID:    "user",
		Groups: nil,
		Extra:  nil,
	}

	tests := []struct {
		desc                  string
		req                   func() *http.Request
		sar                   func(req *http.Request) sar.SubjectAccessReview
		extTokenStore         func() *exttokenstore.SystemStore
		wantUserInfo          *user.DefaultInfo
		wantNextHandlerCalled bool
		wantErr               string
		status                int
	}{
		{
			desc: "unauthenticated",
			req: func() *http.Request {
				return &http.Request{}
			},
			sar: func(_ *http.Request) sar.SubjectAccessReview {
				return mocks.NewMockSubjectAccessReview(ctrl)
			},
			status: http.StatusUnauthorized,
		},
		{
			desc: "no impersonation",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(_ *http.Request) sar.SubjectAccessReview {
				return mocks.NewMockSubjectAccessReview(ctrl)
			},
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate user",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"impUser"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)

				return mock
			},
			wantUserInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra:  nil,
			},
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate group",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":  {"impUser"},
						"Impersonate-Group": {"impGroup"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateGroup(req, "user", "impGroup").Return(true, nil)

				return mock
			},
			wantUserInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"impGroup", "system:authenticated"},
				Extra:  nil,
			},
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate extras",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-foo":            {"bar"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"foo":            {"bar"},
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().Get("kubeconfig-u-user5zfww").Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubeconfig-u-user5zfww",
					},
					UserID: "impUser",
				}, nil)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				secrets.EXPECT().Cache().Return(nil)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantUserInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra: map[string][]string{
					"foo":            {"bar"},
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				},
			},
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate extras, ext token as origin",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-foo":            {"bar"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"foo":            {"bar"},
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().
					Get("kubeconfig-u-user5zfww").
					Return(nil, errors.New("unexpected error"))
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
				secrets.EXPECT().Cache().Return(scache)
				scache.EXPECT().
					Get("cattle-tokens", "kubeconfig-u-user5zfww").
					Return(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "kubeconfig-u-user5zfww",
						},
						Data: map[string][]byte{
							exttokenstore.FieldUserID: []byte("impUser"),
							// everything else to satisfy the ext token read checks
							exttokenstore.FieldAnnotations:    []byte("null"),
							exttokenstore.FieldAuthProvider:   []byte("local"),
							exttokenstore.FieldDisplayName:    []byte(""),
							exttokenstore.FieldEnabled:        []byte("true"),
							exttokenstore.FieldHash:           []byte("kadjsf;alkd"),
							exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
							exttokenstore.FieldLabels:         []byte("null"),
							exttokenstore.FieldLastUpdateTime: []byte("13:00"),
							exttokenstore.FieldPrincipalID:    []byte("local://kubeconfig-u-user5zfww"),
							exttokenstore.FieldTTL:            []byte("57600000"),
							exttokenstore.FieldUID:            []byte("2905498-kafld-lkad"),
							exttokenstore.FieldUserName:       []byte(""),
						},
					}, nil)
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantUserInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra: map[string][]string{
					"foo":            {"bar"},
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				},
			},
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate serviceaccount",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"system:serviceaccount:default:test"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(true, nil)

				return mock
			},
			wantUserInfo:          userInfo,
			wantNextHandlerCalled: true,
			status:                http.StatusOK,
		},
		{
			desc: "impersonate user not allowed",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"impUser"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate user",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate group not allowed",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":  {"impUser"},
						"Impersonate-Group": {"impGroup"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateGroup(req, "user", "impGroup").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate group",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate extras not allowed",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":      {"impUser"},
						"Impersonate-Extra-foo": {"bar"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{"foo": {"bar"}}).Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate extra",
			status:  http.StatusForbidden,
		},
		{
			desc: "user is not the owner of the request token",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().Get("kubeconfig-u-user5zfww").Return(&v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kubeconfig-u-user5zfww",
					},
					UserID: "someoneelse",
				}, nil)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				secrets.EXPECT().Cache().Return(nil)
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantErr: "request token user does not match impersonation user",
			status:  http.StatusForbidden,
		},
		{
			desc: "user is not the owner of the request token, ext token as origin",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().
					Get("kubeconfig-u-user5zfww").
					Return(nil, errors.New("unexpected error"))
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
				secrets.EXPECT().Cache().Return(scache)
				scache.EXPECT().
					Get("cattle-tokens", "kubeconfig-u-user5zfww").
					Return(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "kubeconfig-u-user5zfww",
						},
						Data: map[string][]byte{
							exttokenstore.FieldUserID: []byte("someoneelse"),
							// everything else to satisfy the ext token read checks
							exttokenstore.FieldAnnotations:    []byte("null"),
							exttokenstore.FieldAuthProvider:   []byte("local"),
							exttokenstore.FieldDisplayName:    []byte(""),
							exttokenstore.FieldEnabled:        []byte("true"),
							exttokenstore.FieldHash:           []byte("kadjsf;alkd"),
							exttokenstore.FieldKind:           []byte(exttokenstore.IsLogin),
							exttokenstore.FieldLabels:         []byte("null"),
							exttokenstore.FieldLastUpdateTime: []byte("13:00"),
							exttokenstore.FieldPrincipalID:    []byte("local://kubeconfig-u-user5zfww"),
							exttokenstore.FieldTTL:            []byte("57600000"),
							exttokenstore.FieldUID:            []byte("2905498-kafld-lkad"),
							exttokenstore.FieldUserName:       []byte(""),
						},
					}, nil)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantErr: "request token user does not match impersonation user",
			status:  http.StatusForbidden,
		},
		{
			desc: "multiple requesttokenid values",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww", "kubeconfig-u-otherxyzab"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"requesttokenid": {"kubeconfig-u-user5zfww", "kubeconfig-u-otherxyzab"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().Get(gomock.Any()).Times(0)
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				secrets.EXPECT().Cache().Return(nil)
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantErr: "multiple requesttokenid values",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate serviceaccount not allowed",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"system:serviceaccount:default:test"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate service account",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate user error",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"impUser"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate user: unexpected error",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate group error",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":  {"impUser"},
						"Impersonate-Group": {"impGroup"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateGroup(req, "user", "impGroup").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate group: unexpected error",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate extras error",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":      {"impUser"},
						"Impersonate-Extra-foo": {"bar"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{"foo": {"bar"}}).Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate extras: unexpected error",
			status:  http.StatusForbidden,
		},
		{
			desc: "validating request token error",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User":                 {"impUser"},
						"Impersonate-Extra-requesttokenid": {"kubeconfig-u-user5zfww"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{
					"requesttokenid": {"kubeconfig-u-user5zfww"},
				}).Return(true, nil)

				return mock
			},
			extTokenStore: func() *exttokenstore.SystemStore {
				// Note: Have to fail both norman and ext sides of the token fetch
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.Token](ctrl)
				cache.EXPECT().Get("kubeconfig-u-user5zfww").Return(nil, errors.New("unexpected error"))
				users := fake.NewMockNonNamespacedControllerInterface[*v3.User, *v3.UserList](ctrl)
				users.EXPECT().Cache().Return(nil)
				secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
				scache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
				secrets.EXPECT().Cache().Return(scache)
				scache.EXPECT().
					Get("cattle-tokens", "kubeconfig-u-user5zfww").
					Return(nil, errors.New("unexpected error"))
				store := exttokenstore.NewSystem(nil, secrets, users, cache, nil, nil, nil)
				return store
			},
			wantErr: "error getting request token",
			status:  http.StatusForbidden,
		},
		{
			desc: "impersonate service account error",
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{
					Header: map[string][]string{
						"Impersonate-User": {"system:serviceaccount:default:test"},
					},
				}
				req = req.WithContext(ctx)

				return req
			},
			sar: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate service account: unexpected error",
			status:  http.StatusForbidden,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			req := test.req()
			ia := &ImpersonatingAuth{sar: test.sar(req)}
			if test.extTokenStore != nil {
				ia.extTokenStore = test.extTokenStore()
			}

			mh := &mockHandler{}
			h := ia.ImpersonationMiddleware(mh)

			rw := httptest.NewRecorder()
			h.ServeHTTP(rw, req)

			require.Equal(t, test.status, rw.Code)

			if test.wantErr != "" {
				bodyBytes, err := io.ReadAll(rw.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(bodyBytes), test.wantErr)
			}

			if test.wantUserInfo != nil {
				info, _ := request.UserFrom(req.Context())
				assert.Equal(t, test.wantUserInfo, info)
			}

			assert.Equal(t, test.wantNextHandlerCalled, mh.serveHTTPWasCalled)
		})
	}
}

type mockHandler struct {
	serveHTTPWasCalled bool
}

func (m *mockHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {
	m.serveHTTPWasCalled = true
}
