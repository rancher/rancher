package tokens

import (
	"fmt"

	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	"github.com/rancher/rancher/pkg/ext/resources/types"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const tokenNamespace = "cattle-token-data"

type TokenStore struct {
	secretClient v1.SecretClient
	secretCache  v1.SecretCache
}

func NewTokenStore(secretClient v1.SecretClient, secretCache v1.SecretCache) types.Store[*RancherToken] {
	tokenStore := TokenStore{
		secretClient: secretClient,
		secretCache:  secretCache,
	}
	return &tokenStore
}

func (t *TokenStore) Create(token *RancherToken) (*RancherToken, error) {
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
	secret := secretFromToken(token)
	_, err := t.secretClient.Create(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to create secret for token: %w", err)
	}
	// users don't care about the hashed value
	token.Status.HashedToken = ""
	return token, nil
}

func (t *TokenStore) Update(token *RancherToken) (*RancherToken, error) {
	currentSecret, err := t.secretCache.Get(token.Namespace, token.Name)
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

func (t *TokenStore) Get(name string) (*RancherToken, error) {
	currentSecret, err := t.secretCache.Get(tokenNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("unable to get token %s: %w", name, err)
	}
	token := tokenFromSecret(currentSecret)
	token.Status.HashedToken = ""
	token.Status.PlaintextToken = ""
	return nil, nil
}

func (t *TokenStore) List() ([]*RancherToken, error) {
	secrets, err := t.secretCache.List(tokenNamespace, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list tokens: %w", err)
	}
	var tokens []*RancherToken
	for _, secret := range secrets {
		tokens = append(tokens, tokenFromSecret(secret))
	}
	return tokens, nil
}

func (t *TokenStore) Delete(name string) error {
	_, err := t.secretCache.Get(tokenNamespace, name)
	if err != nil {
		return fmt.Errorf("unable to confirm secret existence %s: %w", name, err)
	}
	err = t.secretClient.Delete(tokenNamespace, name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete secret %s: %w", name, err)
	}
	return nil
}

func secretFromToken(token *RancherToken) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tokenNamespace,
			Name:      token.Name,
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
		ObjectMeta: metav1.ObjectMeta{
			Name: secret.Name,
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
