package rkecontrolplane

import (
	"context"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache:              clients.Mgmt.Cluster().Cache(),
		provClusterCache:          clients.Provisioning.Cluster().Cache(),
		rkeControlPlaneController: clients.RKE.RKEControlPlane(),
		machineDeploymentClient:   clients.CAPI.MachineDeployment(),
		machineDeploymentCache:    clients.CAPI.MachineDeployment().Cache(),
		machineCache:              clients.CAPI.Machine().Cache(),
		machineClient:             clients.CAPI.Machine(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "rke-control-plane", h.OnChange)
	relatedresource.Watch(ctx, "rke-control-plane-trigger", h.clusterWatch, clients.RKE.RKEControlPlane(), clients.Mgmt.Cluster())

	clients.RKE.RKEControlPlane().OnRemove(ctx, "rke-control-plane-remove", h.OnRemove)
}

type handler struct {
	clusterCache              mgmtcontrollers.ClusterCache
	provClusterCache          provcontrollers.ClusterCache
	rkeControlPlaneController rkecontrollers.RKEControlPlaneController
	machineDeploymentClient   capicontrollers.MachineDeploymentClient
	machineDeploymentCache    capicontrollers.MachineDeploymentCache
	machineCache              capicontrollers.MachineCache
	machineClient             capicontrollers.MachineClient
}

func (h *handler) clusterWatch(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return nil, nil
	}

	provClusters, err := h.provClusterCache.GetByIndex(provcluster.ByCluster, cluster.Name)
	if err != nil || len(provClusters) == 0 {
		return nil, err
	}
	return []relatedresource.Key{
		{
			Namespace: provClusters[0].Namespace,
			Name:      provClusters[0].Name,
		},
	}, nil
}

func (h *handler) OnChange(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	status.ObservedGeneration = obj.Generation
	cluster, err := h.clusterCache.Get(obj.Spec.ManagementClusterName)
	if err != nil {
		h.rkeControlPlaneController.EnqueueAfter(obj.Namespace, obj.Name, 2*time.Second)
		return status, nil
	}

	status.AgentConnected = clusterconnected.Connected.IsTrue(cluster)
	return status, nil
}

func (h *handler) OnRemove(_ string, cp *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
	status := cp.Status
	cp = cp.DeepCopy()

	err := capr.DoRemoveAndUpdateStatus(cp, h.doRemove(cp), h.rkeControlPlaneController.EnqueueAfter)

	if equality.Semantic.DeepEqual(status, cp.Status) {
		return cp, err
	}
	cp, updateErr := h.rkeControlPlaneController.UpdateStatus(cp)
	if updateErr != nil {
		return cp, updateErr
	}

	return cp, err
}

func (h *handler) doRemove(cp *rkev1.RKEControlPlane) func() (string, error) {
	return func() (string, error) {
		logrus.Debugf("[rkecontrolplane] (%s/%s) Peforming removal of rkecontrolplane", cp.Namespace, cp.Name)
		// Control plane nodes are managed by the control plane object. Therefore, the control plane object shouldn't be cleaned up before the control plane nodes are removed.
		machines, err := h.machineCache.List(cp.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterNameLabel: cp.Name, capr.ControlPlaneRoleLabel: "true"}))
		if err != nil {
			return "", err
		}

		// Some machines may not have gotten the CAPI cluster-name label in previous versions in Rancher.
		// Because of update issues with the conversion webhook in rancher-webhook, we can't use a "migration" to add the label (it will fail because the conversion webhook is not available).
		// In addition, there is no way to "or" label selectors in the API, so we need to do this manually.
		otherMachines, err := h.machineCache.List(cp.Namespace, labels.SelectorFromSet(labels.Set{capr.ClusterNameLabel: cp.Name, capr.ControlPlaneRoleLabel: "true"}))
		if err != nil {
			return "", err
		}

		logrus.Debugf("[rkecontrolplane] (%s/%s) listed %d machines during removal", cp.Namespace, cp.Name, len(machines))
		logrus.Tracef("[rkecontrolplane] (%s/%s) machine list: %+v", cp.Namespace, cp.Name, machines)
		allMachines := append(machines, otherMachines...)

		for _, machine := range allMachines {
			// Only delete custom machines. Custom machines can be added outside the UI, so it is important to check each machine.
			if machine.Spec.InfrastructureRef.APIVersion != "rke.cattle.io/v1" || machine.Spec.InfrastructureRef.Kind != "CustomMachine" {
				continue
			}
			if machine.DeletionTimestamp == nil {
				if err = h.machineClient.Delete(machine.Namespace, machine.Name, &metav1.DeleteOptions{}); err != nil {
					return "", fmt.Errorf("error deleting machine %s/%s: %v", machine.Namespace, machine.Name, err)
				}
			}
		}

		return capr.GetMachineDeletionStatus(allMachines)
	}
}
