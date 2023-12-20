package machinenodelookup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

const (
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

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
	h := &handler{
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		machines:            clients.CAPI.Machine(),
		machineCache:        clients.CAPI.Machine().Cache(),
		rkeBootstrap:        clients.RKE.RKEBootstrap(),
		kubeconfigManager:   kubeconfigManager,
		dynamic:             clients.Dynamic,
	}

	clients.RKE.RKEBootstrap().OnChange(ctx, "machine-node-lookup", h.associateMachineWithNode)
}

// associateMachineWithNode back-populates the provider ID and addresses from the K8s v1 Node object onto the
// corresponding infrastructure machine object that is referenced by the bootstrap.
func (h *handler) associateMachineWithNode(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	if bootstrap == nil || bootstrap.DeletionTimestamp != nil {
		return bootstrap, nil
	}

	if !bootstrap.Status.Ready || bootstrap.Status.DataSecretName == nil || *bootstrap.Status.DataSecretName == "" {
		return bootstrap, nil
	}

	machine, err := capr.GetMachineByOwner(h.machineCache, bootstrap)
	if err != nil {
		if errors.Is(err, capr.ErrNoMachineOwnerRef) {
			return bootstrap, generic.ErrSkip
		}
		return bootstrap, err
	}

	if machine.Spec.ProviderID != nil && *machine.Spec.ProviderID != "" {
		// If the machine already has its provider ID set, then we do not need to continue
		return bootstrap, nil
	}

	gvk := schema.FromAPIVersionAndKind(machine.Spec.InfrastructureRef.APIVersion, machine.Spec.InfrastructureRef.Kind)
	infra, err := h.dynamic.Get(gvk, machine.Namespace, machine.Spec.InfrastructureRef.Name)
	if apierror.IsNotFound(err) {
		return bootstrap, nil
	} else if err != nil {
		return bootstrap, err
	}

	d, err := data.Convert(infra)
	if err != nil {
		return bootstrap, err
	}

	// Do not mutate the infrastructure machine object if it is not marked as Ready, otherwise it will cause the
	// controller to potentially re-run the provision job
	if c := getCondition(d, "Ready"); c == nil || (c != nil && strings.ToLower(c.Status()) != "true") {
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, nodeErrorEnqueueTime)
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

	nodeLabelSelector := metav1.LabelSelector{MatchLabels: map[string]string{capr.MachineUIDLabel: string(machine.GetUID())}}
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(nodeLabelSelector.MatchLabels).String()})
	if err != nil || len(nodes.Items) == 0 || nodes.Items[0].Spec.ProviderID == "" || !condition.Cond("Ready").IsTrue(nodes.Items[0]) {
		logrus.Debugf("Searching for providerID for selector %s in cluster %s/%s, machine %s: %v",
			labels.Set(nodeLabelSelector.MatchLabels), rancherCluster.Namespace, rancherCluster.Name, machine.Name, err)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, nodeErrorEnqueueTime)
		return bootstrap, nil
	}

	node := &nodes.Items[0]

	if d.String("spec", "providerID") != node.Spec.ProviderID {
		obj, err := data.Convert(infra.DeepCopyObject())
		if err != nil {
			return bootstrap, err
		}

		obj.SetNested(node.Status.Addresses, "status", "addresses")
		newObj, err := h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: obj,
		})
		if err != nil {
			return bootstrap, err
		}

		obj, err = data.Convert(newObj)
		if err != nil {
			return bootstrap, err
		}

		obj.SetNested(node.Spec.ProviderID, "spec", "providerID")
		_, err = h.dynamic.Update(&unstructured.Unstructured{
			Object: obj,
		})
		return bootstrap, err
	}

	return bootstrap, nil
}

func getCondition(d data.Object, conditionType string) *summary.Condition {
	for _, cond := range summary.GetUnstructuredConditions(d) {
		if cond.Type() == conditionType {
			return &cond
		}
	}

	return nil
}
