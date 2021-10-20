package machinenodelookup

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	ClusterNameLabel = "rke.cattle.io/cluster-name"
	planSecret       = "rke.cattle.io/plan-secret-name"
	roleLabel        = "rke.cattle.io/service-account-role"
	roleBootstrap    = "bootstrap"
	rolePlan         = "plan"

	nodeErrorEnqueueTime = 15 * time.Second
)

var (
	bootstrapAPIVersion = fmt.Sprintf("%s/%s", rkev1.SchemeGroupVersion.Group, rkev1.SchemeGroupVersion.Version)
)

type handler struct {
	rancherClusterCache ranchercontrollers.ClusterCache
	machineCache        capicontrollers.MachineCache
	machines            capicontrollers.MachineController
	rkeBootstrap        rkecontroller.RKEBootstrapController
	kubeconfigManager   *kubeconfig.Manager
	dynamic             *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		machines:            clients.CAPI.Machine(),
		machineCache:        clients.CAPI.Machine().Cache(),
		rkeBootstrap:        clients.RKE.RKEBootstrap(),
		kubeconfigManager:   kubeconfig.New(clients),
		dynamic:             clients.Dynamic,
	}

	clients.RKE.RKEBootstrap().OnChange(ctx, "machine-node-lookup", h.associateMachineWithNode)
}

func (h *handler) getMachine(obj *rkev1.RKEBootstrap) (*capi.Machine, error) {
	for _, ref := range obj.OwnerReferences {
		gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)
		if capi.GroupVersion.Group != gvk.Group ||
			ref.Kind != "Machine" {
			continue
		}

		return h.machineCache.Get(obj.Namespace, ref.Name)
	}
	return nil, generic.ErrSkip
}

func (h *handler) associateMachineWithNode(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	if bootstrap == nil || bootstrap.DeletionTimestamp != nil {
		return bootstrap, nil
	}

	if !bootstrap.Status.Ready || bootstrap.Status.DataSecretName == nil || *bootstrap.Status.DataSecretName == "" {
		return bootstrap, nil
	}

	machine, err := h.getMachine(bootstrap)
	if err != nil {
		return bootstrap, err
	}

	if machine.Spec.ProviderID != nil && *machine.Spec.ProviderID != "" {
		// If the machine already has its provider ID set, then we do not need to continue
		return bootstrap, nil
	}

	rancherCluster, err := h.rancherClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if err != nil {
		return bootstrap, err
	}

	config, err := h.kubeconfigManager.GetRESTConfig(rancherCluster, rancherCluster.Status)
	if err != nil {
		return bootstrap, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return bootstrap, err
	}

	nodeLabelSelector := metav1.LabelSelector{MatchLabels: map[string]string{planner.MachineUIDLabel: string(machine.GetUID())}}
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(nodeLabelSelector.MatchLabels).String()})
	if err != nil || len(nodes.Items) == 0 || nodes.Items[0].Spec.ProviderID == "" {
		logrus.Debugf("Searching for providerID for selector %s in cluster %s/%s, machine %s: %v",
			labels.Set(nodeLabelSelector.MatchLabels), rancherCluster.Namespace, rancherCluster.Name, machine.Name, err)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, nodeErrorEnqueueTime)
		return bootstrap, nil
	}

	return bootstrap, h.updateMachine(&nodes.Items[0], machine, rancherCluster)
}

func (h *handler) updateMachineJoinURL(node *corev1.Node, machine *capi.Machine, rancherCluster *v1.Cluster) error {
	address := ""
	for _, nodeAddress := range node.Status.Addresses {
		switch nodeAddress.Type {
		case corev1.NodeInternalIP:
			address = nodeAddress.Address
		case corev1.NodeExternalIP:
			if address == "" {
				address = nodeAddress.Address
			}
		}
	}

	url := fmt.Sprintf("https://%s:%d", address, runtime.GetRuntimeSupervisorPort(rancherCluster.Spec.KubernetesVersion))
	if machine.Annotations[planner.JoinURLAnnotation] == url {
		return nil
	}

	machine = machine.DeepCopy()
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}

	machine.Annotations[planner.JoinURLAnnotation] = url
	_, err := h.machines.Update(machine)
	return err
}

func (h *handler) updateMachine(node *corev1.Node, machine *capi.Machine, rancherCluster *v1.Cluster) error {
	if err := h.updateMachineJoinURL(node, machine, rancherCluster); err != nil {
		return err
	}

	gvk := schema.FromAPIVersionAndKind(machine.Spec.InfrastructureRef.APIVersion, machine.Spec.InfrastructureRef.Kind)
	infra, err := h.dynamic.Get(gvk, machine.Namespace, machine.Spec.InfrastructureRef.Name)
	if apierror.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	d, err := data.Convert(infra)
	if err != nil {
		return err
	}

	if d.String("spec", "providerID") != node.Spec.ProviderID {
		obj, err := data.Convert(infra.DeepCopyObject())
		if err != nil {
			return err
		}

		obj.SetNested(node.Status.Addresses, "status", "addresses")
		newObj, err := h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: obj,
		})
		if err != nil {
			return err
		}

		obj, err = data.Convert(newObj)
		if err != nil {
			return err
		}

		obj.SetNested(node.Spec.ProviderID, "spec", "providerID")
		_, err = h.dynamic.Update(&unstructured.Unstructured{
			Object: obj,
		})
		return err
	}

	return nil
}
