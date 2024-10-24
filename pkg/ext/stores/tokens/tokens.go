package tokens

import (
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	extcore "github.com/rancher/steve/pkg/ext"

	"encoding/json"
	"fmt"
	"strconv"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

const (
	TokenNamespace = "cattle-tokens"
	ThirtyDays     = 30 * 24 * 60 * 60 * 1000 // 30 days in milliseconds.
	UserIDLabel    = "authn.management.cattle.io/token-userId"
)

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// TokenStore is the interface to the token store seen by the extension API and users. Wrapped
// around a SystemTokenStore it performs the necessary checks to ensure that Users have only access
// to the tokens they are permitted to.
type TokenStore struct {
	SystemTokenStore
}

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false

// SystemTokenStore is the interface to the token store used internally by other parts of
// rancher. It does not perform any kind of permission checks, and operates with (implied) admin
// authority. IOW it has access to all the tokens, in all ways.
type SystemTokenStore struct {
	secretClient        v1.SecretClient
	userAttributeClient v3.UserAttributeClient
	userClient          v3.UserClient

	// channel to send watch events to.
	events chan extcore.WatchEvent[*ext.Token]
}

func NewTokenStoreFromWrangler(wranglerContext *wrangler.Context) extcore.Store[*ext.Token, *ext.TokenList] {
	return NewTokenStore(
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.UserAttribute(),
		wranglerContext.Mgmt.User(),
	)
}

func NewTokenStore(
	secretClient v1.SecretClient,
	uaClient v3.UserAttributeController,
	userClient v3.UserController,
) extcore.Store[*ext.Token, *ext.TokenList] {
	tokenStore := TokenStore{
		SystemTokenStore: SystemTokenStore{
			secretClient:        secretClient,
			userAttributeClient: uaClient,
			userClient:          userClient,
			events:              make(chan extcore.WatchEvent[*ext.Token], 100),
		},
	}
	return &tokenStore
}

func NewSystemTokenStoreFromWrangler(wranglerContext *wrangler.Context) *SystemTokenStore {
	return NewSystemTokenStore(
		wranglerContext.Core.Secret(),
		wranglerContext.Mgmt.UserAttribute(),
		wranglerContext.Mgmt.User(),
	)
}

func NewSystemTokenStore(
	secretClient v1.SecretClient,
	uaClient v3.UserAttributeController,
	userClient v3.UserController,
) *SystemTokenStore {
	tokenStore := SystemTokenStore{
		secretClient:        secretClient,
		userAttributeClient: uaClient,
		userClient:          userClient,
	}
	return &tokenStore
}

func (t *TokenStore) Create(ctx extcore.Context, token *ext.Token, opts *metav1.CreateOptions) (*ext.Token, error) {
	if _, err := t.checkForManageToken("create", token, ctx); err != nil {
		return nil, err
	}

	// reject creation attempting to write over existing resource
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

	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to generate token value: %w", err))
	}
	hashedValue, err := hashers.GetHasher().CreateHash(tokenValue)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to hash token value: %w", err))
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
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to get user attributes for %s",
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

	// (len == 1) => The single key in the UA map names the auth provider
	// (and where to look in GPs, if we were using GPs)
	var authProvider string
	for ap, _ := range attribs.ExtraByProvider {
		authProvider = ap
		break
	}

	token.Status.TokenHash = hashedValue
	token.Status.AuthProvider = authProvider
	token.Status.UserPrincipal.DisplayName = user.DisplayName
	token.Status.UserPrincipal.LoginName = user.Username // See also attribs.ExtraByProvider[ap]["username"][0]
	token.Status.UserPrincipal.ObjectMeta.Name = attribs.ExtraByProvider[authProvider]["principalid"][0]
	token.Status.UserPrincipal.PrincipalType = "user"
	token.Status.UserPrincipal.Provider = authProvider

	token.Status.LastUpdateTime = time.Now().Format(time.RFC3339)
	secret, err := secretFromToken(token)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to marshal token %s: %w",
			token.Name, err))
	}

	// Save new secret
	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
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
	isadmin, err := t.checkForManageToken("update", token, ctx)
	if err != nil {
		return nil, err
	}

	return t.SystemTokenStore.update(isadmin, token, opts)

}

