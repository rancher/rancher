package requests

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAuthenticateImpersonation(t *testing.T) {
	ctrl := gomock.NewController(t)
	userInfo := &user.DefaultInfo{
		Name:   "user",
		UID:    "user",
		Groups: nil,
		Extra:  nil,
	}
	tests := map[string]struct {
		req                   func() *http.Request
		sarMock               func(req *http.Request) sar.SubjectAccessReview
		wantUserInfo          *user.DefaultInfo
		wantNextHandlerCalled bool
		wantErr               string
	}{
		"no impersonation": {
			req: func() *http.Request {
				ctx := request.WithUser(context.Background(), userInfo)
				req := &http.Request{}
				req = req.WithContext(ctx)

				return req
			},
			sarMock: func(_ *http.Request) sar.SubjectAccessReview {
				return mocks.NewMockSubjectAccessReview(ctrl)
			},
			wantNextHandlerCalled: true,
		},
		"impersonate user": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
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
		},
		"impersonate group": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
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
		},
		"impersonate extras": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{"foo": {"bar"}}).Return(true, nil)

				return mock
			},
			wantUserInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra:  map[string][]string{"foo": {"bar"}},
			},
			wantNextHandlerCalled: true,
		},
		"impersonate serviceaccount": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(true, nil)

				return mock
			},
			wantUserInfo:          userInfo,
			wantNextHandlerCalled: true,
		},
		"impersonate user not allowed": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate user",
		},
		"impersonate group not allowed": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateGroup(req, "user", "impGroup").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate group",
		},
		"impersonate extras not allowed": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{"foo": {"bar"}}).Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate extra",
		},
		"impersonate serviceaccount not allowed": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(false, nil)

				return mock
			},
			wantErr: "not allowed to impersonate service account",
		},
		"impersonate user error": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate user: unexpected error",
		},
		"impersonate group error": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateGroup(req, "user", "impGroup").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate group: unexpected error",
		},
		"impersonate extras error": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateUser(req, "user", "impUser").Return(true, nil)
				mock.EXPECT().UserCanImpersonateExtras(req, "user", map[string][]string{"foo": {"bar"}}).Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate extras: unexpected error",
		},
		"impersonate service account error": {
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
			sarMock: func(req *http.Request) sar.SubjectAccessReview {
				mock := mocks.NewMockSubjectAccessReview(ctrl)
				mock.EXPECT().UserCanImpersonateServiceAccount(req, "user", "system:serviceaccount:default:test").Return(false, errors.New("unexpected error"))

				return mock
			},
			wantErr: "error checking if user can impersonate service account: unexpected error",
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := test.req()
			ia := NewImpersonatingAuth(test.sarMock(req))
			mh := &mockHandler{}
			rw := httptest.NewRecorder()
			h := ia.ImpersonationMiddleware(mh)

			h.ServeHTTP(rw, req)

			assert.Equal(t, test.wantNextHandlerCalled, mh.serveHTTPWasCalled)
			if test.wantErr != "" {
				bodyBytes, err := io.ReadAll(rw.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(bodyBytes), test.wantErr)
			}
			if test.wantUserInfo != nil {
				info, _ := request.UserFrom(req.Context())
				assert.Equal(t, test.wantUserInfo, info)
			} else {
				info, _ := request.UserFrom(req.Context())
				assert.Equal(t, userInfo, info)
			}
		})
	}
}

type mockHandler struct {
	serveHTTPWasCalled bool
}

func (m *mockHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {
	m.serveHTTPWasCalled = true
}
