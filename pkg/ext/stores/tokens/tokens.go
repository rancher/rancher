package tokens

import (
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	extcore "github.com/rancher/steve/pkg/ext"

	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/registry/rest"
)

const (
	TokenNamespace = "cattle-tokens"
	ThirtyDays     = 30 * 24 * 60 * 60 * 1000 // 30 days in milliseconds.
	UserIDLabel    = "authn.management.cattle.io/token-userId"

	// data fields used by the backing secrets to store token information
	fieldAuthProvider   = "auth-provider"
	fieldClusterName    = "cluster-name"
	fieldDescription    = "description"
	fieldDisplayName    = "display-name"
	fieldEnabled        = "enabled"
	fieldHash           = "hash"
	fieldIsLogin        = "is-login"
	fieldLastUpdateTime = "last-update-time"
	fieldLastUsedAt     = "last-used-at"
	fieldLoginName      = "login-name"
	fieldPrincipalID    = "principal-id"
	fieldTTL            = "ttl"
	fieldUID            = "kube-uid"
	fieldUserID         = "user-id"
	// fieldIdleTimeout = "idle-timeout"	FUTURE ((USER ACTIVITY))

	noUpdatePermission      = 0
	fullUpdatePermission    = 1
	limitedUpdatePermission = 2
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// TokenStore is the interface to the token store seen by the extension API and users. Wrapped
// around a SystemTokenStore it performs the necessary checks to ensure that Users have only access
// to the tokens they are permitted to.
type TokenStore struct {
	namespaceClient v1.NamespaceClient // access to namespaces
	SystemTokenStore
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemTokenStore is the interface to the token store used internally by other parts of
// rancher. It does not perform any kind of permission checks, and operates with (implied) admin
// authority. IOW it has access to all the tokens, in all ways.
type SystemTokenStore struct {
	support             supportActionHandler // subsystem for permission checking, secret value and hashing, timing
	secretClient        v1.SecretClient
	userAttributeClient v3.UserAttributeClient
	userClient          v3.UserClient
}

func NewTokenStoreFromWrangler(wranglerContext *wrangler.Context) extcore.Store[*ext.Token, *ext.TokenList] {
	return NewTokenStore(
		wranglerContext.Core.Namespace(),
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.UserAttribute(),
		wranglerContext.Mgmt.User(),
		NewSupportActionHandler(),
	)
}

func NewTokenStore(
	namespaceClient v1.NamespaceClient,
	secretClient v1.SecretClient,
	uaClient v3.UserAttributeController,
	userClient v3.UserController,
	support supportActionHandler,
) extcore.Store[*ext.Token, *ext.TokenList] {
	tokenStore := TokenStore{
		namespaceClient: namespaceClient,
		SystemTokenStore: SystemTokenStore{
			secretClient:        secretClient,
			userAttributeClient: uaClient,
			userClient:          userClient,
			support:             support,
		},
	}
	return &tokenStore
}

func NewSystemTokenStoreFromWrangler(wranglerContext *wrangler.Context) *SystemTokenStore {
	return NewSystemTokenStore(
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.UserAttribute(),
		wranglerContext.Mgmt.User(),
		NewSupportActionHandler(),
	)
}

func NewSystemTokenStore(
	secretClient v1.SecretClient,
	uaClient v3.UserAttributeController,
	userClient v3.UserController,
	support supportActionHandler,
) *SystemTokenStore {
	tokenStore := SystemTokenStore{
		secretClient:        secretClient,
		userAttributeClient: uaClient,
		userClient:          userClient,
		support:             support,
	}
	return &tokenStore
}

func (t *TokenStore) Create(ctx extcore.Context, token *ext.Token, opts *metav1.CreateOptions) (*ext.Token, error) {
	if _, err := t.support.UserHasPermission("create", token, ctx); err != nil {
		return nil, err
	}
	// note: without error permission is implied.

	// ensure existence of the namespace holding our secrets
	_, err := t.namespaceClient.Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: TokenNamespace,
		},
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}

	// reject creation of token which already exists
	currentSecret, err := t.secretClient.Get(TokenNamespace, token.Name, metav1.GetOptions{})
	if err == nil && currentSecret != nil {
		return nil, apierrors.NewAlreadyExists(ctx.GroupVersionResource.GroupResource(), token.Name)
	}

	// reject user-provided token value, or hash
	if token.Status.TokenValue != "" {
		return nil, apierrors.NewBadRequest("User provided token value is not permitted")
	}
	if token.Status.TokenHash != "" {
		return nil, apierrors.NewBadRequest("User provided token hash is not permitted")
	}

	// Get derived User information for the token.
	//
	// NOTE: The creation process is not given `AuthProvider`, `User-` and `GroupPrincipals`.
	// ..... This information have to be retrieved from somewhere else in the system.
	// ..... This is in contrast to the Norman tokens who get this information either
	// ..... as part of the Login process, or by copying the information out of the
	// ..... base token the new one is derived from. None of that is possible here.
	//
	// A User's `AuthProvider` information is generally captured in their associated
	// `UserAttribute` resource. This is what we retrieve and use here now to fill these fields
	// of the token to be.
	//
	// `ProviderInfo` is not supported. Norman tokens have it as legacy fallback to hold the
	// `access_token` data managed by OIDC-based auth providers. The actual primary storage for
	// this is actually a regular k8s Secret associated with the User.
	//
	// `UserPrincipal` is filled in part with standard information, and in part from the
	// associated `User`s fields.

	attribs, err := t.userAttributeClient.Get(token.Spec.UserID, metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve user attributes of %s: %w",
			token.Spec.UserID, err))
	}
	if attribs == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to get user attributes of %s",
			token.Spec.UserID))
	}

	if len(attribs.ExtraByProvider) != 1 {
		return nil, apierrors.NewInternalError(fmt.Errorf("bad user attributes: bogus ExtraByProvider, empty or ambigous"))
	}

	user, err := t.userClient.Get(token.Spec.UserID, metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve user %s: %w",
			token.Spec.UserID, err))
	}
	if user == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to get user %s", token.Spec.UserID))
	}

	// Generate secret and its hash

	tokenValue, hashedValue, err := t.support.MakeAndHashSecret()
	if err != nil {
		return nil, err
	}

	// (len == 1) => The single key in the UA map names the auth provider
	// (and where to look in GPs, if we were using GPs)
	var authProvider string
	for ap, _ := range attribs.ExtraByProvider {
		authProvider = ap
		break
	}

	token.Status.TokenHash = hashedValue
	token.Status.AuthProvider = authProvider
	token.Status.DisplayName = user.DisplayName
	token.Status.LoginName = user.Username // See also attribs.ExtraByProvider[ap]["username"][0]
	token.Status.PrincipalID = attribs.ExtraByProvider[authProvider]["principalid"][0]
	token.Status.LastUpdateTime = t.support.Now()

	rest.FillObjectMetaSystemFields(token)

	secret := secretFromToken(token)

	// Save new secret
	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// can happen despite the early check for a pre-existing secret.
			// something else may have raced us while the secret was assembled.
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to store token %s: %w",
			token.Name, err))
	}

	// Read changes back to return what was truly created, not what we thought we created
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			token.Name, err))
	}

	// users don't care about the hashed value
	newToken.Status.TokenHash = ""
	newToken.Status.TokenValue = tokenValue
	return newToken, nil
}

