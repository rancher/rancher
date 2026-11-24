package impersonation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	sessionCookie      = tokens.CookieName
	sessionDescription = "UI impersonation session"
)

type requestBody struct {
	UserID string `json:"userId"`
}

type responseBody struct {
	UserID        string `json:"userId"`
	DisplayName   string `json:"displayName"`
	PrincipalID   string `json:"principalId,omitempty"`
	Impersonating bool   `json:"impersonating"`
}

// StartResponse contains the data needed by an HTTP adapter after a
// successful start. StartSession never writes an HTTP response.
type StartResponse struct {
	UserID               string
	DisplayName          string
	PrincipalID          string
	OriginalSessionValue string
	SessionValue         string
}

// StartError is mapped to HTTP by each adapter exactly once.
type StartError struct {
	Status  int
	Message string
}

func (e *StartError) Error() string { return e.Message }

type recoveryResponse struct {
	Impersonating bool `json:"impersonating"`
	Restored      bool `json:"restored"`
}

type Handler struct {
	users              func(string) userGetter
	userAttributes     func(string) userAttributeGetter
	globalRoleBindings func(string) globalRoleBindingLister
	globalRoles        func(string) globalRoleGetter
	tokenManager       tokenManager

	cachedUsers              func(string) userGetter
	cachedUserAttributes     func(string) userAttributeGetter
	cachedGlobalRoleBindings func(string) globalRoleBindingLister
	cachedGlobalRoles        func(string) globalRoleGetter
}

type eligibilityCacheKey struct{}

type eligibilityCache struct {
	handler   *Handler
	evaluator *Handler
	mu        sync.Mutex

	initialized   bool
	callerAllowed bool
	resolver      *adminResolver
	err           error
	targets       map[string]eligibilityDecision
}

type eligibilityDecision struct {
	allowed bool
	err     error
}

type adminResolver struct {
	handler  *Handler
	bindings []apiv3.GlobalRoleBinding
	roles    map[string]adminRoleDecision
}

type adminRoleDecision struct {
	admin bool
	err   error
}

type userGetter interface {
	Get(string, metav1.GetOptions) (*apiv3.User, error)
}

type userAttributeGetter interface {
	Get(string, metav1.GetOptions) (*apiv3.UserAttribute, error)
}

type globalRoleBindingLister interface {
	List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error)
}

type globalRoleGetter interface {
	Get(string, metav1.GetOptions) (*apiv3.GlobalRole, error)
}

type userCacheAdapter struct {
	cache managementcontrollers.UserCache
}

func (a userCacheAdapter) Get(name string, _ metav1.GetOptions) (*apiv3.User, error) {
	return a.cache.Get(name)
}

type userAttributeCacheAdapter struct {
	cache managementcontrollers.UserAttributeCache
}

func (a userAttributeCacheAdapter) Get(name string, _ metav1.GetOptions) (*apiv3.UserAttribute, error) {
	return a.cache.Get(name)
}

type globalRoleBindingCacheAdapter struct {
	cache managementcontrollers.GlobalRoleBindingCache
}

func (a globalRoleBindingCacheAdapter) List(metav1.ListOptions) (*apiv3.GlobalRoleBindingList, error) {
	items, err := a.cache.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	result := &apiv3.GlobalRoleBindingList{Items: make([]apiv3.GlobalRoleBinding, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, *item)
	}
	return result, nil
}

type globalRoleCacheAdapter struct {
	cache managementcontrollers.GlobalRoleCache
}

func (a globalRoleCacheAdapter) Get(name string, _ metav1.GetOptions) (*apiv3.GlobalRole, error) {
	return a.cache.Get(name)
}

type tokenManager interface {
	NewLoginToken(string, apiv3.Principal, []apiv3.Principal, string, int64, string) (*apiv3.Token, string, error)
	UpdateToken(*apiv3.Token) (*apiv3.Token, error)
	GetToken(string) (accessor.TokenAccessor, int, error)
	GetTokenForRecovery(string) (*apiv3.Token, bool, error)
	DeleteTokenByName(string) (int, error)
}

type tokenByNameGetter interface {
	GetTokenByName(string) (accessor.TokenAccessor, int, error)
}

