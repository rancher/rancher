package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"strconv"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/ext/resources/types"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	authzv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

const TokenNamespace = "cattle-tokens"
const ThirtyDays = 30*24*60*60*100 // 30 days in milliseconds.

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type TokenStore struct {
	secretClient v1.SecretClient
	secretCache  v1.SecretCache
	sar          authzv1.SubjectAccessReviewInterface
}

func NewTokenStore(secretClient v1.SecretClient, secretCache v1.SecretCache, sar authzv1.SubjectAccessReviewInterface) types.Store[*Token, *TokenList] {
	tokenStore := TokenStore{
		secretClient: secretClient,
		secretCache:  secretCache,
		sar:          sar,
	}
	return &tokenStore
}

func (t *TokenStore) Create(ctx context.Context, userInfo user.Info, token *Token, opts *metav1.CreateOptions) (*Token, error) {
	if _, err := t.checkAdmin("create", token, userInfo); err != nil {
		return nil, err
	}
	// reject user-provided token values
	if token.Status.TokenValue != "" {
		return nil, fmt.Errorf("User provided token value is not permitted")
	}

	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return nil, fmt.Errorf("unable to generate token value: %w", err)
	}
	token.Status.TokenValue = tokenValue
	hashedValue, err := hashers.GetHasher().CreateHash(tokenValue)
	if err != nil {
		return nil, fmt.Errorf("unable to hash token value: %w", err)
	}
	token.Status.TokenHash = hashedValue

	secret, err := secretFromToken(token)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal secret for token: %w", err)
	}

	// Save new secret
	newSecret, err := t.secretClient.Create(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to create secret for token: %w", err)
	}

	// Read changes back to return what was truly created, not what we thought we created
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to regenerate token %s: %w", token.Name, err)
	}

	// users don't care about the hashed value
	newToken.Status.TokenHash = ""
	return newToken, nil
}

func (t *TokenStore) Update(ctx context.Context, userInfo user.Info, token *Token, opts *metav1.UpdateOptions) (*Token, error) {
	isadmin, err := t.checkAdmin("update", token, userInfo)
	if err != nil {
		return nil, err
	}

	currentSecret, err := t.secretCache.Get(TokenNamespace, token.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get current token %s: %w", token.Name, err)
	}
	currentToken, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token %s: %w", token.Name, err)
	}

	if token.Spec.UserID != currentToken.Spec.UserID {
		return nil, fmt.Errorf ("unable to change token %s: forbidden to edit user id", token.Name)
	}
	if token.Spec.ClusterName != currentToken.Spec.ClusterName {
		return nil, fmt.Errorf ("unable to change token %s: forbidden to edit cluster name", token.Name)
	}
	if token.Spec.IsDerived != currentToken.Spec.IsDerived {
		return nil, fmt.Errorf ("unable to change token %s: forbidden to edit flag is-derived", token.Name)
	}
	if !isadmin && (token.Spec.TTL > currentToken.Spec.TTL) {
		return nil, fmt.Errorf ("unable to change token %s: non-admin forbidden to extend time-to-live", token.Name)
	}

	// Keep status, never store the token value
	token.Status = currentToken.Status
	token.Status.TokenValue = ""

	// Save changes to backing secret
	secret, err := secretFromToken(token)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal secret for token: %w", err)
	}
	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to update token %s: %w", token.Name, err)
	}

	// Read changes back to return what was truly saved, not what we thought we saved
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to regenerate token %s: %w", token.Name, err)
	}

	newToken.Status.TokenValue = ""
	return newToken, nil
}

func (t *TokenStore) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (*Token, error) {
	// have to get token first before we can check permissions on user mismatch

	token, err := t.GetCore (name, opts)
	if err != nil {
		return nil, err
	}
	if _, err := t.checkAdmin("get", token, userInfo); err != nil {
		return nil, err
	}

	return token, nil
}

func (t *TokenStore) GetCore(name string, opts *metav1.GetOptions) (*Token, error) {
	// Core token retrieval from backing secrets
	currentSecret, err := t.secretCache.Get(TokenNamespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("unable to get token secret %s: %w", name, err)
	}
	token, err := tokenFromSecret(currentSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to extract token %s: %w", name, err)
	}
	token.Status.TokenValue = ""
	return token, nil
}

func (t *TokenStore) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (*TokenList, error) {
	// cannot use checkAdmin here. we have lots of tokens to check, with the same admin value.
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}
	secrets, err := t.secretClient.List(TokenNamespace, *opts)
	if err != nil {
		return nil, fmt.Errorf("unable to list tokens: %w", err)
	}
	var tokens []Token
	for _, secret := range secrets.Items {
		token, err := tokenFromSecret(&secret)
		if err != nil {
			// ignore tokens with broken information
			continue
		}
		// users can only list their own tokens, unless they have full permissions on this group
		if !isadmin && token.Spec.UserID != userInfo.GetName() {
			continue
		}
		tokens = append(tokens, *token)
	}
	list := TokenList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: secrets.ResourceVersion,
		},
		Items: tokens,
	}
	return &list, nil
}

// TODO: Close channel
func (t *TokenStore) Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan types.WatchEvent[*Token], error) {
	// cannot use checkAdmin here. we have lots of tokens to check, with the same admin value.
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}

	ch := make(chan types.WatchEvent[*Token])

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
			if !isadmin && token.Spec.UserID != userInfo.GetName() {
				// ignore tokens belonging to other users if this user does not have full perms
				continue
			}

			watchEvent := types.WatchEvent[*Token]{
				Event:  event.Type,
				Object: token,
			}
			ch <- watchEvent
		}
	}()

	return ch, nil
}

