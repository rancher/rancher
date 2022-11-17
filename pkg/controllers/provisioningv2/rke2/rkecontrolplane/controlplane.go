package rkecontrolplane

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		clusterCache:            clients.Mgmt.Cluster().Cache(),
		provClusterCache:        clients.Provisioning.Cluster().Cache(),
		rkeControlPlane:         clients.RKE.RKEControlPlane(),
		machineDeploymentClient: clients.CAPI.MachineDeployment(),
		machineDeploymentCache:  clients.CAPI.MachineDeployment().Cache(),
		machineCache:            clients.CAPI.Machine().Cache(),
		machineClient:           clients.CAPI.Machine(),
		capiClusters:            clients.CAPI.Cluster().Cache(),
	}

	rkecontrollers.RegisterRKEControlPlaneStatusHandler(ctx, clients.RKE.RKEControlPlane(), "", "rke-control-plane", h.OnChange)
	relatedresource.Watch(ctx, "rke-control-plane-trigger", h.clusterWatch, clients.RKE.RKEControlPlane(), clients.Mgmt.Cluster())

	clients.RKE.RKEControlPlane().OnRemove(ctx, "rke-control-plane-remove", h.OnRemove)
}

type handler struct {
	clusterCache            mgmtcontrollers.ClusterCache
	provClusterCache        provcontrollers.ClusterCache
	rkeControlPlane         rkecontrollers.RKEControlPlaneController
	machineDeploymentClient capicontrollers.MachineDeploymentClient
	machineDeploymentCache  capicontrollers.MachineDeploymentCache
	machineCache            capicontrollers.MachineCache
	machineClient           capicontrollers.MachineClient
	capiClusters            capicontrollers.ClusterCache
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

func (h *handler) OnChange(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	capiCluster, err := rke2.FindOwnerCAPICluster(cp, h.capiClusters)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster does not exist", cp.Namespace, cp.Name)
			h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
			return status, generic.ErrSkip
		}
		logrus.Errorf("[rkecontrolplane] %s/%s: error getting CAPI cluster %v", cp.Namespace, cp.Name, err)
		return status, err
	}

	if capiCluster == nil {
		logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster does not exist", cp.Namespace, cp.Name)
		h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
		return status, generic.ErrSkip
	}

	if capiannotations.IsPaused(capiCluster, cp) {
		logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster or RKEControlPlane is paused", cp.Namespace, cp.Name)
		h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
		return status, generic.ErrSkip
	}

	cluster, err := h.clusterCache.Get(cp.Spec.ManagementClusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("[rkecontrolplane] %s/%s: waiting: Management cluster does not exist", cp.Namespace, cp.Name)
			h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
			return status, generic.ErrSkip
		}
		return status, err
	}

	status.Ready = rke2.Ready.IsTrue(cluster)
	status.Initialized = rke2.Ready.IsTrue(cluster)
	status.AgentConnected = clusterconnected.Connected.IsTrue(cluster)

	return status, nil
}

func (h *handler) OnRemove(_ string, cp *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
	logrus.Debugf("[rkecontrolplane] %s/%s: Peforming removal of rkecontrolplane", cp.Namespace, cp.Name)

	cp = cp.DeepCopy()

	capiCluster, err := rke2.FindOwnerCAPICluster(cp, h.capiClusters)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster does not exist", cp.Namespace, cp.Name)
			h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
			return cp, generic.ErrSkip
		}
		logrus.Errorf("[rkecontrolplane] %s/%s: error getting CAPI cluster %v", cp.Namespace, cp.Name, err)
		return cp, err
	}

	if capiCluster == nil {
		logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster does not exist", cp.Namespace, cp.Name)
		h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
		return cp, generic.ErrSkip
	}

	if capiannotations.IsPaused(capiCluster, cp) {
		logrus.Infof("[rkecontrolplane] %s/%s: waiting: CAPI cluster or RKEControlPlane is paused", cp.Namespace, cp.Name)
		h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
		return cp, generic.ErrSkip
	}

	// Some machines may not have gotten the CAPI cluster-name label in previous versions in Rancher.
	// Because of update issues with the conversion webhook in rancher-webhook, we can't use a "migration" to add the label (it will fail because the conversion webhook is not available).
	// In addition, there is no way to "or" label selectors in the API, so we need to do this manually.
	otherMachines, err := h.machineCache.List(cp.Namespace, labels.SelectorFromSet(labels.Set{rke2.ClusterNameLabel: cp.Spec.ClusterName, rke2.ControlPlaneRoleLabel: "true"}))
	if err != nil {
		return cp, err
	}

	// Control plane nodes are managed by the control plane object. Therefore, the control plane object shouldn't be cleaned up before the control plane nodes are removed.
	machines, err := h.machineCache.List(cp.Namespace, labels.SelectorFromSet(labels.Set{capi.ClusterLabelName: cp.Spec.ClusterName, rke2.ControlPlaneRoleLabel: "true"}))
	if err != nil {
		return cp, err
	}

	machineSet := map[string]struct{}{}
	for _, m := range machines {
		machineSet[m.Name] = struct{}{}
	}

	for _, m := range otherMachines {
		if _, ok := machineSet[m.Name]; !ok {
			machines = append(machines, m)
		}
	}

	logrus.Debugf("[rkecontrolplane] %s/%s: listed %d machines during removal", cp.Namespace, cp.Name, len(machines))
	logrus.Tracef("[rkecontrolplane] %s/%s: machine list: %+v", cp.Namespace, cp.Name, machines)

	for _, machine := range machines {
		if machine.DeletionTimestamp == nil {
			err = h.machineClient.Delete(machine.Namespace, machine.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return cp, fmt.Errorf("error deleting machine %s/%s: %v", machine.Namespace, machine.Name, err)
			}
		}
	}

	sort.Slice(machines, func(i, j int) bool {
		return machines[i].Name < machines[j].Name
	})
	if len(machines) == 0 {
		return cp, nil
	}

	status := cp.Status

	var msg string
	msg, err = rke2.GetMachineDeletionStatus(machines)
	if err != nil {
		rke2.Removed.SetError(&status, "", err)
	} else if msg != "" {
		err = generic.ErrSkip
		rke2.Removed.SetStatus(&status, "Unknown")
		rke2.Removed.Reason(&status, "Waiting")
		rke2.Removed.Message(&status, msg)
	}

	if err != nil && reflect.DeepEqual(cp.Status, status) {
		h.rkeControlPlane.EnqueueAfter(cp.Namespace, cp.Name, 5*time.Second)
		return cp, err
	}

	var err2 error
	cp, err2 = h.rkeControlPlane.UpdateStatus(cp)
	if err2 != nil {
		return cp, err2
	}

	return cp, err
}
