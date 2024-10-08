package useractivity

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/ext/resources/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type UserActivityStore struct {
}

func NewUserActivityStore() types.Store[*UserActivity, *UserActivityList] {
	return &UserActivityStore{}
}

func (uas *UserActivityStore) Create(ctx context.Context, userInfo user.Info, useractiviry *UserActivity, opts *metav1.CreateOptions) (*UserActivity, error) {
	return nil, fmt.Errorf("unable to create UserActivity")
}

func (uas *UserActivityStore) Update(ctx context.Context, userInfo user.Info, useractiviry *UserActivity, opts *metav1.UpdateOptions) (*UserActivity, error) {
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

func (uas *UserActivityStore) Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error {
	return fmt.Errorf("unable to delete UserActivity")
}
