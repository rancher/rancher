package machineremoval

import (
	"context"
	"fmt"

	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

type handler struct {
	ctx	context.Context
	rancherClusterCache  ranchercontrollers.ClusterCache
	kubeconfigManager	 *kubeconfig.Manager
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		ctx: ctx,
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		kubeconfigManager:    kubeconfig.New(clients),
	}
	clients.CAPI.Machine().OnRemove(ctx, "machine-removal", h.OnRemove)
}

func (h *handler) OnRemove(key string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil ||
		machine.Spec.Bootstrap.ConfigRef == nil ||
		machine.Spec.Bootstrap.ConfigRef.Kind != "RKEBootstrap" {
		return machine, nil
	}
	if key != "" {
		return machine, nil // lol, preserving logic but allowing for silent removal
	}
	if machine.Status.NodeRef.Name != "" {
		// Retrieve the corresponding cluster for this machine
		cluster, err := h.rancherClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
		if err != nil {
			return machine, err
		}

		if cluster.DeletionTimestamp != nil {
			// In the event that the cluster is deleting, we don't try to coordinate
			// etcd member removal
			return machine, nil
		}

		restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)

		if err != nil {
			return machine, err // we need to do something if we can't get the kubeconfig for this.
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return machine, err
		}

		node, err := clientset.CoreV1().Nodes().Get(h.ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
		if err != nil {
			return machine, err
		}

		if _, ok := node.Labels["node-role.kubernetes.io/etcd"]; !ok {
			// Not an etcd node, so we can proceed with delete.
			return machine, nil
		}

		removeAnnotation := "etcd." + planner.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/remove"
		removedNodeNameAnnotation := "etcd." + planner.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/removed-node-name"

		if val, ok := node.Annotations[removeAnnotation]; ok {
			// check val to see if it's true, if not, continue
			if val == "true" {
				// check the status of the removal
				if removedNodeName, ok := node.Annotations[removedNodeNameAnnotation]; ok {
					// There is the possibility the annotation is defined, but empty.
					if removedNodeName != "" {
						return machine, nil // Proceed with removal.
					}
				}
			}
		}
		// The remove annotation has not been set to true, so we'll go ahead and set it on the node.
		err = retry.RetryOnConflict(retry.DefaultRetry,
			func() error {
				node.Annotations[removeAnnotation] = "true"
				node, err = clientset.CoreV1().Nodes().Update(h.ctx, node, metav1.UpdateOptions{})
				return err
			})
		if err != nil {
			// there was an error updating the node
			return machine, err
		}
		return machine, fmt.Errorf("waiting for machine to be removed from etcd cluster")
	}
	return machine, fmt.Errorf("couldn't delete machine")
}
