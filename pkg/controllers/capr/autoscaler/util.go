package autoscaler

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// autoscalerUserName generates the autoscaler-specific name for a cluster
func autoscalerUserName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Name, "autoscaler")
}

// globalRoleName generates the global role name for a cluster
func globalRoleName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Name, "autoscaler", "global", "role")
}

// globalRoleBindingName generates the global role binding name for a cluster
func globalRoleBindingName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Name, "autoscaler", "global", "rolebinding")
}

// kubeconfigSecretName generates the autoscaler kubeconfig secret name for a v2provClusterClient
func kubeconfigSecretName(cluster *capi.Cluster) string {
	return name.SafeConcatName(cluster.Name, "autoscaler", "kubeconfig")
}

func helmOpName(cluster *capi.Cluster) string {
	return name.SafeConcatName("autoscaler", cluster.Name)
}

// ownerReference creates an owner reference from a cluster
func ownerReference(cluster *capi.Cluster) []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         cluster.APIVersion,
		Kind:               cluster.Kind,
		Name:               cluster.Name,
		UID:                cluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{true}[0],
	}}
}

func generateKubeconfig(token string) ([]byte, error) {
	// Update the kubeconfig data with new token
	serverURL, cacert := settings.InternalServerURL.Get(), settings.CACerts.Get()

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
