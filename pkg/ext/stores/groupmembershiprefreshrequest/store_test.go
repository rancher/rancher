package groupmembershiprefreshrequest

import (
	"context"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestCreate(t *testing.T) {
	fakeUserId := "fake-user-id"
	tests := map[string]struct {
		ctx                     context.Context
		obj                     runtime.Object
		options                 *metav1.CreateOptions
		authorizer              authorizer.Authorizer
		assertUserAuthRefresher func(*testing.T, *fakeUserAuthRefresher)
		wantObj                 *ext.GroupMembershipRefreshRequest
		wantErr                 string
	}{
		"all user refreshed": {
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserId}),
			obj: &ext.GroupMembershipRefreshRequest{
				Spec: ext.GroupMembershipRefreshRequestSpec{
					UserID: allUsers,
				},
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			assertUserAuthRefresher: func(t *testing.T, refresher *fakeUserAuthRefresher) {
				assert.True(t, refresher.TriggerAllUserRefreshCalled)
				assert.Nil(t, refresher.TriggerUserRefreshCalledWith)
			},
			wantObj: &ext.GroupMembershipRefreshRequest{
				Spec: ext.GroupMembershipRefreshRequestSpec{
					UserID: allUsers,
				},
				Status: ext.GroupMembershipRefreshRequestStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "UserRefreshInitiated",
							Status: "True",
						},
					},
					Summary: status.SummaryCompleted,
				},
			},
		},
		"single user refreshed": {
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserId}),
			obj: &ext.GroupMembershipRefreshRequest{
				Spec: ext.GroupMembershipRefreshRequestSpec{
					UserID: fakeUserId,
				},
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			assertUserAuthRefresher: func(t *testing.T, refresher *fakeUserAuthRefresher) {
				assert.False(t, refresher.TriggerAllUserRefreshCalled)
				assert.Equal(t, refresher.TriggerUserRefreshCalledWith, []struct {
					UserID string
					Force  bool
				}{{UserID: fakeUserId, Force: true}})
			},
			wantObj: &ext.GroupMembershipRefreshRequest{
				Spec: ext.GroupMembershipRefreshRequestSpec{
					UserID: fakeUserId,
				},
				Status: ext.GroupMembershipRefreshRequestStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "UserRefreshInitiated",
							Status: "True",
						},
					},
					Summary: status.SummaryCompleted,
				},
			},
		},
		"invalid request": {
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserId}),
			obj: &ext.GroupMembershipRefreshRequest{},
			assertUserAuthRefresher: func(t *testing.T, refresher *fakeUserAuthRefresher) {
				assert.False(t, refresher.TriggerAllUserRefreshCalled)
				assert.Nil(t, refresher.TriggerUserRefreshCalledWith)
			},
			wantErr: "user ID must be set",
		},
		"not authorized": {
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserId}),
			obj: &ext.GroupMembershipRefreshRequest{
				Spec: ext.GroupMembershipRefreshRequestSpec{
					UserID: allUsers,
				},
			},
			authorizer: authorizer.AuthorizerFunc(func(ctx context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionDeny, "", nil
			}),
			assertUserAuthRefresher: func(t *testing.T, refresher *fakeUserAuthRefresher) {
				assert.False(t, refresher.TriggerAllUserRefreshCalled)
				assert.Nil(t, refresher.TriggerUserRefreshCalledWith)
			},
			wantErr: "not authorized to refresh user attributes",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			userRefresher := &fakeUserAuthRefresher{}
			store := Store{
				authorizer:        test.authorizer,
				userAuthRefresher: userRefresher,
			}

			obj, err := store.Create(test.ctx, test.obj, nil, test.options)

			if test.assertUserAuthRefresher != nil {
				test.assertUserAuthRefresher(t, userRefresher)
			}
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
				req, ok := obj.(*ext.GroupMembershipRefreshRequest)
				assert.True(t, ok, "expected object to be of type GroupMembershipRefreshRequest")

				assert.Equal(t, test.wantObj.Spec.UserID, req.Spec.UserID)
				assert.Equal(t, test.wantObj.Status.Summary, req.Status.Summary)
				require.Len(t, req.Status.Conditions, 1)
				assert.Equal(t, test.wantObj.Status.Conditions[0].Type, req.Status.Conditions[0].Type)
				assert.Equal(t, test.wantObj.Status.Conditions[0].Status, req.Status.Conditions[0].Status)
				assert.Equal(t, test.wantObj.Status.Conditions[0].Message, req.Status.Conditions[0].Message)
				assert.Equal(t, test.wantObj.Status.Conditions[0].Reason, req.Status.Conditions[0].Reason)
				assert.NotEmpty(t, req.Status.Conditions[0].LastTransitionTime)
			}
		})
	}
}

// fakeUserAuthRefresher is a fake implementation of UserAuthRefresher.
type fakeUserAuthRefresher struct {
	// TriggerAllUserRefreshCalled tracks if the method was called.
	TriggerAllUserRefreshCalled bool
	// TriggerUserRefreshCalledWith holds the arguments for each call.
	TriggerUserRefreshCalledWith []struct {
		UserID string
		Force  bool
	}
}

// TriggerAllUserRefresh implements the UserAuthRefresher interface.
func (f *fakeUserAuthRefresher) TriggerAllUserRefresh() {
	f.TriggerAllUserRefreshCalled = true
}

// TriggerUserRefresh implements the UserAuthRefresher interface.
func (f *fakeUserAuthRefresher) TriggerUserRefresh(userID string, force bool) {
	f.TriggerUserRefreshCalledWith = append(f.TriggerUserRefreshCalledWith, struct {
		UserID string
		Force  bool
	}{UserID: userID, Force: force})
}