func (t *TokenStore) Delete(ctx context.Context, userInfo user.Info, name string, opts *metav1.DeleteOptions) error {
	// have to pull the token information first before we can check permissions
	secret, err := t.secretCache.Get(TokenNamespace, name)
	if err != nil {
		return fmt.Errorf("unable to confirm secret existence %s: %w", name, err)
	}

	// ignore errors here, only in the conversion of the non-string fields.
	// user id needed for the admin check will be ok.
	token, _ := tokenFromSecret(secret)
	if _, err := t.checkAdmin("delete", token, userInfo); err != nil {
		return err
	}
	err = t.secretClient.Delete(TokenNamespace, name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete secret %s: %w", name, err)
	}
	return nil
}

var _ types.TableConvertor[*TokenList] = (*TokenStore)(nil)

func (t *TokenStore) ConvertToTable(list *TokenList, opts *metav1.TableOptions) *metav1.Table {
	table := &metav1.Table{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Table",
			APIVersion: "meta.k8s.io/v1",
		},
		ListMeta: list.ListMeta,
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{
				Name: "Name",
				Type: "string",
			},
			{
				Name: "Enabled",
				Type: "boolean",
			},
			{
				Name: "Cluster",
				Type: "string",
			},
			{
				Name:   "Age",
				Type:   "string",
				Format: "date",
			},
		},
	}

	for _, item := range list.Items {
		row := metav1.TableRow{
			Cells: []any{
				item.Name,
				item.Spec.Enabled,
				item.Spec.ClusterName,
				item.CreationTimestamp,
			},
		}
		table.Rows = append(table.Rows, row)
	}

	return table
}

func (t *TokenStore) userHasFullPermissions(user user.Info) (bool, error) {
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   user.GetName(),
			Groups: user.GetGroups(),
			UID:    user.GetUID(),
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "*",
				Resource: "tokens",
				Group:    "ext.cattle.io",
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	sar, err := t.sar.Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("uanble to create SAR: %w", err)
	}
	return sar.Status.Allowed, nil
}

// Internal supporting functionality

func setExpired(token *Token) error {
	if token.Spec.TTL < 0 {
		token.Status.Expired = false
		token.Status.ExpiredAt = ""
		return nil
	}

	expiresAt := token.ObjectMeta.CreationTimestamp.Add (time.Duration(token.Spec.TTL)*time.Millisecond)
	isExpired := time.Now().After(expiresAt)

	eAt, err := metav1.NewTime(expiresAt).MarshalJSON()
	if err != nil {
		return err
	}

	token.Status.Expired = isExpired
	token.Status.ExpiredAt = string(eAt)
	return nil
}

func(t *TokenStore) checkAdmin(verb string, token *Token, userInfo user.Info) (bool, error) {
	if token.Spec.UserID == userInfo.GetName() {
		return false, nil
	}
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return false, fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}
	if !isadmin {
		return false, fmt.Errorf("cannot %s token for other user %s since user %s does not have full permissons on tokens", verb, userInfo.GetName(), token.Spec.UserID)
	}
	return true, nil
}

func secretFromToken(token *Token) (*corev1.Secret, error) {

	// UserPrincipal, GroupPrincipals, ProviderInfo
	// Encode the complex data into JSON for storage as string.

	gps, err := json.Marshal(token.Status.GroupPrincipals)
	if err != nil {
		return nil, err
	}
	up, err := json.Marshal(token.Status.UserPrincipal)
	if err != nil {
		return nil, err
	}
	pi, err := json.Marshal(token.Status.ProviderInfo)
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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   TokenNamespace,
			Name:        token.Name,
			Labels:      token.Labels,
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
	secret.StringData["is-derived"] = fmt.Sprintf("%t", token.Spec.IsDerived)

	// status
	secret.StringData["hash"] = token.Status.TokenHash
	secret.StringData["last-update-time"] = token.Status.LastUpdateTime

	// derived data - store ?
	// note: expiratation information is not stored

	secret.StringData["auth-provider"] = token.Status.AuthProvider
	secret.StringData["user-principal"] = string(up)
	secret.StringData["group-principals"] = string(gps)
	secret.StringData["provider-info"] = string(pi)

	return secret, nil
}

func tokenFromSecret(secret *corev1.Secret) (*Token, error) {
	// spec
	enabled, err :=	strconv.ParseBool(string(secret.Data["enabled"]))
	if err != nil {
		return nil, err
	}
	derived, err :=	strconv.ParseBool(string(secret.Data["is-derived"]))
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

	// status
	var up apiv3.Principal
	err = json.Unmarshal(secret.Data["user-principals"], &up)
	if err != nil {
		return nil, err
	}
	var gps []apiv3.Principal
	err = json.Unmarshal(secret.Data["group-principals"], &gps)
	if err != nil {
		return nil, err
	}
	var pi map[string]string
	err = json.Unmarshal(secret.Data["provider-info"], &pi)
	if err != nil {
		return nil, err
	}

	token := &Token{
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
		Spec: TokenSpec{
			UserID:      string(secret.Data["userID"]),
			Description: string(secret.Data["description"]),
			ClusterName: string(secret.Data["clusterName"]),
			TTL:         ttl,
			Enabled:     enabled,
			IsDerived:   derived,
		},
		Status: TokenStatus{
			TokenHash:       string(secret.Data["hashedToken"]),
			AuthProvider:    string(secret.Data["auth-provider"]),
			UserPrincipal:   up,
			GroupPrincipals: gps,
			ProviderInfo:    pi,
			LastUpdateTime:  string(secret.Data["last-update-time"]),
		},
	}

	if err := setExpired(token); err != nil {
		return nil, fmt.Errorf("unable to set expiration information: %w", err)
	}

	return token, nil
}