func (t *TokenStore) Update(ctx extcore.Context, token *ext.Token, opts *metav1.UpdateOptions) (*ext.Token, error) {

	// Work on the time-to-live value is a bit more complicated. Even the owning user is not
	// allowed to extend the TTL. Only a `manage-token`-permitted user is allowed to do
	// that. This is not quite captured by a `hasPermission` flag.

	permissionLevel := noUpdatePermission

	if ctx.User.GetName() == token.Spec.UserID {
		permissionLevel = limitedUpdatePermission
	} else {
		hasPermission, err := t.support.UserHasPermission("update", token, ctx)
		if err != nil {
			return nil, err
		}
		if hasPermission {
			permissionLevel = fullUpdatePermission
		}
	}

	return t.SystemTokenStore.update(permissionLevel, token, opts)
}

func (t *SystemTokenStore) Update(token *ext.Token, opts *metav1.UpdateOptions) (*ext.Token, error) {
	return t.update(fullUpdatePermission, token, opts)
}

func (t *SystemTokenStore) update(permissionLevel int, token *ext.Token, opts *metav1.UpdateOptions) (*ext.Token, error) {
	currentSecret, err := t.secretClient.Get(TokenNamespace, token.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w",
			token.Name, err))
	}
	currentToken, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w",
			token.Name, err))
	}

	if token.Spec.UserID != currentToken.Spec.UserID {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit user id",
			token.Name))
	}
	if token.Spec.ClusterName != currentToken.Spec.ClusterName {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit cluster name",
			token.Name))
	}
	if token.Spec.IsLogin != currentToken.Spec.IsLogin {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit flag isLogin",
			token.Name))
	}

	// Work on the time to live value is a bit more complicated. Even the owning user is not
	// allowed to extend the TTL. Only a `manage-token`-permitted user is allowed to do
	// that. This is not quite captured by a `hasPermission` flag.

	if permissionLevel == limitedUpdatePermission {
		if token.Spec.TTL > currentToken.Spec.TTL {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to extend time-to-live",
				token.Name))
		}
	} else if permissionLevel == noUpdatePermission {
		if token.Spec.TTL != currentToken.Spec.TTL {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to edit time-to-live",
				token.Name))
		}
	}

	// Keep the status of the resource unchanged, never store a token value, etc.
	// IOW changes to display name, login name, etc. are all ignored without a peep.
	token.Status = currentToken.Status
	token.Status.TokenValue = ""
	// Refresh time of last update to current.
	token.Status.LastUpdateTime = t.support.Now()

	// Save changes to backing secret
	secret := secretFromToken(token)

	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to update token %s: %w",
			token.Name, err))
	}

	// Read changes back to return what was truly saved, not what we thought we saved
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to regenerate token %s: %w",
			token.Name, err))
	}

	newToken.Status.TokenValue = ""
	return newToken, nil
}

