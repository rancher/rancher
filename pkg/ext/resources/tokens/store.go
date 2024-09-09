package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"strconv"

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

// +k8s:openapi-gen=false
// +k8s:deepcopy-gen=false
type TokenStore struct {
	secretClient v1.SecretClient
	secretCache  v1.SecretCache
	sar          authzv1.SubjectAccessReviewInterface
}

func NewTokenStore(secretClient v1.SecretClient, secretCache v1.SecretCache, sar authzv1.SubjectAccessReviewInterface) types.Store[*RancherToken, *RancherTokenList] {
	tokenStore := TokenStore{
		secretClient: secretClient,
		secretCache:  secretCache,
		sar:          sar,
	}
	return &tokenStore
}

func (t *TokenStore) Create(ctx context.Context, userInfo user.Info, token *RancherToken, opts *metav1.CreateOptions) (*RancherToken, error) {
	if err := t.checkAdmin("create", token, userInfo); err != nil {
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
	_, err = t.secretClient.Create(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to create secret for token: %w", err)
	}
	// users don't care about the hashed value
	token.Status.TokenHash = ""
	return token, nil
}

func (t *TokenStore) Update(ctx context.Context, userInfo user.Info, token *RancherToken, opts *metav1.UpdateOptions) (*RancherToken, error) {
	if err := t.checkAdmin("update", token, userInfo); err != nil {
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

	token.Status.TokenHash = currentToken.Status.TokenHash
	token.Status.TokenValue = ""
	secret, err := secretFromToken(token)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal secret for token: %w", err)
	}
	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to update token %s: %w", token.Name, err)
	}
	newToken, err := tokenFromSecret(newSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to regenerate token %s: %w", token.Name, err)
	}

	newToken.Status.TokenHash = ""
	newToken.Status.TokenValue = ""

	return newToken, nil
}

func (t *TokenStore) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (*RancherToken, error) {
	// have to get token first before we can check permissions on user mismatch
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
	if err := t.checkAdmin("get", token, userInfo); err != nil {
		return nil, err
	}
	token.Status.TokenValue = ""
	return token, nil
}

func (t *TokenStore) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (*RancherTokenList, error) {
	// cannot use checkAdmin here. we have lots of tokens to check, with the same admin value.
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}
	secrets, err := t.secretClient.List(TokenNamespace, *opts)
	if err != nil {
		return nil, fmt.Errorf("unable to list tokens: %w", err)
	}
	var tokens []RancherToken
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
	list := RancherTokenList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: secrets.ResourceVersion,
		},
		Items: tokens,
	}
	return &list, nil
}

// TODO: Close channel
func (t *TokenStore) Watch(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (<-chan types.WatchEvent[*RancherToken], error) {
	// cannot use checkAdmin here. we have lots of tokens to check, with the same admin value.
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}

	ch := make(chan types.WatchEvent[*RancherToken])

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

			watchEvent := types.WatchEvent[*RancherToken]{
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
	if err := t.checkAdmin("delete", token, userInfo); err != nil {
		return err
	}
	err = t.secretClient.Delete(TokenNamespace, name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete secret %s: %w", name, err)
	}
	return nil
}

var _ types.TableConvertor[*RancherTokenList] = (*TokenStore)(nil)

func (t *TokenStore) ConvertToTable(list *RancherTokenList, opts *metav1.TableOptions) *metav1.Table {
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

func(t *TokenStore) checkAdmin(verb string, token *RancherToken, userInfo user.Info) error {
	if token.Spec.UserID == userInfo.GetName() {
		return nil
	}
	isadmin, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return fmt.Errorf("unable to check if user has full permissions on tokens: %w", err)
	}
	if !isadmin {
		return fmt.Errorf("cannot %s token for other user %s since user %s does not have full permissons on tokens", verb, userInfo.GetName(), token.Spec.UserID)
	}
	return nil
}

func secretFromToken(token *RancherToken) (*corev1.Secret, error) {

	// UserPrincipal (future), GroupPrincipals, ProviderInfo
	// Encode the complex data into JSON for storage as string.

	gps, err := json.Marshal(token.Status.GroupPrincipals)
	if err != nil {
		return nil, err
	}
	pi, err := json.Marshal(token.Status.ProviderInfo)
	if err != nil {
		return nil, err
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

	secret.StringData["userID"] = token.Spec.UserID
	secret.StringData["clusterName"] = token.Spec.ClusterName
	secret.StringData["ttl"] = token.Spec.TTL
	secret.StringData["enabled"] = fmt.Sprintf("%t", token.Spec.Enabled)
	secret.StringData["description"] = token.Spec.Description
	secret.StringData["is-derived"] = fmt.Sprintf("%t", token.Spec.IsDerived)

	secret.StringData["hash"] = token.Status.TokenHash
	secret.StringData["expired"] = fmt.Sprintf("%t", token.Status.Expired)
	secret.StringData["expired-at"] = token.Status.ExpiredAt
	secret.StringData["auth-provider"] = token.Status.AuthProvider
	secret.StringData["user-principal"] = token.Status.UserPrincipal
	secret.StringData["group-principals"] = string(gps)
	secret.StringData["provider-info"] = string(pi)
	secret.StringData["last-update-time"] = token.Status.LastUpdateTime

	return secret, nil
}

func tokenFromSecret(secret *corev1.Secret) (*RancherToken, error) {

	enabled, err :=	strconv.ParseBool(string(secret.Data["enabled"]))
	if err != nil {
		return nil, err
	}
	derived, err :=	strconv.ParseBool(string(secret.Data["is-derived"]))
	if err != nil {
		return nil, err
	}
	expired, err := strconv.ParseBool(string(secret.Data["expired"]))
	if err != nil {
		return nil, err
	}
	var gps []string
	err = json.Unmarshal(secret.Data["group-principals"], &gps)
	if err != nil {
		return nil, err
	}
	var pi map[string]string
	err = json.Unmarshal(secret.Data["provider-info"], &pi)
	if err != nil {
		return nil, err
	}

	token := &RancherToken{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RancherToken",
			APIVersion: "ext.cattle.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			CreationTimestamp: secret.CreationTimestamp,
			Labels:            secret.Labels,
			Annotations:       secret.Annotations,
		},
		Spec: RancherTokenSpec{
			UserID:      string(secret.Data["userID"]),
			Description: string(secret.Data["description"]),
			ClusterName: string(secret.Data["clusterName"]),
			TTL:         string(secret.Data["ttl"]),
			Enabled:     enabled,
			IsDerived:   derived,
		},
		Status: RancherTokenStatus{
			TokenHash:       string(secret.Data["hashedToken"]),
			Expired:         expired,
			ExpiredAt:       string(secret.Data["expired-at"]),
			AuthProvider:    string(secret.Data["auth-provider"]),
			UserPrincipal:   string(secret.Data["user-principal"]),
			GroupPrincipals: gps,
			ProviderInfo:    pi,
			LastUpdateTime:  string(secret.Data["last-update-time"]),
		},
	}

	return token, nil
}
