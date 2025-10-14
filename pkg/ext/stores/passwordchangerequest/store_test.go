package passwordchangerequest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/ext/mocks"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	userID := "fake-user-id"
	username := "fake-username"
	oldPassword := "fake-current-password"
	newPassword := "fake-new-password"

	userCache := func() mgmtv3.UserCache {
		cache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
		cache.EXPECT().Get(gomock.Any()).Return(&v3.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: userID,
			},
			Username: username,
		}, nil).AnyTimes()
		return cache
	}
	pwdUpdater := func() PasswordUpdater {
		return mocks.NewMockPasswordUpdater(ctrl)
	}

	tests := []struct {
		desc       string
		obj        *ext.PasswordChangeRequest
		ctx        context.Context
		options    *metav1.CreateOptions
		authorizer authorizer.Authorizer
		pwdUpdater func() PasswordUpdater
		userCache  func() mgmtv3.UserCache
		wantObj    *ext.PasswordChangeRequest
		wantErr    string
	}{
		{
			desc: "password changed for the same user",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: userID}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctrl)
				mock.EXPECT().VerifyAndUpdatePassword(userID, oldPassword, newPassword).Return(nil)

				return mock
			},
			userCache: userCache,
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
				Status: ext.PasswordChangeRequestStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "PasswordUpdated",
							Status: "True",
						},
					},
					Summary: status.SummaryCompleted,
				},
			},
		},
		{
			desc: "password changed for a different user",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctrl)
				mock.EXPECT().UpdatePassword(userID, newPassword).Return(nil)

				return mock
			},
			userCache: userCache,
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
				Status: ext.PasswordChangeRequestStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "PasswordUpdated",
							Status: "True",
						},
					},
					Summary: status.SummaryCompleted,
				},
			},
		},
		{
			desc: "password is not changed for a different user without enough permissions",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: pwdUpdater,
			userCache:  userCache,
			wantErr:    "not authorized to update password",
		},
		{
			desc: "password too short",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     "short",
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: pwdUpdater,
			wantErr:    "password must be at least 12 characters",
		},
		{
			desc: "password matches username",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     username,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: pwdUpdater,
			userCache:  userCache,
			wantErr:    "password cannot be the same as the username",
		},
		{
			desc: "user not found",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: pwdUpdater,
			userCache: func() mgmtv3.UserCache {
				cache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
				cache.EXPECT().Get(gomock.Any()).Return(nil, apierrors.NewNotFound(v3.Resource("user"), ""))
				return cache
			},
			wantErr: fmt.Sprintf("user %s not found", userID),
		},
		{
			desc: "dry run",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			options: &metav1.CreateOptions{
				DryRun: []string{metav1.DryRunAll},
			},
			pwdUpdater: pwdUpdater,
			userCache:  userCache,
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: userID}),
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
		},
		{
			desc: "error updating password",
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          userID,
					CurrentPassword: oldPassword,
					NewPassword:     newPassword,
				},
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: userID}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctrl)
				mock.EXPECT().UpdatePassword(userID, newPassword).Return(errors.New("unexpected error"))

				return mock
			},
			userCache: userCache,
			wantErr:   "unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			store := Store{
				authorizer:           tt.authorizer,
				getPasswordMinLength: func() int { return 12 },
			}
			if tt.pwdUpdater != nil {
				store.pwdUpdater = tt.pwdUpdater()
			}
			if tt.userCache != nil {
				store.userCache = tt.userCache()
			}

			obj, err := store.Create(tt.ctx, tt.obj, nil, tt.options)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantObj, obj)
			}
		})
	}
}
