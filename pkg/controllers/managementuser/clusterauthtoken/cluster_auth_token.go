package clusterauthtoken

import (
	"encoding/json"
	"time"

	"github.com/rancher/rancher/pkg/auth/tokens"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	clusterv3 "github.com/rancher/rancher/pkg/generated/norman/cluster.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type clusterAuthTokenHandler struct {
	tokenCache  mgmtcontrollers.TokenCache
	tokenClient mgmtcontrollers.TokenClient
}

// Sync ClusterAuthToken back to Token.
func (h *clusterAuthTokenHandler) sync(key string, clusterAuthToken *clusterv3.ClusterAuthToken) (runtime.Object, error) {
	if clusterAuthToken == nil || clusterAuthToken.DeletionTimestamp != nil {
		return nil, nil
	}

	if !clusterAuthToken.Enabled ||
		isExpired(clusterAuthToken) ||
		clusterAuthToken.LastUsedAt == nil {
		return clusterAuthToken, nil
	}

	tokenName := clusterAuthToken.Name

	token, err := h.tokenCache.Get(tokenName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("[%s] Error getting token %s: %v", clusterAuthTokenController, tokenName, err)
		}

		return clusterAuthToken, nil
	}

	if token.LastUsedAt != nil && token.LastUsedAt.After(clusterAuthToken.LastUsedAt.Time) {
		return clusterAuthToken, nil // Nothing to do.
	}

	if tokens.IsExpired(*token) {
		return clusterAuthToken, nil
	}

	if err := func() error {
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		}{{
			Op:    "replace",
			Path:  "/lastUsedAt",
			Value: clusterAuthToken.LastUsedAt,
		}})
		if err != nil {
			return err
		}

		_, err = h.tokenClient.Patch(token.Name, types.JSONPatchType, patch)
		return err
	}(); err != nil {
		// Log the error and move on to avoid failing the request.
		logrus.Errorf("[%s] Error updating lastUsedAt for token %s: %v", clusterAuthTokenController, tokenName, err)
		return clusterAuthToken, nil
	}

	logrus.Debugf("[%s] Updated lastUsedAt for token %s", clusterAuthTokenController, tokenName)

	return clusterAuthToken, nil
}

func isExpired(t *clusterv3.ClusterAuthToken) bool {
	if t.ExpiresAt == "" {
		return false
	}

	expiresAt, err := time.Parse(time.RFC3339, t.ExpiresAt)
	if err != nil {
		return false
	}

	return time.Now().After(expiresAt)
}
