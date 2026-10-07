package clusterregistrationtoken

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretGetter is an interface for retrieving Secrets by namespace and name.
// It is satisfied by both wrangler's SecretCache and Norman's SecretLister.
type SecretGetter interface {
	Get(namespace, name string) (*corev1.Secret, error)
}

const (
	secretPrefix         = "crt-token-"
	tokenDataKey         = "token"
	previousTokenDataKey = "previousToken"
	// expiresAt and gracePeriodExpiresAt keys store the rotation schedule next to the
	// token it governs. Required to stay idempotent against duplicate rotation triggers.
	expiresAtDataKey            = "expiresAt"
	gracePeriodExpiresAtDataKey = "gracePeriodExpiresAt"
)

func SecretName(crtName string) string {
	return secretPrefix + crtName
}
func IsTokenSecret(secret *corev1.Secret) bool {
	return secret != nil && strings.HasPrefix(secret.Name, secretPrefix)
}

// SecretTokenIndexValues returns the plaintext token values stored in a CRT
// token secret, for use as index keys. Returns the current token and, if
// present, the previous token (valid during a rotation grace period).
// Returns nil for non-CRT secrets or secrets with no current token.
func SecretTokenIndexValues(secret *corev1.Secret) []string {
	if !IsTokenSecret(secret) {
		return nil
	}
	token, ok := secret.Data[tokenDataKey]
	if !ok || len(token) == 0 {
		return nil
	}
	values := []string{string(token)}
	if prev, ok := secret.Data[previousTokenDataKey]; ok && len(prev) > 0 {
		values = append(values, string(prev))
	}
	return values
}

// TokenSecretCluster returns the cluster that the token stored in secret registers with: the cluster named
// by the secret's namespace. It is read once, and callers must act on the returned object rather than look
// the cluster up again, which could find a cluster that has since replaced it under the same name.
//
// stale is true for a token left behind by a previous cluster with the same name. A cluster's token
// secrets live in a namespace named after the cluster, which is deleted with it, and the cluster is only
// removed once that namespace is gone. Nothing new can be created in a terminating namespace, so any token
// found in one belongs to a cluster that is gone.
//
// A namespace that can't be found is not treated as stale: a secret can't outlive its namespace, so not
// finding it only means the namespace cache is behind the secret cache. Rejecting then would turn a lagging
// cache into failed registrations. If no cluster has the name, the returned cluster is nil and the token
// isn't stale; callers decide whether they can act without a cluster.
func TokenSecretCluster(secret *corev1.Secret, getNamespace func(name string) (*corev1.Namespace, error), getCluster func(name string) (*v3.Cluster, error)) (cluster *v3.Cluster, stale bool, err error) {
	ns, err := getNamespace(secret.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, fmt.Errorf("failed to get namespace %s of token secret %s: %w", secret.Namespace, secret.Name, err)
	}
	if err == nil && (ns.DeletionTimestamp != nil || ns.Status.Phase == corev1.NamespaceTerminating) {
		return nil, true, nil
	}

	cluster, err = getCluster(secret.Namespace)
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get cluster %s of token secret %s: %w", secret.Namespace, secret.Name, err)
	}
	return cluster, false, nil
}

func GetTokenFromSecret(secrets SecretGetter, crt *v3.ClusterRegistrationToken) (string, error) {
	if crt == nil {
		return "", nil
	}

	if crt.Status.TokenSecretName == "" {
		return "", nil
	}

	secret, err := secrets.Get(crt.Namespace, crt.Status.TokenSecretName)
	if err != nil {
		return "", fmt.Errorf("failed to get token secret %s/%s: %w", crt.Namespace, crt.Status.TokenSecretName, err)
	}

	token, ok := secret.Data[tokenDataKey]
	if !ok {
		return "", fmt.Errorf("token key not found in secret %s/%s", crt.Namespace, crt.Status.TokenSecretName)
	}

	return string(token), nil
}

// NewTokenSecret creates a Secret object for storing a CRT's plaintext token.
func NewTokenSecret(crt *v3.ClusterRegistrationToken, token string, expiresAt string) *corev1.Secret {
	data := map[string][]byte{
		tokenDataKey: []byte(token),
	}

	if expiresAt != "" {
		data[expiresAtDataKey] = []byte(expiresAt)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(crt.Name),
			Namespace: crt.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "management.cattle.io/v3",
					Kind:       "ClusterRegistrationToken",
					Name:       crt.Name,
					UID:        crt.UID,
				},
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}
