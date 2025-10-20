package autoscaler

import (
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// autoscalerUserName generates the autoscaler-specific name for a cluster
func autoscalerUserName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Namespace, cluster.Name, "autoscaler")
}

// globalRoleName generates the global role name for a cluster
func globalRoleName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Namespace, cluster.Name, "autoscaler", "global", "role")
}

// globalRoleBindingName generates the global role binding name for a cluster
func globalRoleBindingName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Namespace, cluster.Name, "autoscaler", "global", "rolebinding")
}

// kubeconfigSecretName generates the autoscaler kubeconfig secret name for a v2provClusterClient
func kubeconfigSecretName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Namespace, cluster.Name, "autoscaler", "kubeconfig")
}

func helmOpName(cluster *capi.Cluster) string {
	return name.SafeConcatName("autoscaler", cluster.Namespace, cluster.Name)
}

// ownerReference creates an owner reference from a cluster
func ownerReference(cluster *capi.Cluster) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         cluster.APIVersion,
		Kind:               cluster.Kind,
		Name:               cluster.Name,
		UID:                cluster.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}}
}

// generateKubeconfig generates a kubeconfig YAML string using the provided token and server URL settings.
func generateKubeconfig(token string) ([]byte, error) {
	// Update the kubeconfig data with new token
	serverURL, cacert := settings.ServerURL.Get(), settings.CACerts.Get()

	data, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:                   fmt.Sprintf("%s/k8s/clusters/local", serverURL),
				CertificateAuthorityData: []byte(strings.TrimSpace(cacert)),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				Token: token,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		CurrentContext: "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate kubeconfig data: %w", err)
	}

	return data, nil
}

// generateToken generates a v3.Token for a user in a specified cluster with given ownership references.
func generateToken(username, clusterName string, owner []metav1.OwnerReference) (*v3.Token, error) {
	// Generate new token value
	tokenValue, err := randomtoken.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token key: %w", err)
	}

	token := &v3.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
			Labels: map[string]string{
				tokens.UserIDLabel:    username,
				tokens.TokenKindLabel: "autoscaler",
				capi.ClusterNameLabel: clusterName,
			},
			Annotations:     map[string]string{},
			OwnerReferences: owner,
		},
		UserID:       username,
		AuthProvider: "local",
		IsDerived:    true,
		Token:        tokenValue,
		TTLMillis:    autoscalerTokenTTL.Milliseconds(),
	}

	if features.TokenHashing.Enabled() {
		err := tokens.ConvertTokenKeyToHash(token)
		if err != nil {
			return nil, fmt.Errorf("unable to hash token: %w", err)
		}
	}
	return token, nil
}
