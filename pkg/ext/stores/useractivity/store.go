package useractivity

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3Legacy "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	SingularName             = "useractivity"
	PluralName               = "useractivities"
	GroupCattleAuthenticated = "system:cattle:authenticated"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type Store struct {
	tokens     v3.TokenController
	tokenCache v3.TokenCache
	userCache  v3.UserCache
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
		tokens:     wranglerCtx.Mgmt.Token(),
		tokenCache: wranglerCtx.Mgmt.Token().Cache(),
		userCache:  wranglerCtx.Mgmt.User().Cache(),
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
	userInfo, err := s.userFrom(ctx)
	if err != nil {
		return nil, err
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

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
	if objUserActivity.Name == "" {
		return nil, apierrors.NewBadRequest("can't retrieve token with empty string")
	}

	authToken, err := s.tokenCache.Get(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	activityToken, err := s.tokenCache.Get(objUserActivity.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", objUserActivity.Name, err))
		} else {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", objUserActivity.Name, err))
		}
	}

	if authToken.AuthProvider != activityToken.AuthProvider {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("request token %s and activity token %s have different auth providers", authTokenID, objUserActivity.Name))
	}

	if authToken.UserPrincipal.Name != activityToken.UserPrincipal.Name {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("request token %s and activity token %s have different user principals", authTokenID, objUserActivity.Name))
	}

	// set when last activity happened
	lastActivity := metav1.Time{
		Time: time.Now().UTC(),
	}
	// retrieve setting for auth-user-session-idle-ttl-minutes
	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()

	// check if it's a dry-run
	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	return s.create(ctx, objUserActivity, activityToken, lastActivity, idleTimeout, dryRun)
}

// create sets the LastActivity and CurrentTimeout fields on the UserActivity object
// provided by the user within the request.
func (s *Store) create(_ context.Context,
	userActivity *ext.UserActivity,
	token *v3Legacy.Token,
	lastActivity metav1.Time,
	authUserSessionIdleTTLMinutes int,
	dryRun bool) (*ext.UserActivity, error) {

	// ensure
	if userActivity.GenerateName != "" {
		return nil, apierrors.NewBadRequest("name generation is not allowed")
	}
	// ensure the token specified in the UserActivity is the same
	// we are using to do the request.
	if token.Name != userActivity.Name {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("token name mismatch: have %s - expected %s", token.Name, userActivity.Name))
	}

	// once validated the request, we can define the lastActivity time.
	newIdleTimeout := metav1.Time{
		Time: lastActivity.Add(time.Minute * time.Duration(authUserSessionIdleTTLMinutes)).UTC(),
	}
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
		_, err = s.tokens.Patch(token.GetName(), types.JSONPatchType, patch)
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
	return s.get(ctx, name)
}

// get returns the UserActivity based on the token name.
// It is used to know, from the frontend, how much time
// remains before the idle timeout triggers.
func (s *Store) get(ctx context.Context, uaname string) (runtime.Object, error) {
	userInfo, err := s.userFrom(ctx)
	if err != nil {
		return nil, err
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

	authToken, err := s.tokenCache.Get(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	activityToken, err := s.tokenCache.Get(uaname)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", uaname, err))
		} else {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", uaname, err))
		}
	}

	if authToken.AuthProvider != activityToken.AuthProvider {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("request token %s and activity token %s have different auth providers", authTokenID, uaname))
	}

	if authToken.UserPrincipal.Name != activityToken.UserPrincipal.Name {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("request token %s and activity token %s have different user principals", authTokenID, uaname))
	}

	// crafting UserActivity from requested Token name.
	ua := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: uaname,
		},
		Status: ext.UserActivityStatus{},
	}

	if activityToken.ActivityLastSeenAt != nil {
		ua.Status.ExpiresAt = activityToken.ActivityLastSeenAt.String()
	} else {
		ua.Status.ExpiresAt = metav1.Time{}.String()
	}

	return ua, nil
}

// userFrom is a helper that extracts and validates the user info from the request's context.
func (s *Store) userFrom(ctx context.Context) (k8suser.Info, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("missing user info"))
	}

	userName := userInfo.GetName()

	if strings.Contains(userName, ":") { // E.g. system:admin
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	if !slices.Contains(userInfo.GetGroups(), GroupCattleAuthenticated) {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	user, err := s.userCache.Get(userName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s not found", userName))
		}

		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user %s: %w", userName, err))
	}

	if user.Enabled != nil && !*user.Enabled {
		return nil, apierrors.NewForbidden(GVR.GroupResource(), "", fmt.Errorf("user %s is disabled", userName))
	}

	return userInfo, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
