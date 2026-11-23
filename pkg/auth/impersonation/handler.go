package impersonation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type tokenManager interface {
	NewLoginToken(string, apiv3.Principal, []apiv3.Principal, string, int64, string) (*apiv3.Token, string, error)
	UpdateToken(*apiv3.Token) (*apiv3.Token, error)
	GetToken(string) (accessor.TokenAccessor, int, error)
	GetTokenForRecovery(string) (*apiv3.Token, bool, error)
	DeleteTokenByName(string) (int, error)
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

	eligible, err := h.CanImpersonateRequest(r, userID)
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

	target, err := h.users("").Get(userID, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &StartError{Status: http.StatusForbidden, Message: "user is not eligible for impersonation"}
		}
		return nil, &StartError{Status: http.StatusInternalServerError, Message: "unable to resolve target user"}
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
func (h *Handler) WriteStartResponse(w http.ResponseWriter, r *http.Request, result *StartResponse) {
	secure := h.isSecureRequest(r)
	setSessionCookie(w, tokens.OriginalSessionCookie, result.OriginalSessionValue, secure)
	setSessionCookie(w, sessionCookie, result.SessionValue, secure)
	writeJSON(w, http.StatusOK, responseBody{
		UserID: result.UserID, DisplayName: result.DisplayName, PrincipalID: result.PrincipalID, Impersonating: true,
	})
}

// CanImpersonate is the single server-side eligibility decision shared by the
// impersonation lifecycle and resource action formatters. A false result is a
// deliberate denial; an error means the decision could not be determined.
func (h *Handler) CanImpersonate(callerID, targetID string) (bool, error) {
	caller, err := h.users("").Get(callerID, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get impersonation caller %s: %w", callerID, err)
	}
	callerAdmin, err := h.isAdmin(caller.Name)
	if err != nil {
		return false, err
	}
	if !callerAdmin {
		return false, nil
	}

	target, err := h.users("").Get(targetID, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get impersonation target %s: %w", targetID, err)
	}
	targetAdmin, err := h.isAdmin(target.Name)
	if err != nil {
		return false, err
	}
	if !target.GetEnabled() || target.IsSystem() || targetAdmin || len(target.PrincipalIDs) == 0 {
		return false, nil
	}
	return true, nil
}

// CanImpersonateRequest applies request-level constraints before the shared
// caller/target eligibility decision.
func (h *Handler) CanImpersonateRequest(r *http.Request, targetID string) (bool, error) {
	caller, ok := request.UserFrom(r.Context())
	if !ok || caller.GetName() == "" {
		return false, nil
	}
	if _, err := r.Cookie(tokens.OriginalSessionCookie); err == nil {
		return false, nil
	}
	return h.CanImpersonate(caller.GetName(), targetID)
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

	secure := h.isSecureRequest(r)
	setSessionCookie(w, sessionCookie, original.Value, secure)
	clearCookie(w, tokens.OriginalSessionCookie, secure)
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
		secure := h.isSecureRequest(r)
		clearCookie(w, sessionCookie, secure)
		clearCookie(w, tokens.OriginalSessionCookie, secure)
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

	secure := h.isSecureRequest(r)
	setSessionCookie(w, sessionCookie, originalCookie.Value, secure)
	clearCookie(w, tokens.OriginalSessionCookie, secure)
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
	if _, _, err := h.validateSession(r, original.Value, effective.Value); err != nil {
		http.Error(w, err.Error(), sessionErrorStatus(err))
		return
	}

	displayName := current.GetName()
	principalID := ""
	if target, err := h.users("").Get(current.GetName(), metav1.GetOptions{}); err == nil {
		displayName = target.DisplayName
		if len(target.PrincipalIDs) > 0 {
			principalID = target.PrincipalIDs[0]
		}
	}

	writeJSON(w, http.StatusOK, responseBody{
		UserID:        current.GetName(),
		DisplayName:   displayName,
		PrincipalID:   principalID,
		Impersonating: true,
	})
}

// AuditMiddleware preserves the administrator identity in audit entries while
// the target user's session is active.
func (h *Handler) AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original, originalErr := r.Cookie(tokens.OriginalSessionCookie)
		effective, effectiveErr := r.Cookie(sessionCookie)
		current, currentErr := request.UserFrom(r.Context())
		auditUser, auditOK := audit.FromContext(r.Context())
		if originalErr == nil && effectiveErr == nil && original.Value != "" && effective.Value != "" && currentErr && current.GetName() != "" && auditOK {
			if originalToken, _, err := h.validateSession(r, original.Value, effective.Value); err == nil {
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
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) validateSession(r *http.Request, originalValue, effectiveValue string) (accessor.TokenAccessor, accessor.TokenAccessor, error) {
	originalToken, _, err := h.tokenManager.GetToken(originalValue)
	if err != nil || originalToken == nil || originalToken.GetUserID() == "" || originalToken.GetIsExpired() {
		return nil, nil, sessionError{status: http.StatusUnauthorized, message: "original administrator session is invalid"}
	}
	admin, err := h.isAdmin(originalToken.GetUserID())
	if err != nil {
		return nil, nil, sessionError{status: http.StatusInternalServerError, message: "unable to determine administrator access"}
	}
	if !admin {
		return nil, nil, sessionError{status: http.StatusUnauthorized, message: "original administrator session is invalid"}
	}
	effectiveToken, _, err := h.tokenManager.GetToken(effectiveValue)
	if err != nil || effectiveToken == nil || effectiveToken.GetUserID() != requestUserID(r) || effectiveToken.GetIsExpired() {
		return nil, nil, sessionError{status: http.StatusUnauthorized, message: "current impersonation session is invalid"}
	}
	if !tokens.IsImpersonationSessionBound(effectiveToken, originalToken) {
		return nil, nil, sessionError{status: http.StatusUnauthorized, message: "impersonation session binding is invalid"}
	}
	return originalToken, effectiveToken, nil
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
	principals := map[string]struct{}{}
	for _, principalID := range user.PrincipalIDs {
		principals[principalID] = struct{}{}
	}
	attributes, err := h.userAttributes("").Get(userID, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get user attributes %s: %w", userID, err)
	}
	if err == nil && attributes != nil {
		for _, groups := range attributes.GroupPrincipals {
			for _, group := range groups.Items {
				principals[group.Name] = struct{}{}
			}
		}
	}
	for _, binding := range bindings.Items {
		matches := binding.UserName == userID
		if !matches {
			_, matches = principals[binding.UserPrincipalName]
		}
		if !matches {
			_, matches = principals[binding.GroupPrincipalName]
		}
		if !matches {
			continue
		}
		role, err := h.globalRoles("").Get(binding.GlobalRoleName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("get global role %s: %w", binding.GlobalRoleName, err)
		}
		if role == nil {
			return false, fmt.Errorf("get global role %s: empty response", binding.GlobalRoleName)
		}
		if rbac.IsAdminGlobalRole(role) {
			return true, nil
		}
	}
	return false, nil
}

func requestUserID(r *http.Request) string {
	current, ok := request.UserFrom(r.Context())
	if !ok {
		return ""
	}
	return current.GetName()
}

func (h *Handler) isSecureRequest(r *http.Request) bool {
	return r.TLS != nil
}

func setSessionCookie(w http.ResponseWriter, name, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", Secure: secure, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", Secure: secure, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
