package useractivity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
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
	authorizer                       authorizer.Authorizer
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

func New(wranglerCtx *wrangler.Context, authorizer authorizer.Authorizer) *Store {
	return &Store{
		authorizer:                       authorizer,
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
func (s *Store) NamespaceScoped() bool { return false }

// GetSingularName implements [rest.SingularNameProvider]
func (s *Store) GetSingularName() string {
	return SingularName
}

// New implements [rest.Storage]
func (s *Store) New() runtime.Object {
	return &ext.UserActivity{}
}

// Destroy implements [rest.Storage]
func (s *Store) Destroy() {}

// Get implements [rest.Getter] and returns the UserActivity for a session token.
// The status.expiresAt tells when the idle timeout expires for the session.
// The spec.seenAt is calculated using the token's activityLastSeenAt value and the idle timeout setting.
func (s *Store) Get(ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	userInfo, _, isRancherUser, err := s.userFrom(ctx, "get")
	if err != nil {
		return nil, err
	}

	if !isRancherUser {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("user %s is not a Rancher user", userInfo.GetName()))
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("missing request token ID"))
	}

	// Retrieve the auth token.
	authToken, err := s.extTokenStore.Fetch(authTokenID)
	if err != nil {
		return nil, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	// Retrieve the activity token.
	token, err := s.extTokenStore.Fetch(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("token not found %s: %v", name, err))
		}

		return nil, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", name, err))
	}

	if err = validateToken(authToken, token, timeNow()); err != nil {
		return nil, err
	}

	idleTimeout := s.getAuthUserSessionIdleTTLMinutes()

	userActivity, err := s.fromToken(token, idleTimeout)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error creating useractivity from token %s: %w", name, err))
	}

	return userActivity, nil
}

// Update implements [rest.Updater] and sets the activityLastSeenAt of the session token
// to the provided spec.seenAt value or the current timestamp if omitted or if the spec.seenAt
// is in the future.
// If the spec.seenAt is before the current activityLastSeenAt, the latter is not updated.
// It returns Unauthorized if the session token itself and/or the session idle timeout has expired.
func (s *Store) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	userInfo, _, isRancherUser, err := s.userFrom(ctx, "update")
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error getting user info: %w", err))
	}

	if !isRancherUser {
		return nil, false, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("user %s is not a Rancher user", userInfo.GetName()))
	}

	extras := userInfo.GetExtra()

	authTokenID := first(extras[common.ExtraRequestTokenID])
	if authTokenID == "" {
		return nil, false, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("missing request token ID"))
	}

	// Retrieve the auth token.
	authToken, err := s.extTokenStore.Fetch(authTokenID)
	if err != nil {
		return nil, false, apierrors.NewForbidden(gvr.GroupResource(), name, fmt.Errorf("error getting request token %s: %w", authTokenID, err))
	}

	// Retrieve the session token.
	token, err := s.extTokenStore.Fetch(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, apierrors.NewNotFound(gvr.GroupResource(), name)
		}

		return nil, false, apierrors.NewInternalError(fmt.Errorf("failed to get token %s: %w", name, err))
	}

	// Maintain the same idle timeout value when reading and updating the UserActivity.
	idleTimeout := s.getAuthUserSessionIdleTTLMinutes()
	oldUserActivity, err := s.fromToken(token, idleTimeout)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error creating useractivity from token %s: %w", name, err))
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldUserActivity)
	if err != nil {
		return nil, false, apierrors.NewBadRequest(err.Error())
	}

	userActivity, ok := newObj.(*ext.UserActivity)
	if !ok {
		var zeroUA *ext.UserActivity
		return nil, false, apierrors.NewInternalError(fmt.Errorf("expected %T but got %T", zeroUA, userActivity))
	}

	if updateValidation != nil {
		err = updateValidation(ctx, userActivity, oldUserActivity)
		if err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				return nil, false, err
			}
			return nil, false, apierrors.NewBadRequest(fmt.Sprintf("update validation for useractivity %s failed: %s", name, err))
		}
	}

	now := timeNow()

	if err = validateToken(authToken, token, now); err != nil { // Always returns an apierror.
		return nil, false, err
	}

	// Default to the current timestamp.
	seen := now
	if userActivity.Spec.SeenAt != nil && userActivity.Spec.SeenAt.Time.Before(now) {
		// Use the provided seenAt timestamp only if it's not in the future.
		// This is to make sure the idle timeout can't be extended.
		seen = userActivity.Spec.SeenAt.Time
	}

	shouldUpdate := true
	lastSeen := token.GetLastActivitySeen()
	if lastSeen != nil && seen.Before(lastSeen.Time) {
		// If the SeenAt provided is before the last activity we have recorded,
		// we don't update the last activity time.
		seen = lastSeen.Time
		shouldUpdate = false
	}

	expiresAt := seen.Add(time.Minute * time.Duration(idleTimeout)).UTC()
	userActivity.Status.ExpiresAt = expiresAt.Format(time.RFC3339)
	// Always return the SeenAt we actually used.
	userActivity.Spec.SeenAt = &metav1.Time{Time: seen}

	dryRun := options != nil && len(options.DryRun) > 0 && options.DryRun[0] == metav1.DryRunAll

	if dryRun || !shouldUpdate {
		return userActivity, false, nil
	}

	var patched accessor.TokenAccessor
	switch token.(type) {
	case *apiv3.Token:
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
			return nil, false, apierrors.NewInternalError(fmt.Errorf("failed to marshall patch data: %w", err))
		}

		patched, err = s.tokens.Patch(name, types.JSONPatchType, patch)
		if err != nil {
			return nil, false, apierrors.NewInternalError(fmt.Errorf("failed to store activityLastSeenAt to token %s: %w", name, err))
		}
	case *ext.Token:
		patched, err = s.extTokenStore.UpdateLastActivitySeen(name, seen)
		if err != nil {
			return nil, false, apierrors.NewInternalError(fmt.Errorf("failed to store activityLastSeenAt to ext token %s: %w", name, err))
		}
	default:
		return nil, false, apierrors.NewInternalError(fmt.Errorf("unexpected token type %T", token))
	}

	userActivity, err = s.fromToken(patched, idleTimeout)
	if err != nil {
		return nil, false, apierrors.NewInternalError(fmt.Errorf("error creating useractivity from token %s: %w", name, err))
	}

	return userActivity, false, nil
}