// The exported aliases allow package-level route tests to construct a handler
// with deterministic dependencies without changing the production constructor.
type UserGetter = userGetter
type UserAttributeGetter = userAttributeGetter
type GlobalRoleBindingLister = globalRoleBindingLister
type GlobalRoleGetter = globalRoleGetter
type TokenManager = tokenManager

func NewHandler(management *config.ScaledContext) *Handler {
	handler := newHandler(
		func(namespace string) userGetter {
			return management.Management.Users(namespace)
		},
		func(namespace string) userAttributeGetter {
			return management.Management.UserAttributes(namespace)
		},
		func(namespace string) globalRoleBindingLister {
			return management.Management.GlobalRoleBindings(namespace)
		},
		func(namespace string) globalRoleGetter {
			return management.Management.GlobalRoles(namespace)
		},
		tokens.NewManager(management.Wrangler),
	)
	handler.cachedUsers = func(string) userGetter {
		return userCacheAdapter{cache: management.Wrangler.Mgmt.User().Cache()}
	}
	handler.cachedUserAttributes = func(string) userAttributeGetter {
		return userAttributeCacheAdapter{cache: management.Wrangler.Mgmt.UserAttribute().Cache()}
	}
	handler.cachedGlobalRoleBindings = func(string) globalRoleBindingLister {
		return globalRoleBindingCacheAdapter{cache: management.Wrangler.Mgmt.GlobalRoleBinding().Cache()}
	}
	handler.cachedGlobalRoles = func(string) globalRoleGetter {
		return globalRoleCacheAdapter{cache: management.Wrangler.Mgmt.GlobalRole().Cache()}
	}
	return handler
}

func newHandler(
	users func(string) userGetter,
	userAttributes func(string) userAttributeGetter,
	globalRoleBindings func(string) globalRoleBindingLister,
	globalRoles func(string) globalRoleGetter,
	tokenManager tokenManager,
) *Handler {
	return &Handler{
		users:              users,
		userAttributes:     userAttributes,
		globalRoleBindings: globalRoleBindings,
		globalRoles:        globalRoles,
		tokenManager:       tokenManager,
	}
}

// NewHandlerForTest constructs the same lifecycle handler used in production
// with test doubles. It is intentionally limited to deterministic tests.
func NewHandlerForTest(
	users func(string) UserGetter,
	userAttributes func(string) UserAttributeGetter,
	globalRoleBindings func(string) GlobalRoleBindingLister,
	globalRoles func(string) GlobalRoleGetter,
	tokenManager TokenManager,
) *Handler {
	return newHandler(users, userAttributes, globalRoleBindings, globalRoles, tokenManager)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var input requestBody
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.UserID) == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	h.start(w, r, input.UserID)
}

func (h *Handler) StartForUser(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	h.start(w, r, userID)
}