func (t *SystemTokenStore) Update(token *ext.Token, opts *metav1.UpdateOptions) (*ext.Token, error) {
	return t.update(true, token, opts)
}

func (t *SystemTokenStore) update(isadmin bool, token *ext.Token, opts *metav1.UpdateOptions) (*ext.Token, error) {
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
	if !isadmin && (token.Spec.TTL > currentToken.Spec.TTL) {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("rejecting change of token %s: forbidden to extend time-to-live",
			token.Name))
	}

	// Keep status, never store the token value, refresh time
	token.Status = currentToken.Status
	token.Status.TokenValue = ""
	token.Status.LastUpdateTime = time.Now().Format(time.RFC3339)

	// Save changes to backing secret
	secret, err := secretFromToken(token)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to marshal token %s: %w",
			token.Name, err))
	}
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

func (t *TokenStore) Get(ctx extcore.Context, name string, opts *metav1.GetOptions) (*ext.Token, error) {
	// have to get token first before we can check permissions on user mismatch

	token, err := t.SystemTokenStore.Get(name, opts)
	if err != nil {
		return nil, err
	}
	if _, err := t.checkForManageToken("get", token, ctx); err != nil {
		return nil, err
	}

	return token, nil
}

func (t *SystemTokenStore) Get(name string, opts *metav1.GetOptions) (*ext.Token, error) {
	// Core token retrieval from backing secrets
	currentSecret, err := t.secretClient.Get(TokenNamespace, name, metav1.GetOptions{})
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
	isadmin, err := t.userHasManageTokenPermissions(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}
	return t.SystemTokenStore.list(isadmin, ctx.User.GetName(), opts)
}

func (t *SystemTokenStore) List(opts *metav1.ListOptions) (*ext.TokenList, error) {
	return t.list(true, "", opts)
}

func (t *SystemTokenStore) list(isadmin bool, user string, opts *metav1.ListOptions) (*ext.TokenList, error) {
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
		if !isadmin && token.Spec.UserID != user {
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

// TODO: Close channel
func (t *TokenStore) Watch(ctx extcore.Context, opts *metav1.ListOptions) (<-chan extcore.WatchEvent[*ext.Token], error) {
	// cannot use checkForManageToken here. we have lots of tokens to check, with the same admin value.
	isadmin, err := t.userHasManageTokenPermissions(ctx)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}

	ch := make(chan extcore.WatchEvent[*ext.Token])

	go func() {
		watcher, err := t.secretClient.Watch(TokenNamespace, metav1.ListOptions{
			ResourceVersion: opts.ResourceVersion,
		})
		if err != nil {
			return
		}
		defer watcher.Stop()

		for event := range watcher.ResultChan() {
			secret, ok := event.Object.(*corev1.Secret)
			if !ok {
				continue
			}

			token, err := tokenFromSecret(secret)
			if err != nil {
				// ignore broken tokens
				continue
			}
			if !isadmin && token.Spec.UserID != ctx.User.GetName() {
				// ignore tokens belonging to other users if this user does not have full perms
				continue
			}

			watchEvent := extcore.WatchEvent[*ext.Token]{
				Event:  event.Type,
				Object: token,
			}
			ch <- watchEvent
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

	// ignore errors here, only in the conversion of the non-string fields.
	// user id needed for the admin check will be ok.
	token, _ := tokenFromSecret(secret)
	if _, err := t.checkForManageToken("delete", token, ctx); err != nil {
		return err
	}

	err = t.secretClient.Delete(TokenNamespace, name, &metav1.DeleteOptions{})
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("failed to delete token %s: %w", name, err))
	}
	return nil
}

func (t *SystemTokenStore) Delete(name string, opts *metav1.DeleteOptions) error {
	_, err := t.secretClient.Get(TokenNamespace, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return apierrors.NewInternalError(fmt.Errorf("failed delete token %s: %w", name, err))
		}
		// not found, nothing to do
		return nil
	}

	err = t.secretClient.Delete(TokenNamespace, name, &metav1.DeleteOptions{})
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("failed delete token %s: %w", name, err))
	}
	return nil
}

