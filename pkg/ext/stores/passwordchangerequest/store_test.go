package passwordchangerequest

import (
	"context"
	"errors"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/ext/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestCreate(t *testing.T) {
	ctlr := gomock.NewController(t)
	fakeUserID := "fake-user-id"
	fakeCurrentPassword := "fake-current-password"
	fakeNewPassword := "fake-new-password"

	tests := map[string]struct {
		obj        *ext.PasswordChangeRequest
		ctx        context.Context
		options    *metav1.CreateOptions
		authorizer authorizer.Authorizer
		pwdUpdater func() PasswordUpdater
		wantObj    *ext.PasswordChangeRequest
		wantErr    string
	}{
		"password changed for the same user": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserID}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctlr)
				mock.EXPECT().VerifyAndUpdatePassword(fakeUserID, fakeCurrentPassword, fakeNewPassword).Return(nil)

				return mock
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
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
		"password changed for a different user": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctlr)
				mock.EXPECT().UpdatePassword(fakeUserID, fakeNewPassword).Return(nil)

				return mock
			},
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
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
		"password is not changed for a different user without enough permissions": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: func() PasswordUpdater {
				return mocks.NewMockPasswordUpdater(ctlr)
			},
			wantErr: "not authorized to update password",
		},
		"password too short": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     "short",
				},
			},
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: "another-user"}),
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			pwdUpdater: func() PasswordUpdater {
				return mocks.NewMockPasswordUpdater(ctlr)
			},
			wantErr: "error validating password: password must be at least 12 characters",
		},
		"dry run": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
			options: &metav1.CreateOptions{
				DryRun: []string{metav1.DryRunAll},
			},
			pwdUpdater: func() PasswordUpdater {
				return mocks.NewMockPasswordUpdater(ctlr)
			},

			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserID}),
			wantObj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
		},
		"error updating password": {
			obj: &ext.PasswordChangeRequest{
				Spec: ext.PasswordChangeRequestSpec{
					UserID:          fakeUserID,
					CurrentPassword: fakeCurrentPassword,
					NewPassword:     fakeNewPassword,
				},
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserID}),
			pwdUpdater: func() PasswordUpdater {
				mock := mocks.NewMockPasswordUpdater(ctlr)
				mock.EXPECT().UpdatePassword(fakeUserID, fakeNewPassword).Return(errors.New("unexpected error"))

				return mock
			},
			wantErr: "unexpected error",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := Store{
				authorizer: test.authorizer,
				pwdUpdater: test.pwdUpdater(),
			}

			obj, err := store.Create(test.ctx, test.obj, nil, test.options)

			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.wantObj, obj)
			}
		})
	}
}