// FUTURE ((USER ACTIVITY)) modify and fill in as required by the field type.
// func (t *SystemTokenStore) UpdateIdleTimeout(name string, now time.Time) error {
// }

func (t *SystemTokenStore) UpdateLastUsedAt(name string, now time.Time) error {
	// Operate directly on the backend secret holding the token

	nowStr := now.Format(time.RFC3339)
	patch, err := json.Marshal([]struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value"`
	}{{
		Op:    "replace",
		Path:  "/data/" + fieldLastUsedAt,
		Value: nowStr,
	}})
	if err != nil {
		return err
	}

	_, err = t.secretClient.Patch(TokenNamespace, name, types.JSONPatchType, patch)
	return err
}

func (t *TokenStore) Get(ctx extcore.Context, name string, opts *metav1.GetOptions) (*ext.Token, error) {
	// have to get token first before we can check permissions on user mismatch

	token, err := t.SystemTokenStore.Get(name, opts)
	if err != nil {
		return nil, err
	}
	if _, err := t.support.UserHasPermission("get", token, ctx); err != nil {
		return nil, err
	}

	return token, nil
}

func (t *SystemTokenStore) Get(name string, opts *metav1.GetOptions) (*ext.Token, error) {
	// Core token retrieval from backing secrets
	currentSecret, err := t.secretClient.Get(TokenNamespace, name, *opts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", name, err))
	}
	token, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to extract token %s: %w", name, err))
	}
	token.Status.TokenValue = ""
	return token, nil
}

func (t *TokenStore) List(ctx extcore.Context, opts *metav1.ListOptions) (*ext.TokenList, error) {
	// cannot use checkForManageToken here. we have lots of tokens to check, with the same admin value.
	hasPermission, err := t.support.UserHasManageTokenPermissions(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}
	return t.SystemTokenStore.list(hasPermission, ctx.User.GetName(), opts)
}

func (t *SystemTokenStore) List(opts *metav1.ListOptions) (*ext.TokenList, error) {
	return t.list(true, "", opts)
}

func (t *SystemTokenStore) list(hasPermission bool, user string, opts *metav1.ListOptions) (*ext.TokenList, error) {
	// Core token listing from backing secrets
	secrets, err := t.secretClient.List(TokenNamespace, *opts)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list tokens: %w", err))
	}
	var tokens []ext.Token
	for _, secret := range secrets.Items {
		token, err := tokenFromSecret(&secret)
		if err != nil {
			// ignore tokens with broken information
			continue
		}
		// users can only list their own tokens, unless they have full permissions on this group
		if !hasPermission && token.Spec.UserID != user {
			continue
		}
		tokens = append(tokens, *token)
	}
	list := ext.TokenList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: secrets.ResourceVersion,
		},
		Items: tokens,
	}
	return &list, nil
}

