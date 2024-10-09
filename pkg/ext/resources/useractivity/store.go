package useractivity

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/ext/resources/types"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type UserActivityStore struct {
	tokenController v3.TokenController
}

const tokenUserId = "authn.management.cattle.io/token-userId"

func NewUserActivityStore(token v3.TokenController) types.Store[*UserActivity, *UserActivityList] {
	return &UserActivityStore{
		tokenController: token,
	}
}

func (uas *UserActivityStore) Create(ctx context.Context, userInfo user.Info, useractivity *UserActivity, opts *metav1.CreateOptions) (*UserActivity, error) {
	token, err := uas.tokenController.Get(useractivity.Spec.TokenId, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", useractivity.Spec.TokenId)
	}

	// checks if token has label with user which made the request
	if token.Labels[tokenUserId] == userInfo.GetName() {
		// once validated the request, we can define the lastActivity time.
		lastActivity := time.Now()
		useractivity.Status.LastActivity = lastActivity.String()
		// TODO: replace 10 with the value of auth-user-session-ttl-minutes
		useractivity.Status.CurrentTimeout = lastActivity.Local().Add(time.Minute * time.Duration(10)).String()
		return useractivity, nil
	}
	//if useractivity.Spec.TokenId
	return nil, fmt.Errorf("unable to create UserActivity")
}

// Leave empty.
func (uas *UserActivityStore) Update(ctx context.Context, userInfo user.Info, useractivity *UserActivity, opts *metav1.UpdateOptions) (*UserActivity, error) {
	return nil, fmt.Errorf("unable to update UserActivity")
}

func (uas *UserActivityStore) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (*UserActivity, error) {
	return nil, fmt.Errorf("unable to get UserActivity")
}

func (uas *UserActivityStore) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (*UserActivityList, error) {
	return nil, fmt.Errorf("unable to list UserActivity")
}

func (uas *UserActivityStore) Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan types.WatchEvent[*UserActivity], error) {
	return nil, fmt.Errorf("unable to watch UserActivity")
}

// Leave empty.
func (uas *UserActivityStore) Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error {
	return fmt.Errorf("unable to delete UserActivity")
}
