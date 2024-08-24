package tokens

import (
	"context"
	"fmt"
	"time"

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

const TokenNamespace = "cattle-token-data"

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
	if token.Status.PlaintextToken == "" {
		tokenValue, err := randomtoken.Generate()
		if err != nil {
			return nil, fmt.Errorf("unable to generate token value: %w", err)
		}
		token.Status.PlaintextToken = tokenValue
		hashedValue, err := hashers.GetHasher().CreateHash(tokenValue)
		if err != nil {
			return nil, fmt.Errorf("unable to hash token value: %w", err)
		}
		token.Status.HashedToken = hashedValue
	}
	// user is trying to create a token for another user
	if token.Spec.UserID != userInfo.GetName() {
		authed, err := t.userHasFullPermissions(userInfo)
		if err != nil {
			return nil, fmt.Errorf("unable to check if user has * on tokens: %w", err)
		}
		if !authed {
			return nil, fmt.Errorf("can't create token for other user %s since user %s doesn't have * on ranchertokens", userInfo.GetName(), token.Spec.UserID)
		}
	}
	secret := secretFromToken(token)
	_, err := t.secretClient.Create(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to create secret for token: %w", err)
	}
	// users don't care about the hashed value
	token.Status.HashedToken = ""
	return token, nil
}

func (t *TokenStore) Update(ctx context.Context, userInfo user.Info, token *RancherToken, opts *metav1.UpdateOptions) (*RancherToken, error) {
	// user is trying to update a token for another user
	if token.Spec.UserID != userInfo.GetName() {
		authed, err := t.userHasFullPermissions(userInfo)
		if err != nil {
			return nil, fmt.Errorf("unable to check if user has * on tokens: %w", err)
		}
		if !authed {
			return nil, fmt.Errorf("can't create token for other user %s since user %s doesn't have * on ranchertokens", userInfo.GetName(), token.Spec.UserID)
		}
	}
	currentSecret, err := t.secretCache.Get(TokenNamespace, token.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get current token %s: %w", token.Name, err)
	}
	currentToken := tokenFromSecret(currentSecret)
	token.Status.HashedToken = currentToken.Status.HashedToken
	token.Status.PlaintextToken = ""
	secret := secretFromToken(token)
	newSecret, err := t.secretClient.Update(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to update token %s: %w", token.Name, err)
	}
	newToken := tokenFromSecret(newSecret)
	newToken.Status.HashedToken = ""
	newToken.Status.PlaintextToken = ""

	return newToken, nil
}

func (t *TokenStore) Get(ctx context.Context, userInfo user.Info, name string, opts *metav1.GetOptions) (*RancherToken, error) {
	currentSecret, err := t.secretCache.Get(TokenNamespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("unable to get token %s: %w", name, err)
	}
	token := tokenFromSecret(currentSecret)
	// user is trying to get a token for another user
	if token.Spec.UserID != userInfo.GetName() {
		authed, err := t.userHasFullPermissions(userInfo)
		if err != nil {
			return nil, fmt.Errorf("unable to check if user has * on tokens: %w", err)
		}
		if !authed {
			return nil, fmt.Errorf("can't create token for other user %s since user %s doesn't have * on ranchertokens", userInfo.GetName(), token.Spec.UserID)
		}
	}
	token.Status.PlaintextToken = ""
	return token, nil
}

func (t *TokenStore) List(ctx context.Context, userInfo user.Info, opts *metav1.ListOptions) (*RancherTokenList, error) {
	secrets, err := t.secretClient.List(TokenNamespace, *opts)
	if err != nil {
		return nil, fmt.Errorf("unable to list tokens: %w", err)
	}
	authed, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has * on tokens: %w", err)
	}
	var tokens []RancherToken
	for _, secret := range secrets.Items {
		token := tokenFromSecret(&secret)
		// users can only list their own tokens, unless they have * on this group
		if !authed && token.Spec.UserID != userInfo.GetName() {
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
	authed, err := t.userHasFullPermissions(userInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to check if user has * on tokens: %w", err)
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

			token := tokenFromSecret(secret)
			if !authed && token.Spec.UserID != userInfo.GetName() {
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
	fmt.Println(userInfo, name)
	secret, err := t.secretCache.Get(TokenNamespace, name)
	if err != nil {
		return fmt.Errorf("unable to confirm secret existence %s: %w", name, err)
	}
	token := tokenFromSecret(secret)
	// user is trying to get a token for another user
	if token.Spec.UserID != userInfo.GetName() {
		authed, err := t.userHasFullPermissions(userInfo)
		if err != nil {
			return fmt.Errorf("unable to check if user has * on tokens: %w", err)
		}
		if !authed {
			return fmt.Errorf("can't create token for other user %s since user %s doesn't have * on ranchertokens", userInfo.GetName(), token.Spec.UserID)
		}
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
				Resource: "ranchertokens",
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

func secretFromToken(token *RancherToken) *corev1.Secret {
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
	secret.StringData["enabled"] = token.Spec.Enabled
	secret.StringData["hashedToken"] = token.Status.HashedToken
	return secret
}

func tokenFromSecret(secret *corev1.Secret) *RancherToken {
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
			ClusterName: string(secret.Data["clusterName"]),
			TTL:         string(secret.Data["ttl"]),
			Enabled:     string(secret.Data["enabled"]),
		},
		Status: RancherTokenStatus{
			HashedToken: string(secret.Data["hashedToken"]),
		},
	}
	return token
}