func (t *TokenStore) Watch(ctx extcore.Context, opts *metav1.ListOptions) (<-chan extcore.WatchEvent[*ext.Token], error) {
	// cannot use checkForManageToken here. we have lots of tokens to check, with the same admin value.
	hasPermission, err := t.support.UserHasManageTokenPermissions(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}

	ch := make(chan extcore.WatchEvent[*ext.Token])

	go func() {
		// watch backend secret, transform its events into token events
		watcher, err := t.secretClient.Watch(TokenNamespace, metav1.ListOptions{
			ResourceVersion: opts.ResourceVersion,
		})
		if err != nil {
			close(ch)
			return
		}

		ctxUser := ctx.User.GetName()

		defer watcher.Stop()
		for {
			select {
			case <-ctx.Done():
				// context got cancelled - done
				close(ch)
				return
			case event, more := <-watcher.ResultChan():
				// no more events - done
				if !more {
					close(ch)
					return
				}

				secret, ok := event.Object.(*corev1.Secret)
				if !ok {
					continue
				}

				token, err := tokenFromSecret(secret)
				if err != nil {
					// ignore broken tokens
					continue
				}
				if !hasPermission && token.Spec.UserID != ctxUser {
					// ignore tokens belonging to other users if this user does
					// not have the necessary permissions
					continue
				}

				ch <- extcore.WatchEvent[*ext.Token]{
					Event:  event.Type,
					Object: token,
				}
			}
		}
	}()

	return ch, nil
}

func (t *TokenStore) Delete(ctx extcore.Context, name string, opts *metav1.DeleteOptions) error {
	// have to pull the token information first before we can check permissions
	secret, err := t.secretClient.Get(TokenNamespace, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}
		return apierrors.NewInternalError(fmt.Errorf("failed to retrieve token %s: %w", name, err))
	}

	// (**) conversion errors are ignored here. we still get a partially filled token usable
	// enough for the permission check. even a missing user id (== empty string) is
	// acceptable. this simply triggers the non-owned secret code path, preventing deletion from
	// all but token admins.
	token, _ := tokenFromSecret(secret)
	if _, err := t.support.UserHasPermission("delete", token, ctx); err != nil {
		return err
	}

	return t.SystemTokenStore.Delete(name, opts)
}

func (t *SystemTokenStore) Delete(name string, opts *metav1.DeleteOptions) error {
	err := t.secretClient.Delete(TokenNamespace, name, opts)
	if err == nil {
		return nil
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	return apierrors.NewInternalError(fmt.Errorf("failed to delete token %s: %w", name, err))
}

// Mockable setup for permission checking, secret generation and hashing, and timing

// supportActionHandler is an interface hiding the details of permission checking
// secret generation and hashing, and timeing from the store. This makes these
// operations mockable for store testing.
type supportActionHandler interface {
	Now() string
	MakeAndHashSecret() (string, string, error)
	UserHasPermission(verb string, token *ext.Token, ctx extcore.Context) (bool, error)
	UserHasManageTokenPermissions(ctx extcore.Context) (bool, error)
}

func NewSupportActionHandler() supportActionHandler {
	return &tokenSupport{}
}

// tokenSupport is an implementation of the supportActionHandler interface.
type tokenSupport struct{}

// Now returns the current time as a RFC 3339 formatted string.
func (tp *tokenSupport) Now() string {
	return time.Now().Format(time.RFC3339)
}

// MakeAndHashSecret creates a token secret, hashes it, and returns both secret and hash.
func (tp *tokenSupport) MakeAndHashSecret() (string, string, error) {
	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return "", "", apierrors.NewInternalError(fmt.Errorf("failed to generate token value: %w", err))
	}
	hashedValue, err := hashers.GetHasher().CreateHash(tokenValue)
	if err != nil {
		return "", "", apierrors.NewInternalError(fmt.Errorf("failed to hash token value: %w", err))
	}

	return tokenValue, hashedValue, nil
}