// StartSession performs the complete start lifecycle without writing HTTP.
// Direct, Norman, and Steve entry points all use this operation.
func (h *Handler) StartSession(r *http.Request, userID string) (*StartResponse, error) {
	current, ok := request.UserFrom(r.Context())
	if !ok || current.GetName() == "" {
		return nil, &StartError{Status: http.StatusUnauthorized, Message: http.StatusText(http.StatusUnauthorized)}
	}
	if _, err := r.Cookie(tokens.OriginalSessionCookie); err == nil {
		return nil, &StartError{Status: http.StatusConflict, Message: "nested impersonation is not allowed"}
	}

	target, eligible, err := h.resolveImpersonation(current.GetName(), userID)
	if err != nil {
		status := http.StatusInternalServerError
		message := "unable to determine impersonation eligibility"
		if apierrors.IsNotFound(err) && strings.Contains(err.Error(), "target") {
			status = http.StatusForbidden
			message = "user is not eligible for impersonation"
		}
		return nil, &StartError{Status: status, Message: message}
	}
	if !eligible {
		return nil, &StartError{Status: http.StatusForbidden, Message: "impersonation requires administrator access"}
	}

	original, err := r.Cookie(sessionCookie)
	if err != nil || original.Value == "" {
		return nil, &StartError{Status: http.StatusUnauthorized, Message: "current session is unavailable"}
	}
	originalToken, _, err := h.tokenManager.GetToken(original.Value)
	if err != nil || originalToken == nil || originalToken.GetUserID() != current.GetName() || originalToken.GetIsExpired() {
		return nil, &StartError{Status: http.StatusUnauthorized, Message: "current session is unavailable"}
	}

	ttl, err := exttokenstore.ParseTTLToMilliseconds(settings.AuthUserSessionTTLMinutes)
	if err != nil {
		return nil, &StartError{Status: http.StatusInternalServerError, Message: "unable to configure impersonation session"}
	}
	if originalExpiresAt := originalToken.GetExpiresAt(); originalExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, originalExpiresAt)
		if err != nil {
			return nil, &StartError{Status: http.StatusUnauthorized, Message: "current session expiry is invalid"}
		}
		remainingTTL := time.Until(expiresAt).Milliseconds()
		if remainingTTL <= 0 {
			return nil, &StartError{Status: http.StatusUnauthorized, Message: "current session has expired"}
		}
		if remainingTTL < ttl {
			ttl = remainingTTL
		}
	}
	configName, _, _, err := common.SplitPrincipalID(target.PrincipalIDs[0])
	if err != nil {
		return nil, &StartError{Status: http.StatusForbidden, Message: "target user principal is invalid"}
	}
	targetToken, tokenKey, err := h.tokenManager.NewLoginToken(target.Name, apiv3.Principal{
		ObjectMeta: metav1.ObjectMeta{Name: target.PrincipalIDs[0]},
		Provider:   configName, PrincipalType: "user", Me: true,
	}, nil, "", ttl, sessionDescription)
	if err != nil {
		return nil, &StartError{Status: http.StatusInternalServerError, Message: "unable to create impersonation session"}
	}
	tokens.BindImpersonationSession(targetToken, originalToken)
	updatedToken, err := h.tokenManager.UpdateToken(targetToken)
	if err != nil || updatedToken == nil {
		_, _ = h.tokenManager.DeleteTokenByName(targetToken.Name)
		return nil, &StartError{Status: http.StatusInternalServerError, Message: "unable to bind impersonation session"}
	}

	return &StartResponse{
		UserID: target.Name, DisplayName: target.DisplayName, PrincipalID: target.PrincipalIDs[0],
		OriginalSessionValue: original.Value, SessionValue: updatedToken.Name + ":" + tokenKey,
	}, nil
}

// WriteStartResponse is the shared successful HTTP adapter.
func (h *Handler) WriteStartResponse(w http.ResponseWriter, _ *http.Request, result *StartResponse) {
	setSessionCookie(w, tokens.OriginalSessionCookie, result.OriginalSessionValue)
	setSessionCookie(w, sessionCookie, result.SessionValue)
	writeJSON(w, http.StatusOK, responseBody{
		UserID: result.UserID, DisplayName: result.DisplayName, PrincipalID: result.PrincipalID, Impersonating: true,
	})
}

// CanImpersonate is the single server-side eligibility decision shared by the
// impersonation lifecycle and resource action formatters. A false result is a
// deliberate denial; an error means the decision could not be determined.
func (h *Handler) CanImpersonate(callerID, targetID string) (bool, error) {
	_, eligible, err := h.resolveImpersonation(callerID, targetID)
	return eligible, err
}

func (h *Handler) resolveImpersonation(callerID, targetID string) (*apiv3.User, bool, error) {
	caller, err := h.users("").Get(callerID, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("get impersonation caller %s: %w", callerID, err)
	}
	resolver, err := h.newAdminResolver()
	if err != nil {
		return nil, false, err
	}
	callerAdmin, err := resolver.isAdmin(caller)
	if err != nil {
		return nil, false, err
	}
	if !callerAdmin {
		return nil, false, nil
	}

	target, err := h.users("").Get(targetID, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("get impersonation target %s: %w", targetID, err)
	}
	if target == nil {
		return nil, false, fmt.Errorf("get impersonation target %s: empty response", targetID)
	}
	eligible, err := h.canImpersonateUser(resolver, target)
	return target, eligible, err
}

func (h *Handler) canImpersonateTarget(resolver *adminResolver, targetID string) (bool, error) {
	target, err := h.users("").Get(targetID, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get impersonation target %s: %w", targetID, err)
	}
	return h.canImpersonateUser(resolver, target)
}

