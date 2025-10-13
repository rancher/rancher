package autoscaler

import (
	"context"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	renewalCheckInterval = 24 * time.Hour
	renewalThreshold     = 7 * 24 * time.Hour
)

var (
	// Label selector to identify tokens created by the autoscaler controller
	autoscalerTokenSelector = labels.Set{tokens.TokenKindLabel: "autoscaler"}.AsSelector().String()
)

// startTokenRenewal begins the background token renewal process
func (h *autoscalerHandler) startTokenRenewal(ctx context.Context) {
	go func() {
		for range ticker.Context(ctx, renewalCheckInterval) {
			if err := h.checkAndRenewTokens(); err != nil {
				logrus.Errorf("[autoscaler] Failed to check and renew token: %v", err)
			}
		}
	}()
}

// checkAndRenewTokens finds and renews expiring autoscaler token
func (h *autoscalerHandler) checkAndRenewTokens() error {
	tokens, err := h.findAutoscalerTokens()
	if err != nil {
		return err
	}

	expiringCount := 0

	for _, token := range tokens {
		if token.TTLMillis == 0 {
			continue // Skip non-expiring token
		}

		expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err != nil {
			logrus.Warnf("invalid token expires at on token %v: %v", token.Name, err)
			return err
		}

		if expiresAt.Before(time.Now().Add(renewalThreshold)) {
			expiringCount++
			if err := h.renewToken(&token); err != nil {
				logrus.Errorf("[autoscaler] Failed to renew token %s: %v", token.Name, err)
			}
		}
	}

	if expiringCount > 0 {
		logrus.Infof("[autoscaler] Processed %d expiring autoscaler token", expiringCount)
	} else {
		logrus.Debugf("[autoscaler] No expiring autoscaler token found")
	}

	return nil
}

func (h *autoscalerHandler) findAutoscalerTokens() ([]v3.Token, error) {
	tokenList, err := h.token.List(metav1.ListOptions{LabelSelector: autoscalerTokenSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list autoscaler token: %w", err)
	}

	return tokenList.Items, nil
}

// renewToken creates a new token to replace an expired one
func (h *autoscalerHandler) renewToken(token *v3.Token) error {
	// Delete the old token
	err := h.token.Delete(token.Name, nil)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete old token %s: %w", token.Name, err)
	}

	newToken, err := generateToken(token.UserID, token.Labels[capi.ClusterNameLabel], token.OwnerReferences)
	if err != nil {
		return err
	}

	token, err = h.token.Create(newToken)
	if err != nil {
		return fmt.Errorf("failed to create renewed token %s: %w", token.Name, err)
	}

	if token.Labels == nil {
		logrus.Errorf("[autoscaler] Token %s is missing capi cluster label, cannot update kubeconfig secret", token.Name)
		return fmt.Errorf("token %s is missing capi cluster label, cannot update kubeconfig secret", token.Name)
	}

	// Update the kubeconfig secret with the new token

	clusterName := token.Labels[capi.ClusterNameLabel]

	// Find the capi cluster to update the secret
	// Since clusters are cluster-scoped, we use "" as namespace and filter by label selector
	clusters, err := h.capiCluster.List("", labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: clusterName,
	}))

	if err != nil {
		logrus.Errorf("[autoscaler] Failed to list clusters for token %s: %v", token.Name, err)
		return fmt.Errorf("failed to list clusters for token %s: %v", token.Name, err)
	} else if len(clusters) == 0 {
		logrus.Errorf("[autoscaler] No cluster found for token %s with name %s", token.Name, clusterName)
		return fmt.Errorf("no cluster found for token %s with name %s", token.Name, clusterName)
	}

	// Use the first matching cluster to update the secret
	capiCluster := clusters[0]
	kubeconfig, err := h.updateKubeConfigSecretWithToken(capiCluster, fmt.Sprintf("%s:%s", token.UserID, newToken.Token))
	if err != nil {
		logrus.Errorf("[autoscaler] Failed to update kubeconfig secret for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
		return fmt.Errorf("failed to update kubeconfig secret for cluster %s/%s: %v", capiCluster.Namespace, capiCluster.Name, err)
	}

	// then re-rollout the cluster-autoscaler helm chart with the new kubeconfig resource version
	err = h.ensureFleetHelmOp(capiCluster, kubeconfig.ResourceVersion, 1)
	if err != nil {
		return err
	}

	logrus.Infof("[autoscaler] Successfully renewed token %s and updated associated kubeconfig secret", token.Name)
	return nil
}

// updateKubeConfigSecretWithToken updates an existing kubeconfig secret with a new token
func (h *autoscalerHandler) updateKubeConfigSecretWithToken(cluster *capi.Cluster, token string) (*corev1.Secret, error) {
	secretName := kubeconfigSecretName(cluster)

	// Get the existing secret
	secret, err := h.secretsCache.Get(cluster.Namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing kubeconfig secret %s/%s: %w", cluster.Namespace, secretName, err)
	}

	data, err := generateKubeconfig(token)
	if err != nil {
		return nil, err
	}

	// Update both the full kubeconfig and the token field
	secret.DeepCopy()
	secret.Data["value"] = data
	secret.Data["token"] = []byte(token)

	// Update the secret
	kubeconfig, err := h.secrets.Update(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to update kubeconfig secret %s/%s: %w", cluster.Namespace, secretName, err)
	}

	logrus.Infof("[autoscaler] Successfully updated kubeconfig secret %s/%s with new token", cluster.Namespace, secretName)
	return kubeconfig, nil
}