// UserHasPermission determines if the user (see context) requesting the
// operation (verb) is allowed to work with the token. Either because it owns
// the token, or has the `manage-token` verb/permission.
func (tp *tokenSupport) UserHasPermission(verb string, token *ext.Token, ctx extcore.Context) (bool, error) {
	// requesting user matches token user - this is ok
	if ctx.User.GetName() == token.Spec.UserID {
		return true, nil
	}
	// requesting user tries to work on/with token for other user. that requires posession of
	// the manage-token verb.
	hasPermission, err := tp.UserHasManageTokenPermissions(ctx)
	if err != nil {
		return false, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}
	if !hasPermission {
		return false, apierrors.NewForbidden(
			ctx.GroupVersionResource.GroupResource(),
			token.Name,
			fmt.Errorf("cannot %s '%s'-owned token, user '%s' does not have manage-tokens permission",
				verb, token.Spec.UserID, ctx.User.GetName()))
	}
	return true, nil
}

// UserHasManageTokenPermissions determiens if the user (see context) requesting
// an operation has the `manage-token` verb/permission.
func (tp *tokenSupport) UserHasManageTokenPermissions(ctx extcore.Context) (bool, error) {
	decision, _, err := ctx.Authorizer.Authorize(ctx, authorizer.AttributesRecord{
		User:            ctx.User,
		Verb:            "manage-token",
		Resource:        "tokens",
		ResourceRequest: true,
		APIGroup:        "ext.cattle.io",
	})
	if err != nil {
		return false, err
	}

	return decision == authorizer.DecisionAllow, nil
}

// Internal supporting functionality

// secretFromToken converts the token argument into the equivalent secrets to
// store in k8s.
func secretFromToken(token *ext.Token) *corev1.Secret {
	// inject default on creation
	ttl := token.Spec.TTL
	if ttl == 0 {
		ttl = ThirtyDays
		// pass back to caller (Create)
		token.Spec.TTL = ttl
	}

	// extend labels for future filtering of tokens by user
	labels := token.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[UserIDLabel] = token.Spec.UserID

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   TokenNamespace,
			Name:        token.Name,
			Labels:      labels,
			Annotations: token.Annotations,
		},
		StringData: make(map[string]string),
		Data:       make(map[string][]byte),
	}

	// system
	secret.StringData[fieldUID] = string(token.ObjectMeta.UID)

	// spec
	secret.StringData[fieldUserID] = token.Spec.UserID
	secret.StringData[fieldClusterName] = token.Spec.ClusterName
	secret.StringData[fieldTTL] = fmt.Sprintf("%d", ttl)
	secret.StringData[fieldEnabled] = fmt.Sprintf("%t", token.Spec.Enabled)
	secret.StringData[fieldDescription] = token.Spec.Description
	secret.StringData[fieldIsLogin] = fmt.Sprintf("%t", token.Spec.IsLogin)

	lastUsedAsString := ""
	if token.Status.LastUsedAt != nil {
		lastUsedAsString = token.Status.LastUsedAt.Format(time.RFC3339)
	}

	// status
	secret.StringData[fieldHash] = token.Status.TokenHash
	secret.StringData[fieldLastUpdateTime] = token.Status.LastUpdateTime
	secret.StringData[fieldLastUsedAt] = lastUsedAsString

	// FUTURE ((USER ACTIVITY)) change as required by the field type
	// secret.StringData[fieldIdleTimeout] = fmt.Sprintf("%d", token.Status.IdleTimeout)

	// Note:
	// - While the derived expiration data is not stored, the user-related information is.
	// - The expiration data is computed trivially from spec and resource data.
	// - Getting the user-related information on the other hand is expensive.
	// - It is better to cache it in the backing secret

	secret.StringData[fieldAuthProvider] = token.Status.AuthProvider
	secret.StringData[fieldDisplayName] = token.Status.DisplayName
	secret.StringData[fieldLoginName] = token.Status.LoginName
	secret.StringData[fieldPrincipalID] = token.Status.PrincipalID

	return secret
}

