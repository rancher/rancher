package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// startTokenRenewal begins the background token renewal process.
// It starts a goroutine that periodically checks for expiring tokens and renews them.
// The renewal check runs every 24 hours and renews tokens that expire within 7 days.
func (h *autoscalerHandler) startTokenRenewal(ctx context.Context) {
	go func() {
		for range ticker.Context(ctx, renewalCheckInterval) {
			if err := h.checkAndRenewTokens(); err != nil {
				logrus.Errorf("[autoscaler] Failed to check and renew token: %v", err)
			}
		}
	}()
}

// checkAndRenewTokens finds and renews expiring autoscaler tokens.
// It lists all tokens with the "autoscaler" label, checks their expiration dates,
// and renews any tokens that are within the renewal threshold (7 days from now).
// Returns an error if the token listing or renewal process fails.
func (h *autoscalerHandler) checkAndRenewTokens() error {
	tokens, err := h.findAutoscalerTokens()
	if err != nil {
		return err
	}

	expiringCount := 0

	processingErrs := make([]error, 0, len(tokens))

	for _, token := range tokens {
		if token.TTLMillis == 0 {
			continue // Skip non-expiring token
		}

		expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err != nil {
			logrus.Warnf("[autoscaler] invalid token expires at on token %v: %v", token.Name, err)
			processingErrs = append(processingErrs, err)
		}

		if expiresAt.Before(time.Now().Add(renewalThreshold)) {
			expiringCount++
			if err := h.renewToken(&token); err != nil {
				logrus.Errorf("[autoscaler] Failed to renew token %s: %v", token.Name, err)
				processingErrs = append(processingErrs, err)
			}
		}
	}

	logrus.Infof("[autoscaler] Processed %d expiring autoscaler tokens, renewed %d", len(tokens), expiringCount)
	return errors.Join(processingErrs...)
}

// findAutoscalerTokens retrieves all tokens created by the autoscaler controller.
// It uses a label selector to filter tokens with the "autoscaler" token kind.
// Returns a slice of tokens or an error if the listing operation fails.
func (h *autoscalerHandler) findAutoscalerTokens() ([]v3.Token, error) {
	tokenList, err := h.tokenClient.List(metav1.ListOptions{LabelSelector: autoscalerTokenSelector})
	if err != nil {
		return nil, fmt.Errorf("failed to list autoscaler token: %w", err)
	}

	return tokenList.Items, nil
}

// renewToken creates a new token to replace an expired one.
// It deletes the old token, generates a new token with the same user ID and cluster name,
// updates the associated kubeconfig secret with the new token, and ensures the
// cluster-autoscaler helm chart is redeployed with the updated kubeconfig.
// Returns an error if any step in the renewal process fails.
func (h *autoscalerHandler) renewToken(token *v3.Token) error {
	// Delete the old token
	err := h.tokenClient.Delete(token.Name, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete old token %s: %w", token.Name, err)
	}

	newToken, err := generateToken(token.UserID, token.Labels[capi.ClusterNameLabel], token.OwnerReferences)
	if err != nil {
		return err
	}

	newToken, err = h.tokenClient.Create(newToken)
	if err != nil {
		return fmt.Errorf("failed to create renewed token: %w", err)
	}

	// Update the kubeconfig secret with the new token
	clusterName := token.Labels[capi.ClusterNameLabel]

	// Find the capi cluster to update the secret
	// Since clusters are cluster-scoped, we use "" as namespace and filter by label selector
	clusters, err := h.capiClusterCache.List("", labels.SelectorFromSet(labels.Set{
		capi.ClusterNameLabel: clusterName,
	}))

	if err != nil {
		logrus.Errorf("[autoscaler] Failed to list clusters for token %s: %v", newToken.Name, err)
		return fmt.Errorf("failed to list clusters for token %s: %v", newToken.Name, err)
	} else if len(clusters) == 0 {
		logrus.Errorf("[autoscaler] No cluster found for token %s with name %s", newToken.Name, clusterName)
		return fmt.Errorf("no cluster found for token %s with name %s", newToken.Name, clusterName)
	} else if len(clusters) > 1 {
		logrus.Errorf("[autoscaler] Multiple clusters found for token %s with name %s: %d clusters found", newToken.Name, clusterName, len(clusters))
		return fmt.Errorf("multiple clusters found for token %s with name %s: %d clusters found", newToken.Name, clusterName, len(clusters))
	}

	// Use the single matching capiCluster to update the secret
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

	logrus.Infof("[autoscaler] Successfully renewed token %s and updated associated kubeconfig secret", newToken.Name)
	return nil
}

// updateKubeConfigSecretWithToken updates an existing kubeconfig secret with a new token.
// It retrieves the existing kubeconfig secret for the cluster, generates a new kubeconfig
// with the provided token, and updates both the full kubeconfig and the token field in the secret.
// Returns the updated secret or an error if the retrieval or update fails.
func (h *autoscalerHandler) updateKubeConfigSecretWithToken(cluster *capi.Cluster, token string) (*corev1.Secret, error) {
	secretName := kubeconfigSecretName(cluster)

	// Get the existing secret
	secret, err := h.secretCache.Get(cluster.Namespace, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing kubeconfig secret %s/%s: %w", cluster.Namespace, secretName, err)
	}

	data, err := generateKubeconfig(token)
	if err != nil {
		return nil, err
	}

	// Update both the full kubeconfig and the token field
	secret = secret.DeepCopy()
	secret.Data["value"] = data
	secret.Data["token"] = []byte(token)

	// Update the secret
	kubeconfig, err := h.secretClient.Update(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to update kubeconfig secret %s/%s: %w", cluster.Namespace, secretName, err)
	}

	logrus.Infof("[autoscaler] Successfully updated kubeconfig secret %s/%s with new token", cluster.Namespace, secretName)
	return kubeconfig, nil
}
