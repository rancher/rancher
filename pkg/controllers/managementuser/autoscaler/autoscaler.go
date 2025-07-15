package autoscaler

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollersv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type handler struct {
	secrets corecontrollersv1.SecretController

	downstreamPods    v1.PodInterface
	downstreamSecrets v1.SecretInterface

	clusterName string
	lastToken   string
}

func Register(ctx context.Context, clients *wrangler.CAPIContext, cluster *config.UserContext, clusterRec *apimgmtv3.Cluster) {
	// don't run this controller for the local cluster
	if cluster.ClusterName == "local" {
		return
	}

	h := handler{
		clusterName:       clusterRec.Spec.DisplayName,
		secrets:           clients.Core.Secret(),
		downstreamPods:    cluster.Core.Pods("kube-system"),
		downstreamSecrets: cluster.Core.Secrets("kube-system"),
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

	if !strings.HasSuffix(secret.Name, "autoscaler-kubeconfig") {
		return secret, nil
	}

	// we already deleted a pod for this token, don't re-restart the pod
	if h.lastToken == string(secret.Data["token"]) {
		return secret, nil
	}

	var outerErr = fmt.Errorf("timed out waiting for downstream mgmt-kubeconfig secret to be updated")
	for range 10 {
		downstreamKubeconfig, err := h.downstreamSecrets.Get("mgmt-kubeconfig", metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if downstreamKubeconfig == nil {
			continue
		}

		if bytes.Equal(downstreamKubeconfig.Data["token"], secret.Data["token"]) {
			outerErr = nil
			break
		}

		logrus.Debugf("waiting on kubeconfig to synchronize to downstream cluster")
		time.Sleep(10 * time.Second)
	}

	if outerErr != nil {
		// don't retry this token - since it seems like this would happen on clusters that are disconnected
		h.lastToken = string(secret.Data["token"])
		logrus.Warnf("failed to bounce cluster-autoscaler pod for cluster %v/%v", secret.Namespace, secret.Labels[capi.ClusterNameLabel])
		return nil, outerErr
	}

	// delete the cluster-autoscaler pod since the secret changed
	pods, err := h.downstreamPods.List(metav1.ListOptions{LabelSelector: clusterAutoscalerSelector})
	if err != nil {
		return secret, err
	}

	if len(pods.Items) < 1 {
		logrus.Debugf("no autoscaler pods found for cluster %v/%v", secret.Namespace, secret.Labels[capi.ClusterNameLabel])
		return secret, err
	}

	for _, pod := range pods.Items {
		err = h.downstreamPods.Delete(pod.Name, &metav1.DeleteOptions{})
		if err != nil {
			return nil, err
		}
	}

	h.lastToken = string(secret.Data["token"])

	return secret, nil
}