// tokenFromSecret converts the secret argument (retrieved from the k8s store)
// into the equivalent token.
func tokenFromSecret(secret *corev1.Secret) (*ext.Token, error) {
	// Basic result. This will be incrementally filled as data is extracted from the secret.
	// On error a partially filled token is returned.
	// See the token store `Delete` (marker **) for where this is important.
	token := &ext.Token{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Token",
			APIVersion: "ext.cattle.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			CreationTimestamp: secret.CreationTimestamp,
			Labels:            secret.Labels,
			Annotations:       secret.Annotations,
		},
	}

	token.Spec.Description = string(secret.Data[fieldDescription])
	token.Spec.ClusterName = string(secret.Data[fieldClusterName])
	token.Status.DisplayName = string(secret.Data[fieldDisplayName])
	token.Status.LoginName = string(secret.Data[fieldLoginName])

	userId := string(secret.Data[fieldUserID])
	if userId == "" {
		return token, fmt.Errorf("user id missing")
	}
	token.Spec.UserID = userId

	// spec
	enabled, err := strconv.ParseBool(string(secret.Data[fieldEnabled]))
	if err != nil {
		return token, err
	}
	token.Spec.Enabled = enabled

	isLogin, err := strconv.ParseBool(string(secret.Data[fieldIsLogin]))
	if err != nil {
		return token, err
	}
	token.Spec.IsLogin = isLogin

	ttl, err := strconv.ParseInt(string(secret.Data[fieldTTL]), 10, 64)
	if err != nil {
		return token, err
	}
	// inject default on retrieval
	if ttl == 0 {
		ttl = ThirtyDays
	}
	token.Spec.TTL = ttl

	// FUTURE ((USER ACTIVITY)) change as required by the field type
	//
	// BEWARE. Depending on releases made this code may have to handle
	// rancher instances containing ext tokens without idle information,
	// without crashing. I.e. insert a default when the information is not
	// present, instead of returning an error.
	//
	// idle, err := strconv.ParseInt(string(secret.Data[fieldIdleTimeout]), 10, 64)
	// if err != nil {
	// 	return token, err
	// }
	// token.Status.IdleTimeout = idle

	tokenHash := string(secret.Data[fieldHash])
	if tokenHash == "" {
		return token, fmt.Errorf("token hash missing")
	}
	token.Status.TokenHash = tokenHash

	authProvider := string(secret.Data[fieldAuthProvider])
	if authProvider == "" {
		return token, fmt.Errorf("auth provider missing")
	}
	token.Status.AuthProvider = authProvider

	lastUpdateTime := string(secret.Data[fieldLastUpdateTime])
	if lastUpdateTime == "" {
		return token, fmt.Errorf("last update time missing")
	}
	token.Status.LastUpdateTime = lastUpdateTime

	// The principal id is the object name of the virtual v3.Principal
	// resource and is therefore a required data element. display and login
	// name on the other hand are optional.
	principalID := string(secret.Data[fieldPrincipalID])
	if principalID == "" {
		return token, fmt.Errorf("principal id missing")
	}
	token.Status.PrincipalID = principalID

	kubeUID := string(secret.Data[fieldUID])
	if kubeUID == "" {
		return token, fmt.Errorf("kube uid missing")
	}
	token.ObjectMeta.UID = types.UID(kubeUID)

	var lastUsedAt *metav1.Time
	lastUsedAsString := string(secret.Data[fieldLastUsedAt])
	if lastUsedAsString != "" {
		lastUsed, err := time.Parse(time.RFC3339, lastUsedAsString)
		if err != nil {
			return token, fmt.Errorf("failed to parse lastUsed data: %w", err)
		}
		lastUsedTime := metav1.NewTime(lastUsed)
		lastUsedAt = &lastUsedTime
	} // else: empty => lastUsedAt == nil
	token.Status.LastUsedAt = lastUsedAt

	if err := setExpired(token); err != nil {
		return token, fmt.Errorf("failed to set expiration information: %w", err)
	}

	return token, nil
}

// setExpired computes the expiration data (isExpired, expiresAt) from token
// creation time and time to live and places the results into the associated
// token fields.
func setExpired(token *ext.Token) error {
	if token.Spec.TTL < 0 {
		token.Status.Expired = false
		token.Status.ExpiresAt = ""
		return nil
	}

	expiresAt := token.ObjectMeta.CreationTimestamp.Add(time.Duration(token.Spec.TTL) * time.Millisecond)
	isExpired := time.Now().After(expiresAt)

	eAt, err := metav1.NewTime(expiresAt).MarshalJSON()
	if err != nil {
		return err
	}

	// note: The marshalling puts quotes around the string. strip them
	// before handing this to the token and yaml adding another layer
	// of quotes around such a string
	token.Status.ExpiresAt = string(eAt[1 : len(eAt)-1])
	token.Status.Expired = isExpired
	return nil
}
