package useractivity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3Legacy "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	UserActivityNamespace = "cattle-useractivity-data"
	tokenUserId           = "authn.management.cattle.io/token-userId"
	SingularName          = "useractivity"
	PluralName            = "useractivities"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type Store struct {
	tokenController v3.TokenController
}

var GV = schema.GroupVersion{
	Group:   "ext.cattle.io",
	Version: "v1",
}

var GVK = schema.GroupVersionKind{
	Group:   GV.Group,
	Version: GV.Version,
	Kind:    "UserActivity",
}
var GVR = schema.GroupVersionResource{
	Group:    GV.Group,
	Version:  GV.Version,
	Resource: PluralName,
}

func New(wranglerCtx *wrangler.Context) *Store {
	return &Store{
		tokenController: wranglerCtx.Mgmt.Token(),
	}
}

// GroupVersionKind implements [rest.GroupVersionKindProvider]
func (s *Store) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return GVK
}

// NamespaceScoped implements [rest.Scoper]
func (s *Store) NamespaceScoped() bool {
	return false
}

// GetSingularName implements [rest.SingularNameProvider]
func (s *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (s *Store) New() runtime.Object {
	obj := &ext.UserActivity{}
	obj.GetObjectKind().SetGroupVersionKind(GVK)
	return obj
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {
}

// Create implements [rest.Creator]
func (s *Store) Create(ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		err := createValidation(ctx, obj)
		if err != nil {
			return obj, err
		}
	}

	// retrieving useractivity object from raw data
	objUserActivity, ok := obj.(*ext.UserActivity)
	if !ok {
		var zeroUA *ext.UserActivity
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T", zeroUA, objUserActivity))
	}
	// retrieve token information
	if objUserActivity.Spec.TokenID == "" {
		return nil, apierrors.NewBadRequest("can't retrieve token with empty string")
	}
	token, err := s.tokenController.Get(objUserActivity.Spec.TokenID, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", objUserActivity.Spec.TokenID, err))
		} else {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", objUserActivity.Spec.TokenID, err))
		}
	}
	// set when last activity happened
	lastActivity := metav1.Time{
		Time: time.Now().UTC(),
	}
	// retrieve setting for auth-user-session-idle-ttl-minutes
	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()

	// check if it's a dry-run
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	return s.create(ctx, objUserActivity, token, token.UserID, lastActivity, idleTimeout, dryRun)
}

// create sets the LastActivity and CurrentTimeout fields on the UserActivity object
// provided by the user within the request.
func (s *Store) create(_ context.Context,
	userActivity *ext.UserActivity,
	token *v3Legacy.Token,
	user string,
	lastActivity metav1.Time,
	authUserSessionIdleTTLMinutes int,
	dryRun bool) (*ext.UserActivity, error) {

	expectedName, err := setUserActivityName(user, token.Name)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to set useractivity name: %w", err))
	}
	// ensure the UserActivity object is crafted as expected.
	if userActivity.Name != expectedName {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("useractivity name mismatch: have %s - expected %s", userActivity.Name, expectedName))
	}
	// ensure the token specified in the UserActivity is the same
	// we are using to do the request.
	if token.Name != userActivity.Spec.TokenID {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("token name mismatch: have %s - expected %s", token.Name, userActivity.Spec.TokenID))
	}

	// once validated the request, we can define the lastActivity time.
	newIdleTimeout := metav1.Time{
		Time: lastActivity.Add(time.Minute * time.Duration(authUserSessionIdleTTLMinutes)).UTC(),
	}
	userActivity.Status.LastSeetAt = lastActivity.String()
	userActivity.Status.ExpiresAt = newIdleTimeout.String()

	// if it's not a dry-run, commit the changes
	if !dryRun {
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "replace",
			Path:  "/activityLastSeenAt",
			Value: newIdleTimeout,
		}})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("%w", err))
		}
		_, err = s.tokenController.Patch(token.GetName(), types.JSONPatchType, patch)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to patch token: %w", err))
		}
	}

	return userActivity, nil
}

// Get implements [rest.Getter]
func (s *Store) Get(ctx context.Context,
	name string,
	options *metav1.GetOptions) (runtime.Object, error) {
	return s.get(ctx, name, options)
}

// get returns the UserActivity based on the token name.
// It is used to know, from the frontend, how much time
// remains before the idle timeout triggers.
func (s *Store) get(_ context.Context, uaname string, options *metav1.GetOptions) (runtime.Object, error) {
	user, token, err := getUserActivityName(uaname)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("wrong useractivity name: %v", err))
	}
	// retrieve token information
	tokenId, err := s.tokenController.Get(token, *options)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", token, err))
	}
	// verify user is the same
	if tokenId.UserID != user {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("user provided mismatch: have %s - expected %s", user, tokenId.UserID))
	}

	// crafting UserActivity from requested Token name.
	ua := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: uaname,
		},
		Spec: ext.UserActivitySpec{
			TokenID: tokenId.Name,
		},
		Status: ext.UserActivityStatus{},
	}

	if tokenId.ActivityLastSeenAt != nil {
		ua.Status.ExpiresAt = tokenId.ActivityLastSeenAt.String()
	} else {
		ua.Status.ExpiresAt = metav1.Time{}.String()
	}

	return ua, nil
}
