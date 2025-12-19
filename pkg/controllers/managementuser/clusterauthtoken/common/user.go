package common

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewClusterAuthToken creates a new cluster auth token from a given token
// accessor and it's hashed value. It does not create the token in the remote
// cluster.
func NewClusterAuthToken(token accessor.TokenAccessor, hashedValue string) *clusterv3.ClusterAuthToken {
	return &clusterv3.ClusterAuthToken{
		ObjectMeta: metav1.ObjectMeta{
			Name: token.GetName(),
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterAuthToken",
		},
		UserName:  token.GetUserID(),
		ExpiresAt: token.GetExpiresAt(),
		Enabled:   token.GetIsEnabled(),
	}
}

// NewClusterAuthTokenSecret creates a new secret from the given token and its hash value
// The cluster auth token is managed separately.
// Does not create the secret in the remote cluster.
func NewClusterAuthTokenSecret(ns string, token accessor.TokenAccessor, hashedValue string) *corev1.Secret {
	return NewClusterAuthTokenSecretForName(ns, token.GetName(), hashedValue)
}

// NewClusterAuthTokenSecretForName creates a new secret from the given token and its hash value
// The cluster auth token is managed separately.
// Does not create the secret in the remote cluster.
func NewClusterAuthTokenSecretForName(ns, name, hashedValue string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterAuthTokenSecretName(name),
			Namespace: ns,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.Version,
			Kind:       "Secret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			ClusterAuthSecretHashField: []byte(hashedValue),
		},
	}
}

// ClusterAuthTokenSecretName builds the name for the cluster auth token's secret.
func ClusterAuthTokenSecretName(tokenName string) string {
	return tokenName
}

// ClusterAuthTokenSecretValue extracts the token hash stored in the secret.
func ClusterAuthTokenSecretValue(clusterAuthSecret *corev1.Secret) string {
	return string(clusterAuthSecret.Data[ClusterAuthSecretHashField])
}

// VerifyClusterAuthToken verifies that a provided secret key is valid for the
// given clusterAuthToken and hashed value. Also determines if the hashed value
// requires migration from cluster auth token to cluster auth token secret.
func VerifyClusterAuthToken(secretKey string, clusterAuthToken *clusterv3.ClusterAuthToken, clusterAuthTokenSecret *corev1.Secret) (error, bool) { //nolint:revive
	if !clusterAuthToken.Enabled {
		return fmt.Errorf("token is not enabled"), false
	}

	expiresAt := clusterAuthToken.ExpiresAt
	if expiresAt != "" {
		expires, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return err, false
		}
		if expires.Before(time.Now()) {
			return fmt.Errorf("auth expired at %s", expiresAt), false
		}
	}

	hashedValue := clusterAuthToken.SecretKeyHash
	migrate := true

	if hashedValue == "" {
		if clusterAuthTokenSecret == nil {
			return fmt.Errorf("hash secret is missing"), false
		}

		hashedValue = ClusterAuthTokenSecretValue(clusterAuthTokenSecret)
		migrate = false
	}

	hasher, err := hashers.GetHasherForHash(hashedValue)
	if err != nil {
		return fmt.Errorf("unable to get hasher for clusterAuthToken %s/%s, err: %w",
			clusterAuthToken.Name, clusterAuthToken.Namespace, err), false
	}

	return hasher.VerifyHash(hashedValue, secretKey), migrate
}