func (t *TokenStore) userHasManageTokenPermissions(ctx extcore.Context) (bool, error) {
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

func (t *TokenStore) checkForManageToken(verb string, token *ext.Token, ctx extcore.Context) (bool, error) {
	// requesting user matches token user - this is ok
	if ctx.User.GetName() == token.Spec.UserID {
		return false, nil
	}
	// requesting user tries to work on/with token for other user. that requires posession of
	// the manage-token verb.
	isadmin, err := t.userHasManageTokenPermissions(ctx)
	if err != nil {
		return false, apierrors.NewInternalError(fmt.Errorf("failed to check user permissions: %w", err))
	}
	if !isadmin {
		return false, apierrors.NewForbidden(
			ctx.GroupVersionResource.GroupResource(),
			token.Name,
			fmt.Errorf("cannot %s '%s'-owned token, user '%s' does not have manage-tokens permission",
				verb, token.Spec.UserID, ctx.User.GetName()))
	}
	return true, nil
}

func secretFromToken(token *ext.Token) (*corev1.Secret, error) {
	// UserPrincipal, GroupPrincipals, ProviderInfo
	// Encode the complex data into JSON foprmatting strings for storage in the secret.
	// See also note below on why this derived information is stored.

	up, err := json.Marshal(token.Status.UserPrincipal)
	if err != nil {
		return nil, err
	}

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

	// spec
	secret.StringData["userID"] = token.Spec.UserID
	secret.StringData["clusterName"] = token.Spec.ClusterName
	secret.StringData["ttl"] = fmt.Sprintf("%d", ttl)
	secret.StringData["enabled"] = fmt.Sprintf("%t", token.Spec.Enabled)
	secret.StringData["description"] = token.Spec.Description
	secret.StringData["is-login"] = fmt.Sprintf("%t", token.Spec.IsLogin)

	// status
	secret.StringData["hash"] = token.Status.TokenHash
	secret.StringData["last-update-time"] = token.Status.LastUpdateTime

	// Note:
	// - While the derived expiration data is not stored, the user-related information is.
	// - The expiration data is computed trivially from spec and resource data.
	// - Getting the user-related information on the other hand is expensive.
	// - It is better to cache it in the backing secret

	secret.StringData["auth-provider"] = token.Status.AuthProvider
	secret.StringData["user-principal"] = string(up)

	return secret, nil
}

func tokenFromSecret(secret *corev1.Secret) (*ext.Token, error) {
	// spec
	enabled, err := strconv.ParseBool(string(secret.Data["enabled"]))
	if err != nil {
		return nil, err
	}
	isLogin, err := strconv.ParseBool(string(secret.Data["is-login"]))
	if err != nil {
		return nil, err
	}
	ttl, err := strconv.ParseInt(string(secret.Data["ttl"]), 10, 64)
	if err != nil {
		return nil, err
	}
	// inject default on retrieval
	if ttl == 0 {
		ttl = ThirtyDays
	}

	userId := string(secret.Data["userID"])
	if userId == "" {
		return nil, fmt.Errorf("userId missing")
	}

	tokenHash := string(secret.Data["hash"])
	if tokenHash == "" {
		return nil, fmt.Errorf("token hash missing")
	}

	authProvider := string(secret.Data["auth-provider"])
	if authProvider == "" {
		return nil, fmt.Errorf("auth provider missing")
	}

	lastUpdateTime := string(secret.Data["last-update-time"])
	if lastUpdateTime == "" {
		return nil, fmt.Errorf("last update time missing")
	}

	// status
	var up apiv3.Principal
	err = json.Unmarshal(secret.Data["user-principal"], &up)
	if err != nil {
		return nil, err
	}

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
		Spec: ext.TokenSpec{
			UserID:      userId,
			Description: string(secret.Data["description"]),
			ClusterName: string(secret.Data["clusterName"]),
			TTL:         ttl,
			Enabled:     enabled,
			IsLogin:     isLogin,
		},
		Status: ext.TokenStatus{
			TokenHash:      tokenHash,
			AuthProvider:   authProvider,
			UserPrincipal:  up,
			LastUpdateTime: lastUpdateTime,
		},
	}

	if err := setExpired(token); err != nil {
		return nil, fmt.Errorf("failed to set expiration information: %w", err)
	}

	return token, nil
}
