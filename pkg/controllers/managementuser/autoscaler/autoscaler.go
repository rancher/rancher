package autoscaler

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollersv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const autoscalerKubeconfigSecretSuffix = "autoscaler-kubeconfig"

type handler struct {
	secrets corecontrollersv1.SecretController

	downstreamDeployments appsv1.DeploymentInterface
	downstreamSecrets     v1.SecretInterface

	clusterName string
	lastToken   string
}

func Register(ctx context.Context, clients *wrangler.CAPIContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) {
	// only run these handlers if autoscaling is enabled
	if !features.ClusterAutoscaling.Enabled() {
		return
	}

	// don't run this controller for the local cluster
	if cluster.ClusterName == "local" {
		return
	}

	h := handler{
		clusterName:           clusterRec.Spec.DisplayName,
		secrets:               clients.Core.Secret(),
		downstreamDeployments: cluster.Apps.Deployments("kube-system"),
		downstreamSecrets:     cluster.Core.Secrets("kube-system"),
	}

	h.secrets.OnChange(ctx, "autoscaler-token-refresh", h.autoscalerTokenRefresh)
}

var clusterAutoscalerSelector = labels.SelectorFromSet(labels.Set{
	"app.kubernetes.io/instance": "cluster-autoscaler",
}).String()

// autoscalerTokenRefresh checks and maintains connectivity for the cluster autoscaler.
// It verifies that secret synchronization occurred and restarts autoscaler pods when needed.
// NOTE: as currently implemented this will restart _all_ autoscaler pods on bootup
func (h *handler) autoscalerTokenRefresh(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.DeletionTimestamp != nil {
		return secret, nil
	}

	if secret.Labels[capi.ClusterNameLabel] != h.clusterName {
		return secret, nil
	}

	if !strings.HasSuffix(secret.Name, autoscalerKubeconfigSecretSuffix) {
		return secret, nil
	}

	// we already deleted a pod for this token, don't re-restart the pod
	if h.lastToken == string(secret.Data["token"]) {
		return secret, nil
	}

	// fetch the downstream cluster-autoscaler deployment in order to scale it down to 0
	deployments, err := h.downstreamDeployments.List(metav1.ListOptions{LabelSelector: clusterAutoscalerSelector})
	if err != nil {
		return secret, err
	}

	// no cluster-autoscaler found in the cluster yet - might have just been installed.
	if len(deployments.Items) == 0 {
		logrus.Debugf("no autoscaler deployment found for cluster %v/%v", secret.Namespace, secret.Labels[capi.ClusterNameLabel])
		return secret, fmt.Errorf("no autoscaler deployment found for cluster %v/%v", secret.Namespace, secret.Labels[capi.ClusterNameLabel])
	}

	deploy := &deployments.Items[0]

	*deploy.Spec.Replicas = 0
	_, err = h.downstreamDeployments.Update(deploy)
	if err != nil {
		return secret, fmt.Errorf("failed to update autoscaler deployment in cluster %v/%v: %v", secret.Namespace, secret.Labels[capi.ClusterNameLabel], err)
	}

	start := time.Now()
	err = wait.ExponentialBackoff(retry.DefaultBackoff, func() (done bool, err error) {
		downstreamKubeconfig, err := h.downstreamSecrets.Get("mgmt-kubeconfig", metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return false, nil
		}
		if downstreamKubeconfig == nil {
			return false, nil
		}

		if bytes.Equal(downstreamKubeconfig.Data["token"], secret.Data["token"]) {
			return true, nil
		}

		// try for 5 minutes - if it doesn't work by then just log it and continue.
		if time.Since(start).Minutes() > 5 {
			return true, fmt.Errorf("timed out waiting for downstream mgmt-kubeconfig secret to be updated")
		}

		logrus.Debugf("waiting on kubeconfig to synchronize to downstream cluster")
		return false, nil
	})

	if err != nil {
		// don't retry this token - since it seems like this would happen on clusters that are disconnected
		h.lastToken = string(secret.Data["token"])
		logrus.Warnf("failed to bounce cluster-autoscaler pod for cluster %v/%v", secret.Namespace, secret.Labels[capi.ClusterNameLabel])
		return nil, err
	}

	// get a fresh version of the deployment and scale it back up
	deploy, err = h.downstreamDeployments.Get(deploy.Name, metav1.GetOptions{})
	if err != nil {
		return secret, err
	}

	*deploy.Spec.Replicas = 1
	_, err = h.downstreamDeployments.Update(deploy)
	if err != nil {
		return secret, err
	}

	h.lastToken = string(secret.Data["token"])
	return secret, nil
}
