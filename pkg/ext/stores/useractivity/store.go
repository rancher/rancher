package useractivity

import (
	"context"
	"fmt"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	extcore "github.com/rancher/steve/pkg/ext"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	UserActivityNamespace = "cattle-useractivity-data"
	tokenUserId           = "authn.management.cattle.io/token-userId"
	SingularName          = "useractivity"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type Store struct {
	tokenController v3.TokenController
	configMapClient v1.ConfigMapClient
	checker         userHandler
}

var GV = schema.GroupVersion{
	Group:   "ext.cattle.io",
	Version: "v1",
}

var GVK = schema.GroupVersionKind{
	Group:   GV.Group,
	Version: GV.Version,
	Kind:    SingularName,
}
var GVR = schema.GroupVersionResource{
	Group:    GV.Group,
	Version:  GV.Version,
	Resource: SingularName,
}

func NewUserActivityStore(token v3.TokenController, cmclient v1.ConfigMapClient) *Store {
	return &Store{
		tokenController: token,
		configMapClient: cmclient,
		checker:         &tokenChecker{},
	}
}

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (t *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper]
func (t *Store) NamespaceScoped() bool {
	return false
}

// GetSingularName implements [rest.SingularNameProvider]
func (t *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (t *Store) New() runtime.Object {
	obj := &ext.UserActivity{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage]
func (t *Store) Destroy() {
}

// Create implements [rest.Creator]
func (uas *Store) Create(ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions) (runtime.Object, error) {

	// retrieving useractivity object from raw data
	objUserActivity, ok := obj.(*ext.UserActivity)
	if !ok {

	}

	token, err := uas.tokenController.Get(objUserActivity.Spec.TokenId, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", objUserActivity.Spec.TokenId)
	}

	user, err := uas.checker.UserName(ctx)
	if err != nil {
		return nil, fmt.Errorf("no user found: %v", err)
	}
	// verifies the token has label with user which made the request.
	if token.Labels[tokenUserId] == user {
		// once validated the request, we can define the lastActivity time.
		lastActivity := time.Now()
		// TODO: replace '10' with the value of auth-user-session-ttl-minutes
		newIdleTimeout := lastActivity.Local().Add(time.Minute * time.Duration(10))

		token.LastIdleTimeout = newIdleTimeout
		uas.tokenController.Update(token)

		objUserActivity.Status.LastActivity = lastActivity.String()
		objUserActivity.Status.CurrentTimeout = newIdleTimeout.String()

		return objUserActivity, nil
	}

	return nil, fmt.Errorf("unable to create useractivity")
}

// The rest of the methods will be left empty.

// Delete implements [rest.GracefulDeleter]
func (uas *Store) Delete(ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("unable to delete useractivity")
}

// Get implements [rest.Getter]
func (uas *Store) Get(ctx context.Context,
	name string,
	options *metav1.GetOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("unable to get useractivity")
}

// NewList implements [rest.Lister]
func (t *Store) NewList() runtime.Object {
	objList := &ext.UserActivityList{}
	objList.GetObjectKind().SetGroupVersionKind(GVK)
	return objList
}

// List implements [rest.Lister]
func (uas *Store) List(ctx context.Context,
	internaloptions *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("unable to list useractivity")
}

// ConvertToTable implements [rest.Lister]
func (t *Store) ConvertToTable(
	ctx context.Context,
	object runtime.Object,
	tableOptions runtime.Object) (*metav1.Table, error) {

	return extcore.ConvertToTableDefault[*ext.UserActivity](ctx, object, tableOptions,
		GVR.GroupResource())
}

// Update implements [rest.Updater]
func (uas *Store) Update(ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, fmt.Errorf("unable to update useractivity")
}

// Watch implements [rest.Watcher]
func (uas *Store) Watch(ctx context.Context, internaloptions *metainternalversion.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("unable to watch useractivity")
}

// userHandler is an interface hiding the details of retrieving the user name
// from the store. This makes these operations mockable for store testing.
type userHandler interface {
	UserName(ctx context.Context) (string, error)
}

type tokenChecker struct{}

// UserName hides the details of extracting a user name from the request context
// TODO: move under dedicated package once Andrea's PR is merged, since both PRs implement the same methods.
// (https://github.com/rancher/rancher/pull/47643/files#top)
func (tp *tokenChecker) UserName(ctx context.Context) (string, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return "", fmt.Errorf("context has no user info")
	}

	return userInfo.GetName(), nil
}