func (h *Handler) canImpersonateUser(resolver *adminResolver, target *apiv3.User) (bool, error) {
	targetAdmin, err := resolver.isAdmin(target)
	if err != nil {
		return false, err
	}
	if !target.GetEnabled() || target.IsSystem() || targetAdmin || len(target.PrincipalIDs) == 0 {
		return false, nil
	}
	return true, nil
}

// WithEligibilityCache adds request-scoped memoization used by resource
// formatters. The lifecycle action itself deliberately performs a fresh
// authorization decision.
func (h *Handler) WithEligibilityCache(r *http.Request) *http.Request {
	if r == nil {
		return nil
	}
	if cache, ok := r.Context().Value(eligibilityCacheKey{}).(*eligibilityCache); ok && cache.handler == h {
		return r
	}
	cache := &eligibilityCache{
		handler:   h,
		evaluator: h.cachedEligibilityHandler(),
		targets:   map[string]eligibilityDecision{},
	}
	return r.WithContext(context.WithValue(r.Context(), eligibilityCacheKey{}, cache))
}

// CanImpersonateRequest applies request-level constraints before the shared
// caller/target eligibility decision.
func (h *Handler) CanImpersonateRequest(r *http.Request, targetID string) (bool, error) {
	return h.canImpersonateRequest(r, targetID, nil)
}

// CanImpersonateUserRequest evaluates a target already loaded by a resource
// formatter, avoiding a second user lookup.
func (h *Handler) CanImpersonateUserRequest(r *http.Request, target *apiv3.User) (bool, error) {
	if target == nil || target.Name == "" {
		return false, nil
	}
	return h.canImpersonateRequest(r, target.Name, target)
}

func (h *Handler) canImpersonateRequest(r *http.Request, targetID string, target *apiv3.User) (bool, error) {
	caller, ok := request.UserFrom(r.Context())
	if !ok || caller.GetName() == "" {
		return false, nil
	}
	if _, err := r.Cookie(tokens.OriginalSessionCookie); err == nil {
		return false, nil
	}
	if cache, ok := r.Context().Value(eligibilityCacheKey{}).(*eligibilityCache); ok && cache.handler == h {
		return cache.canImpersonate(caller.GetName(), targetID, target)
	}
	if target != nil {
		callerUser, err := h.users("").Get(caller.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("get impersonation caller %s: %w", caller.GetName(), err)
		}
		resolver, err := h.newAdminResolver()
		if err != nil {
			return false, err
		}
		admin, err := resolver.isAdmin(callerUser)
		if err != nil || !admin {
			return false, err
		}
		return h.canImpersonateUser(resolver, target)
	}
	return h.CanImpersonate(caller.GetName(), targetID)
}

func (c *eligibilityCache) canImpersonate(callerID, targetID string, target *apiv3.User) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		c.initialized = true
		caller, err := c.evaluator.users("").Get(callerID, metav1.GetOptions{})
		if err != nil {
			c.err = fmt.Errorf("get impersonation caller %s: %w", callerID, err)
		} else if c.resolver, c.err = c.evaluator.newAdminResolver(); c.err == nil {
			c.callerAllowed, c.err = c.resolver.isAdmin(caller)
		}
	}
	if c.err != nil || !c.callerAllowed {
		return false, c.err
	}
	if decision, ok := c.targets[targetID]; ok {
		return decision.allowed, decision.err
	}
	var allowed bool
	var err error
	if target == nil {
		allowed, err = c.evaluator.canImpersonateTarget(c.resolver, targetID)
	} else {
		allowed, err = c.evaluator.canImpersonateUser(c.resolver, target)
	}
	c.targets[targetID] = eligibilityDecision{allowed: allowed, err: err}
	return allowed, err
}

