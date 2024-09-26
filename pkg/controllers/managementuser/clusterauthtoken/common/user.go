package common

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/auth/tokens/hashers"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewClusterAuthToken creates a new cluster auth token from a given token and it's hashed value.
// Does not create the token in the remote cluster.
func NewClusterAuthToken(token *managementv3.Token, hashedValue string) (*clusterv3.ClusterAuthToken, error) {
	tokenEnabled := token.Enabled == nil || *token.Enabled
	result := &clusterv3.ClusterAuthToken{
		ObjectMeta: metav1.ObjectMeta{
			Name: token.ObjectMeta.Name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterAuthToken",
		},
		UserName:      token.UserID,
		SecretKeyHash: hashedValue,
		ExpiresAt:     token.ExpiresAt,
		Enabled:       tokenEnabled,
	}
	return result, nil
}

// VerifyClusterAuthToken verifies that a provided secret key is valid for the given clusterAuthToken.
func VerifyClusterAuthToken(secretKey string, clusterAuthToken *clusterv3.ClusterAuthToken) error {
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
	hasher, err := hashers.GetHasherForHash(clusterAuthToken.SecretKeyHash)
	if err != nil {
		return fmt.Errorf("unable to get hasher for clusterAuthToken %s/%s, err: %w", clusterAuthToken.Name, clusterAuthToken.Namespace, err)
	}
	return hasher.VerifyHash(clusterAuthToken.SecretKeyHash, secretKey)
}
