package common

import (
	"fmt"
	"time"

	clusterv3 "github.com/rancher/rancher/pkg/types/apis/cluster.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewClusterAuthToken(token *managementv3.Token) (*clusterv3.ClusterAuthToken, error) {
	hash, err := CreateHash(token.Token)
	if err != nil {
		return nil, err
	}

	tokenEnabled := token.Enabled == nil || *token.Enabled
	result := &clusterv3.ClusterAuthToken{
		ObjectMeta: metav1.ObjectMeta{
			Name: token.ObjectMeta.Name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterAuthToken",
		},
		UserName:      token.UserID,
		SecretKeyHash: hash,
		ExpiresAt:     token.ExpiresAt,
		Enabled:       tokenEnabled,
	}
	return result, nil
}

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

	return VerifyHash(clusterAuthToken.SecretKeyHash, secretKey)
}
