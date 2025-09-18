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
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
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
	Kind                     = "UserActivity"
	SingularName             = "useractivity"
	GroupCattleAuthenticated = "system:cattle:authenticated"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type Store struct {
	tokens                           v3.TokenClient
	userCache                        v3.UserCache
	extTokenStore                    *exttokenstore.SystemStore
	getAuthUserSessionIdleTTLMinutes func() int
}

var (
	GVK     = ext.SchemeGroupVersion.WithKind(Kind)
	gvr     = ext.SchemeGroupVersion.WithResource(ext.UserActivityResourceName)
	timeNow = func() time.Time { return time.Now() }
)

func New(wranglerCtx *wrangler.Context) *Store {
	return &Store{
		tokens:                           wranglerCtx.Mgmt.Token(),
		userCache:                        wranglerCtx.Mgmt.User().Cache(),
		extTokenStore:                    exttokenstore.NewSystemFromWrangler(wranglerCtx),
		getAuthUserSessionIdleTTLMinutes: settings.AuthUserSessionIdleTTLMinutes.GetInt,
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
	return &ext.UserActivity{}
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {
}

// Create implements [rest.Creator]
// Sets the activityLastSeenAt timestamp to the value of spec.seenAt
// on the session token with the same name as the provided UserActivity's name
// and returns the expiration timestamp in status.expiresAt.
// If the spec.seenAt is not provided, the current time is used.
// If the spec.seenAt is less than the stored activityLastSeenAt, the stored value is not updated.
// The spec.seenAt value cannot be in the future.
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
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

	if createValidation != nil {
		err := createValidation(ctx, obj)
		if err != nil {
			return obj, err
		}
	}

	userActivity, ok := obj.(*ext.UserActivity)
	if !ok {
		var zeroUA *ext.UserActivity
		return nil, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T", zeroUA, userActivity))
	}

	if userActivity.Name == "" {
		return nil, apierrors.NewBadRequest("name is required")
	}

	if userActivity.GenerateName != "" {
		return nil, apierrors.NewBadRequest("name generation is not allowed")
	}

	// Retrieve the auth token.
	authToken, err := s.extTokenStore.Fetch(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	// Retrieve the activity token.
	activityToken, err := s.extTokenStore.Fetch(userActivity.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", userActivity.Name, err))
		} else {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", userActivity.Name, err))
		}
	}

	if err = validateActivityToken(authToken, activityToken); err != nil {
		return nil, err
	}

	// Retrieve auth-user-session-idle-ttl-minutes setting value.
	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()

	seen := timeNow()
	if userActivity.Spec.SeenAt != nil {
		if userActivity.Spec.SeenAt.After(seen) {
			// Make sure the idle timeout can't be bypassed by providing a future timestamp.
			return nil, apierrors.NewBadRequest("seenAt can't be in the future")
		}
		seen = userActivity.Spec.SeenAt.Time
	}

	shouldUpdate := true
	lastSeen := activityToken.GetLastActivitySeen()
	if lastSeen != nil && seen.Before(lastSeen.Time) {
		// If the SeenAt provided is before the last activity we have recorded,
		// we don't update the last activity time.
		seen = lastSeen.Time
		shouldUpdate = false
	}

	expiresAt := seen.Add(time.Minute * time.Duration(idleTimeout)).UTC()

	userActivity.Status.ExpiresAt = expiresAt.Format(time.RFC3339)
	userActivity.CreationTimestamp = activityToken.GetCreationTime()

	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	if dryRun || !shouldUpdate {
		return userActivity, nil
	}

	switch activityToken.(type) {
	case *v3Legacy.Token:
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "replace",
			Path:  "/activityLastSeenAt",
			Value: seen.UTC().Format(time.RFC3339),
		}})
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to marshall patch data: %w", err))
		}
		_, err = s.tokens.Patch(activityToken.GetName(), types.JSONPatchType, patch)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to store activityLastSeenAt to token %s: %w", activityToken.GetName(), err))
		}
	case *ext.Token:
		err := s.extTokenStore.UpdateLastActivitySeen(activityToken.GetName(), seen)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to store activityLastSeenAt to ext token %s: %w", activityToken.GetName(), err))
		}
	}

	return userActivity, nil
}

// Get implements [rest.Getter]
// Get returns the UserActivity based on the token name.
// It is used to know, from the frontend, how much time
// remains before the idle timeout triggers.
func (s *Store) Get(ctx context.Context,
	name string,
	options *metav1.GetOptions) (runtime.Object, error) {
	userInfo, err := s.userFrom(ctx)
	if err != nil {
		return nil, err
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("missing request token ID"))
	}

	// Retrieve the auth token.
	authToken, err := s.extTokenStore.Fetch(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	// Retrieve the activity token.
	activityToken, err := s.extTokenStore.Fetch(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", name, err))
		} else {
			return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", name, err))
		}
	}

	// validate activity token
	if err = validateActivityToken(authToken, activityToken); err != nil {
		return nil, err
	}

	// crafting UserActivity from requested Token name.
	userActivity := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: activityToken.GetCreationTime(),
			Name:              name,
		},
		Status: ext.UserActivityStatus{},
	}

	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()

	if lastSeen := activityToken.GetLastActivitySeen(); lastSeen != nil {
		userActivity.Status.ExpiresAt = lastSeen.Add(time.Minute * time.Duration(idleTimeout)).UTC().Format(time.RFC3339)
	}

	return userActivity, nil
}

// userFrom is a helper that extracts and validates the user info from the request's context.
func (s *Store) userFrom(ctx context.Context) (k8suser.Info, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("missing user info"))
	}

	userName := userInfo.GetName()

	if strings.Contains(userName, ":") { // E.g. system:admin
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	if !slices.Contains(userInfo.GetGroups(), GroupCattleAuthenticated) {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is not a Rancher user", userName))
	}

	user, err := s.userCache.Get(userName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s not found", userName))
		}

		return nil, apierrors.NewInternalError(fmt.Errorf("error getting user %s: %w", userName, err))
	}

	if user.Enabled != nil && !*user.Enabled {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), "", fmt.Errorf("user %s is disabled", userName))
	}

	return userInfo, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func validateActivityToken(auth, activity accessor.TokenAccessor) error {
	// verify auth and activity token have the same userID
	if auth.GetUserID() != activity.GetUserID() {
		return apierrors.NewForbidden(gvr.GroupResource(), "",
			fmt.Errorf("request token %s and activity token %s have different users", auth.GetName(), activity.GetName()))
	}

	// verify auth and activity token has the same auth provider
	if auth.GetAuthProvider() != activity.GetAuthProvider() {
		return apierrors.NewForbidden(gvr.GroupResource(), "",
			fmt.Errorf("request token %s and activity token %s have different auth providers", auth.GetName(), activity.GetName()))
	}

	// verify auth and activity token has the same auth user principal
	if auth.GetUserPrincipal().Name != activity.GetUserPrincipal().Name {
		return apierrors.NewForbidden(gvr.GroupResource(), "",
			fmt.Errorf("request token %s and activity token %s have different user principals", auth.GetName(), activity.GetName()))
	}

	// verify that activity token is a session token
	if activity.GetIsDerived() {
		return apierrors.NewForbidden(gvr.GroupResource(), "",
			fmt.Errorf("activity token %s is not a session token", activity.GetName()))
	}

	if !activity.GetIsEnabled() {
		return apierrors.NewForbidden(gvr.GroupResource(), "",
			fmt.Errorf("activity token %s is disabled", activity.GetName()))
	}

	return nil
}

var (
	_ rest.Creater                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)