func (h *Handler) cachedEligibilityHandler() *Handler {
	if h.cachedUsers == nil || h.cachedUserAttributes == nil || h.cachedGlobalRoleBindings == nil || h.cachedGlobalRoles == nil {
		return h
	}
	cached := *h
	cached.users = h.cachedUsers
	cached.userAttributes = h.cachedUserAttributes
	cached.globalRoleBindings = h.cachedGlobalRoleBindings
	cached.globalRoles = h.cachedGlobalRoles
	return &cached
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request, userID string) {
	result, err := h.StartSession(r, userID)
	if err != nil {
		startErr, ok := err.(*StartError)
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		http.Error(w, startErr.Message, startErr.Status)
		return
	}
	h.WriteStartResponse(w, r, result)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	original, err := r.Cookie(tokens.OriginalSessionCookie)
	if err != nil || original.Value == "" {
		http.Error(w, "no impersonation session is active", http.StatusNotFound)
		return
	}
	current, err := r.Cookie(sessionCookie)
	if err != nil || current.Value == "" {
		http.Error(w, "current impersonation session is unavailable", http.StatusUnauthorized)
		return
	}

	_, currentToken, err := h.validateSession(r, original.Value, current.Value)
	if err != nil {
		http.Error(w, err.Error(), sessionErrorStatus(err))
		return
	}
	if _, err := h.tokenManager.DeleteTokenByName(currentToken.GetFullName()); err != nil {
		http.Error(w, "unable to end impersonation session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionCookie, original.Value)
	clearCookie(w, tokens.OriginalSessionCookie)
	writeJSON(w, http.StatusOK, responseBody{Impersonating: false})
}

// Recover cleans up an expired impersonation session. It deliberately does
// not use the normal authenticated middleware because the effective token may
// already be expired; both cookie secrets and the server-side binding are
// validated here instead.
func (h *Handler) Recover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	originalCookie, originalErr := r.Cookie(tokens.OriginalSessionCookie)
	effectiveCookie, effectiveErr := r.Cookie(sessionCookie)
	clear := func() {
		clearCookie(w, sessionCookie)
		clearCookie(w, tokens.OriginalSessionCookie)
	}
	cleanup := func() {
		clear()
		writeJSON(w, http.StatusOK, recoveryResponse{Impersonating: false, Restored: false})
	}

	if originalErr != nil || effectiveErr != nil || originalCookie.Value == "" || effectiveCookie.Value == "" {
		cleanup()
		return
	}

	originalToken, status, err := h.tokenManager.GetToken(originalCookie.Value)
	if err != nil || originalToken == nil || originalToken.GetIsExpired() {
		clear()
		if status >= http.StatusInternalServerError || status == 0 {
			http.Error(w, "unable to inspect impersonation session", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, recoveryResponse{Impersonating: false, Restored: false})
		return
	}
	admin, err := h.isAdmin(originalToken.GetUserID())
	if err != nil {
		clear()
		http.Error(w, "unable to determine administrator access", http.StatusInternalServerError)
		return
	}
	if !admin {
		cleanup()
		return
	}

	effectiveToken, expired, err := h.tokenManager.GetTokenForRecovery(effectiveCookie.Value)
	if err != nil || effectiveToken == nil || !tokens.IsImpersonationSessionBound(effectiveToken, originalToken) {
		cleanup()
		return
	}
	if !expired {
		http.Error(w, "impersonation session is still active", http.StatusConflict)
		return
	}
	if _, err := h.tokenManager.DeleteTokenByName(effectiveToken.GetFullName()); err != nil {
		clear()
		http.Error(w, "unable to end impersonation session", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, sessionCookie, originalCookie.Value)
	clearCookie(w, tokens.OriginalSessionCookie)
	writeJSON(w, http.StatusOK, recoveryResponse{Impersonating: false, Restored: true})
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	original, err := r.Cookie(tokens.OriginalSessionCookie)
	if err != nil || original.Value == "" {
		http.Error(w, "no impersonation session is active", http.StatusNotFound)
		return
	}

	current, ok := request.UserFrom(r.Context())
	if !ok || current.GetName() == "" {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	effective, err := r.Cookie(sessionCookie)
	if err != nil || effective.Value == "" {
		http.Error(w, "current impersonation session is unavailable", http.StatusUnauthorized)
		return
	}
	_, effectiveToken, err := h.validateSession(r, original.Value, effective.Value)
	if err != nil {
		http.Error(w, err.Error(), sessionErrorStatus(err))
		return
	}

	userID := effectiveToken.GetUserID()
	displayName := userID
	principalID := effectiveToken.GetUserPrincipal().Name
	if target, err := h.users("").Get(userID, metav1.GetOptions{}); err == nil && target != nil && target.DisplayName != "" {
		displayName = target.DisplayName
	}

	writeJSON(w, http.StatusOK, responseBody{
		UserID:        userID,
		DisplayName:   displayName,
		PrincipalID:   principalID,
		Impersonating: true,
	})
}

// AuditMiddleware enforces the original-to-effective session binding and
// preserves the administrator identity in audit entries.
func (h *Handler) AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Recovery validates both cookies itself and must remain reachable after
		// the effective token has expired.
		if r.URL.Path == "/v3/impersonation-sessions/recover" {
			next.ServeHTTP(w, r)
			return
		}

		original, originalErr := r.Cookie(tokens.OriginalSessionCookie)
		current, currentErr := request.UserFrom(r.Context())
		auditUser, auditOK := audit.FromContext(r.Context())
		hasOriginal := originalErr == nil && original.Value != ""
		hasAuthenticatedUser := currentErr && current.GetName() != "" && current.GetName() != "system:cattle:error"
		var tokenIDs []string
		if hasAuthenticatedUser {
			tokenIDs = current.GetExtra()[tokens.ImpersonationTokenIDUserExtra]
		}
		selectedImpersonation := len(tokenIDs) > 0

		// Public, failed-authentication, and regular authenticated requests are
		// enforced by their route handlers. A trusted authenticator marker
		// identifies the exact selected token, independent of its transport.
		if !selectedImpersonation {
			next.ServeHTTP(w, r)
			return
		}

		if len(tokenIDs) != 1 || tokenIDs[0] == "" {
			http.Error(w, "current impersonation session is invalid", http.StatusUnauthorized)
			return
		}
		tokenGetter, ok := h.tokenManager.(tokenByNameGetter)
		if !ok {
			http.Error(w, "unable to validate current session", http.StatusInternalServerError)
			return
		}
		effectiveToken, effectiveStatus, effectiveLookupErr := tokenGetter.GetTokenByName(tokenIDs[0])
		if effectiveLookupErr != nil || effectiveToken == nil {
			status := http.StatusUnauthorized
			if effectiveStatus >= http.StatusInternalServerError {
				status = http.StatusInternalServerError
			}
			http.Error(w, "unable to validate current session", status)
			return
		}
		if !tokens.IsImpersonationSession(effectiveToken) {
			http.Error(w, "current impersonation session is invalid", http.StatusUnauthorized)
			return
		}
		if !hasOriginal {
			http.Error(w, "original administrator session is invalid", http.StatusUnauthorized)
			return
		}

		// Re-evaluate the original administrator against the latest informer
		// state on every request. This keeps role revocation effective without
		// amplifying normal Dashboard traffic into management API calls.
		originalToken, err := h.cachedEligibilityHandler().validateSelectedSession(r, original.Value, effectiveToken)
		if err != nil {
			http.Error(w, err.Error(), sessionErrorStatus(err))
			return
		}
		if auditOK {
			realPrincipal := originalToken.GetUserPrincipal()
			realExtra := map[string][]string{}
			for key, value := range realPrincipal.ExtraInfo {
				realExtra[key] = []string{value}
			}
			realGroups := make([]string, 0, len(originalToken.GetGroupPrincipals()))
			for _, group := range originalToken.GetGroupPrincipals() {
				realGroups = append(realGroups, group.Name)
			}
			real := audit.WithIdentity(&k8suser.DefaultInfo{
				Name:   originalToken.GetUserID(),
				UID:    originalToken.GetUserID(),
				Groups: realGroups,
				Extra:  realExtra,
			}, audit.Identity{
				Username:    realPrincipal.LoginName,
				Provider:    realPrincipal.Provider,
				PrincipalID: realPrincipal.Name,
			})
			auditUser.Impersonate(real, current)
			auditUser.SetImpersonationSessionID(originalToken.GetFullName())
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) validateSession(r *http.Request, originalValue, effectiveValue string) (accessor.TokenAccessor, accessor.TokenAccessor, error) {
	effectiveToken, _, err := h.tokenManager.GetToken(effectiveValue)
	if err != nil || effectiveToken == nil {
		return nil, nil, sessionError{status: http.StatusUnauthorized, message: "current impersonation session is invalid"}
	}
	originalToken, err := h.validateSelectedSession(r, originalValue, effectiveToken)
	if err != nil {
		return nil, nil, err
	}
	return originalToken, effectiveToken, nil
}

func (h *Handler) validateSelectedSession(r *http.Request, originalValue string, effectiveToken accessor.TokenAccessor) (accessor.TokenAccessor, error) {
	originalToken, _, err := h.tokenManager.GetToken(originalValue)
	if err != nil || originalToken == nil || originalToken.GetUserID() == "" || originalToken.GetIsExpired() {
		return nil, sessionError{status: http.StatusUnauthorized, message: "original administrator session is invalid"}
	}
	admin, err := h.isAdmin(originalToken.GetUserID())
	if err != nil {
		return nil, sessionError{status: http.StatusInternalServerError, message: "unable to determine administrator access"}
	}
	if !admin {
		return nil, sessionError{status: http.StatusUnauthorized, message: "original administrator session is invalid"}
	}
	if effectiveToken.GetUserID() != requestUserID(r) || effectiveToken.GetIsExpired() {
		return nil, sessionError{status: http.StatusUnauthorized, message: "current impersonation session is invalid"}
	}
	if !tokens.IsImpersonationSessionBound(effectiveToken, originalToken) {
		return nil, sessionError{status: http.StatusUnauthorized, message: "impersonation session binding is invalid"}
	}
	return originalToken, nil
}

type sessionError struct {
	status  int
	message string
}

func (e sessionError) Error() string {
	return e.message
}

func sessionErrorStatus(err error) int {
	if sessionErr, ok := err.(sessionError); ok {
		return sessionErr.status
	}
	return http.StatusUnauthorized
}

func (h *Handler) isAdmin(userID string) (bool, error) {
	bindings, err := h.globalRoleBindings("").List(metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("list global role bindings: %w", err)
	}
	user, err := h.users("").Get(userID, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get user %s: %w", userID, err)
	}
	if user == nil {
		return false, fmt.Errorf("get user %s: empty response", userID)
	}
	resolver := &adminResolver{
		handler:  h,
		bindings: bindings.Items,
		roles:    map[string]adminRoleDecision{},
	}
	return resolver.isAdmin(user)
}

func (h *Handler) newAdminResolver() (*adminResolver, error) {
	bindings, err := h.globalRoleBindings("").List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list global role bindings: %w", err)
	}
	return &adminResolver{
		handler:  h,
		bindings: bindings.Items,
		roles:    map[string]adminRoleDecision{},
	}, nil
}

func (r *adminResolver) isAdmin(user *apiv3.User) (bool, error) {
	if user == nil {
		return false, fmt.Errorf("get user: empty response")
	}
	principals := map[string]struct{}{}
	for _, principalID := range user.PrincipalIDs {
		if principalID != "" {
			principals[principalID] = struct{}{}
		}
	}
	attributes, err := r.handler.userAttributes("").Get(user.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get user attributes %s: %w", user.Name, err)
	}
	if err == nil && attributes != nil {
		for _, groups := range attributes.GroupPrincipals {
			for _, group := range groups.Items {
				if group.Name != "" {
					principals[group.Name] = struct{}{}
				}
			}
		}
	}
	for _, binding := range r.bindings {
		matches := binding.UserName != "" && binding.UserName == user.Name
		if !matches && binding.UserPrincipalName != "" {
			_, matches = principals[binding.UserPrincipalName]
		}
		if !matches && binding.GroupPrincipalName != "" {
			_, matches = principals[binding.GroupPrincipalName]
		}
		if !matches {
			continue
		}
		admin, err := r.isAdminRole(binding.GlobalRoleName)
		if err != nil {
			return false, err
		}
		if admin {
			return true, nil
		}
	}
	return false, nil
}

func (r *adminResolver) isAdminRole(name string) (bool, error) {
	if decision, ok := r.roles[name]; ok {
		return decision.admin, decision.err
	}
	role, err := r.handler.globalRoles("").Get(name, metav1.GetOptions{})
	if err != nil {
		err = fmt.Errorf("get global role %s: %w", name, err)
	} else if role == nil {
		err = fmt.Errorf("get global role %s: empty response", name)
	}
	decision := adminRoleDecision{err: err}
	if err == nil {
		decision.admin = rbac.IsAdminGlobalRole(role)
	}
	r.roles[name] = decision
	return decision.admin, decision.err
}

func requestUserID(r *http.Request) string {
	current, ok := request.UserFrom(r.Context())
	if !ok {
		return ""
	}
	return current.GetName()
}

func setSessionCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
