package requests

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rancher/rancher/pkg/auth/requests/mocks"
	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/stretchr/testify/assert"
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
		req            func() *http.Request
		sarMock        func(req *http.Request) sar.SubjectAccessReview
		expectedInfo   user.Info
		expectedAuthed bool
		expectedErr    string
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
			expectedInfo:   userInfo,
			expectedAuthed: true,
			expectedErr:    "",
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
			expectedInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra:  nil,
			},
			expectedAuthed: true,
			expectedErr:    "",
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
			expectedInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"impGroup", "system:authenticated"},
				Extra:  nil,
			},
			expectedAuthed: true,
			expectedErr:    "",
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
			expectedInfo: &user.DefaultInfo{
				Name:   "impUser",
				UID:    "impUser",
				Groups: []string{"system:authenticated"},
				Extra:  map[string][]string{"foo": {"bar"}},
			},
			expectedAuthed: true,
			expectedErr:    "",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "not allowed to impersonate user",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "not allowed to impersonate group",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "not allowed to impersonate extra",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "unexpected error",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "unexpected error",
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
			expectedInfo:   nil,
			expectedAuthed: false,
			expectedErr:    "unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := test.req()
			ia := NewImpersonatingAuth(test.sarMock(req))
			info, authed, err := ia.Authenticate(req)

			if test.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErr)
			}
			assert.Equal(t, test.expectedAuthed, authed)
			assert.Equal(t, test.expectedInfo, info)
		})
	}
}