func (s *Store) fromToken(obj any, idleTimeout int) (*ext.UserActivity, error) {
	token, ok := obj.(accessor.TokenAccessor)
	if !ok {
		return nil, fmt.Errorf("unexpected object type %T", obj)
	}

	meta, err := meta.Accessor(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get meta accessor: %w", err)
	}

	activity := &ext.UserActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name:              token.GetName(),
			CreationTimestamp: meta.GetCreationTimestamp(),
			UID:               meta.GetUID(), // Needed for objInfo.UpdatedObject to work.
			ResourceVersion:   meta.GetResourceVersion(),
		},
	}

	if lastSeen := token.GetLastActivitySeen(); lastSeen != nil {
		activity.Spec.SeenAt = &metav1.Time{Time: lastSeen.Time}
		activity.Status.ExpiresAt = lastSeen.Add(time.Minute * time.Duration(idleTimeout)).UTC().Format(time.RFC3339)
	}

	return activity, nil
}

// userFrom is a helper that extracts and validates the user info from the request's context.
func (s *Store) userFrom(ctx context.Context, verb string) (k8suser.Info, bool, bool, error) {
	userInfo, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false, false, fmt.Errorf("missing user info")
	}

	decision, _, err := s.authorizer.Authorize(ctx, &authorizer.AttributesRecord{
		User:            userInfo,
		Verb:            verb,
		Resource:        "*",
		ResourceRequest: true,
	})
	if err != nil {
		return nil, false, false, err
	}

	isAdmin := decision == authorizer.DecisionAllow

	isRancherUser := false

	if name := userInfo.GetName(); !strings.Contains(name, ":") { // E.g. system:admin
		_, err := s.userCache.Get(name)
		if err == nil {
			isRancherUser = true
		} else if !apierrors.IsNotFound(err) {
			return nil, false, false, fmt.Errorf("error getting user %s: %w", name, err)
		}
	}

	return userInfo, isAdmin, isRancherUser, nil
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func validateToken(authToken, token accessor.TokenAccessor, now time.Time) error {
	if authToken.GetUserID() != token.GetUserID() {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("users don't match"))
	}

	if authToken.GetAuthProvider() != token.GetAuthProvider() {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("auth providers don't match"))
	}

	if authToken.GetUserPrincipal().Name != token.GetUserPrincipal().Name {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("user principals don't match"))
	}

	if token.GetIsDerived() { // Not a session token.
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("not a session token"))
	}

	if !token.GetIsEnabled() {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("token is disabled"))
	}

	if token.GetIsExpired() {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("token is expired"))
	}

	// Don't revive the session if the idle timeout has alredy expired.
	if tokens.IsIdleExpired(token, now) {
		return apierrors.NewForbidden(gvr.GroupResource(), token.GetName(), errors.New("session idle timeout expired"))
	}

	return nil
}

var (
	_ rest.Updater                  = &Store{}
	_ rest.Patcher                  = &Store{}
	_ rest.Storage                  = &Store{}
	_ rest.Scoper                   = &Store{}
	_ rest.SingularNameProvider     = &Store{}
	_ rest.GroupVersionKindProvider = &Store{}
)
