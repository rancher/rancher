package rkecontrolplane

import (
	"context"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache:              clients.Mgmt.Cluster().Cache(),
		rkeControlPlaneController: clients.RKE.RKEControlPlane(),
		machineDeploymentClient:   clients.CAPI.MachineDeployment(),
		machineDeploymentCache:    clients.CAPI.MachineDeployment().Cache(),
		machineCache:              clients.CAPI.Machine().Cache(),
		machineClient:             clients.CAPI.Machine(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(),
		"", "rke-control-plane", h.OnChange)
	relatedresource.Watch(ctx, "rke-control-plane-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		return []relatedresource.Key{{
			Namespace: namespace,
			Name:      name,
		}}, nil
	}, clients.RKE.RKEControlPlane(), clients.Mgmt.Cluster())

	clients.RKE.RKEControlPlane().OnRemove(ctx, "rke-control-plane-remove", h.OnRemove)
}

type handler struct {
	clusterCache              mgmtcontrollers.ClusterCache
	rkeControlPlaneController rkecontrollers.RKEControlPlaneController
	machineDeploymentClient   capicontrollers.MachineDeploymentClient
	machineDeploymentCache    capicontrollers.MachineDeploymentCache
	machineCache              capicontrollers.MachineCache
	machineClient             capicontrollers.MachineClient
}

func (h *handler) OnChange(obj *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	status.ObservedGeneration = obj.Generation
	cluster, err := h.clusterCache.Get(obj.Spec.ManagementClusterName)
	if err != nil {
		h.rkeControlPlaneController.EnqueueAfter(obj.Namespace, obj.Name, 2*time.Second)
		return status, nil
	}

	status.Ready = rke2.Ready.IsTrue(cluster)
	status.Initialized = rke2.Ready.IsTrue(cluster)
	return status, nil
}

func (h *handler) OnRemove(_ string, cp *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
	status := cp.Status
	cp = cp.DeepCopy()
	err := rke2.DoRemoveAndUpdateStatus(cp, h.doRemove(cp), h.rkeControlPlaneController.EnqueueAfter)

	if equality.Semantic.DeepEqual(status, cp.Status) {
		return cp, err
	}
	cp, updateErr := h.rkeControlPlaneController.UpdateStatus(cp)
	if updateErr != nil {
		return cp, updateErr
	}

	return cp, err
}

func (h *handler) doRemove(obj *rkev1.RKEControlPlane) func() (string, error) {
	return func() (string, error) {
		logrus.Debugf("[rkecontrolplane] (%s/%s) Peforming removal of rkecontrolplane", obj.Namespace, obj.Name)
		// Control plane nodes are managed by the control plane object. Therefore, the control plane object shouldn't be cleaned up before the control plane nodes are removed.
		machines, err := h.machineCache.List(obj.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterLabelName: obj.Name, rke2.ControlPlaneRoleLabel: "true"}))
		if err != nil {
			return "", err
		}

		logrus.Debugf("[rkecontrolplane] (%s/%s) listed %d machines during removal", obj.Namespace, obj.Name, len(machines))
		logrus.Tracef("[rkecontrolplane] (%s/%s) machine list: %+v", obj.Namespace, obj.Name, machines)

		return rke2.GetMachineDeletionStatus(machines)
	}
}
