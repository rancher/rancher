package common

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewClusterAuthToken creates a new cluster auth token from a given token.
// Does not create the token in the remote cluster.
func NewClusterAuthToken(token *managementv3.Token) *clusterv3.ClusterAuthToken {
	tokenEnabled := token.Enabled == nil || *token.Enabled
	return &clusterv3.ClusterAuthToken{
		ObjectMeta: metav1.ObjectMeta{
			Name: token.ObjectMeta.Name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterAuthToken",
		},
		UserName:  token.UserID,
		ExpiresAt: token.ExpiresAt,
		Enabled:   tokenEnabled,
	}
}

// NewClusterAuthSecret creates a new secret from the given token and its hash value
// The cluster auth token is managed separately.
// Does not create the secret in the remote cluster.
func NewClusterAuthTokenSecret(token *managementv3.Token, hashedValue string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterAuthTokenSecretName(token.ObjectMeta.Name),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.Version,
			Kind:       "Secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"hash": []byte(hashedValue),
		},
	}
}

// ClusterAuthTokenSecretName builds the name for the cluster auth token's secret.
func ClusterAuthTokenSecretName(tokenName string) string {
	return tokenName
}

// ClusterAuthSecretValue extracts the token hash stored in the secret.
func ClusterAuthTokenSecretValue(clusterAuthSecret *corev1.Secret) string {
	return string(clusterAuthSecret.Data["hash"])
}

// VerifyClusterAuthToken verifies that a provided secret key is valid for the
// given clusterAuthToken and hashed value.
func VerifyClusterAuthToken(secretKey string, clusterAuthToken *clusterv3.ClusterAuthToken, clusterAuthSecret *corev1.Secret) error {
	if !clusterAuthToken.Enabled {
		return fmt.Errorf("token is not enabled")
	}

	expiresAt := clusterAuthToken.ExpiresAt
	if expiresAt != "" {
		expires, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return err
		}
		if expires.Before(time.Now()) {
			return fmt.Errorf("auth expired at %s", expiresAt)
		}
	}

	hashedValue := clusterAuthToken.SecretKeyHash
	if clusterAuthSecret != nil {
		hashedValue = ClusterAuthTokenSecretValue(clusterAuthSecret)
	}
	hasher, err := hashers.GetHasherForHash(hashedValue)
	if err != nil {
		return fmt.Errorf("unable to get hasher for clusterAuthToken %s/%s, err: %w", clusterAuthToken.Name, clusterAuthToken.Namespace, err)
	}
	return hasher.VerifyHash(hashedValue, secretKey)
}
