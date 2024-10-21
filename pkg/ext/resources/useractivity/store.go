package useractivity

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/ext/resources/types"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type UserActivityStore struct {
	tokenController v3.TokenController
	configMapClient v1.ConfigMapClient
}

const UserActivityNamespace = "cattle-useractivity-data"
const tokenUserId = "authn.management.cattle.io/token-userId"

func NewUserActivityStore(token v3.TokenController, cmclient v1.ConfigMapClient) types.Store[*UserActivity, *UserActivityList] {
	return &UserActivityStore{
		tokenController: token,
		configMapClient: cmclient,
	}
}

func (uas *UserActivityStore) Create(ctx context.Context, userInfo user.Info, useractivity *UserActivity, opts *metav1.CreateOptions) (*UserActivity, error) {
	token, err := uas.tokenController.Get(useractivity.Spec.TokenId, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", useractivity.Spec.TokenId)
	}

	// verifies the token has label with user which made the request.
	if token.Labels[tokenUserId] == userInfo.GetName() {
		// once validated the request, we can define the lastActivity time.
		lastActivity := time.Now()
		// TODO: replace '10' with the value of auth-user-session-ttl-minutes
		newIdleTimeout := lastActivity.Local().Add(time.Minute * time.Duration(10))

		token.CurrentIdleTimeout = newIdleTimeout
		uas.tokenController.Update(token)

		useractivity.Status.LastActivity = lastActivity.String()
		useractivity.Status.CurrentTimeout = newIdleTimeout.String()

		return useractivity, nil
	}

	return nil, fmt.Errorf("unable to create useractivity")
}

// Leave empty.
func (uas *UserActivityStore) Update(ctx context.Context, userInfo user.Info, useractivity *UserActivity, opts *metav1.UpdateOptions) (*UserActivity, error) {
	return nil, fmt.Errorf("unable to update useractivity")
}

func (uas *UserActivityStore) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (*UserActivity, error) {
	return nil, fmt.Errorf("unable to get useractivity")
}

func (uas *UserActivityStore) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (*UserActivityList, error) {
	return nil, fmt.Errorf("unable to list useractivity")
}

func (uas *UserActivityStore) Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan types.WatchEvent[*UserActivity], error) {
	return nil, fmt.Errorf("unable to watch useractivity")
}

// Leave empty.
func (uas *UserActivityStore) Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error {
	return fmt.Errorf("unable to delete useractivity")
}

func configMapFromUserActivity(ua *UserActivity) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   UserActivityNamespace,
			Name:        ua.Spec.TokenId,
			Labels:      ua.Labels,
			Annotations: ua.Annotations,
		},
		Data: make(map[string]string),
	}
	cm.Data["currentTimeout"] = ua.Status.CurrentTimeout
	cm.Data["lastActivity"] = ua.Status.LastActivity
	return cm
}
