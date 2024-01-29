package unmanaged

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterindex"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/data"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v2/pkg/kv"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
	h := handler{
		kubeconfigManager: kubeconfigManager,
		unmanagedMachine:  clients.RKE.CustomMachine(),
		mgmtClusterCache:  clients.Mgmt.Cluster().Cache(),
		clusterCache:      clients.Provisioning.Cluster().Cache(),
		capiClusterCache:  clients.CAPI.Cluster().Cache(),
		machineCache:      clients.CAPI.Machine().Cache(),
		machineClient:     clients.CAPI.Machine(),
		secrets:           clients.Core.Secret(),
		apply: clients.Apply.WithSetID("unmanaged-machine").
			WithCacheTypes(
				clients.Mgmt.Cluster(),
				clients.Provisioning.Cluster(),
				clients.RKE.CustomMachine(),
				clients.CAPI.Machine(),
				clients.RKE.RKEBootstrap()),
	}
	clients.RKE.CustomMachine().OnRemove(ctx, "unmanaged-machine", h.onUnmanagedMachineOnRemove)
	clients.RKE.CustomMachine().OnChange(ctx, "unmanaged-health", h.onUnmanagedMachineChange)
	clients.Core.Secret().OnChange(ctx, "unmanaged-machine-secret", h.onSecretChange)
}

type handler struct {
	kubeconfigManager *kubeconfig.Manager
	unmanagedMachine  rkecontroller.CustomMachineController
	mgmtClusterCache  mgmtcontroller.ClusterCache
	capiClusterCache  capicontrollers.ClusterCache
	machineCache      capicontrollers.MachineCache
	machineClient     capicontrollers.MachineClient
	clusterCache      rocontrollers.ClusterCache
	secrets           corecontrollers.SecretClient
	apply             apply.Apply
}

func (h *handler) findMachine(cluster *capi.Cluster, machineName, machineID string) (string, error) {
	_, err := h.machineCache.Get(cluster.Namespace, machineName)
	if apierror.IsNotFound(err) {
		machines, err := h.machineCache.List(cluster.Namespace, labels.SelectorFromSet(map[string]string{
			capr.MachineIDLabel: machineID,
		}))
		if len(machines) != 1 || err != nil || machines[0].Spec.ClusterName != cluster.Name {
			return "", err
		}
		return machines[0].Name, nil
	} else if err != nil {
		return "", err
	}

	return machineName, nil
}

func (h *handler) onSecretChange(_ string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != capr.MachineRequestType {
		return secret, nil
	}

	go func() {
		// Only keep requests for 1 minute
		time.Sleep(time.Minute)
		_ = h.secrets.Delete(secret.Namespace, secret.Name, nil)
	}()

	data := data.Object{}
	if err := json.Unmarshal(secret.Data["data"], &data); err != nil {
		// ignore invalid json, wait until it's valid
		return secret, nil
	}

	capiCluster, err := h.getCAPICluster(secret)
	if err != nil {
		return secret, err
	} else if capiCluster == nil {
		return secret, nil
	}

	machineName, err := h.findMachine(capiCluster, secret.Name, data.String("id"))
	if err != nil {
		return nil, err
	} else if machineName == "" {
		machineName = secret.Name
		if err = h.createMachine(capiCluster, secret, data); err != nil {
			return nil, err
		}
	}

	if secret.Labels[capr.MachineNamespaceLabel] != capiCluster.Namespace ||
		secret.Labels[capr.MachineNameLabel] != machineName {
		secret = secret.DeepCopy()
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[capr.MachineNamespaceLabel] = capiCluster.Namespace
		secret.Labels[capr.MachineNameLabel] = machineName

		return h.secrets.Update(secret)
	}

	return secret, nil
}

func (h *handler) createMachine(capiCluster *capi.Cluster, secret *corev1.Secret, data data.Object) error {
	objs, err := h.createMachineObjects(capiCluster, secret.Name, data)
	if err != nil {
		return err
	}
	return h.apply.WithOwner(secret).ApplyObjects(objs...)
}

func (h *handler) createMachineObjects(capiCluster *capi.Cluster, machineName string, data data.Object) ([]runtime.Object, error) {
	labels := map[string]string{}
	annotations := map[string]string{}

	if data.Bool("role-control-plane") {
		labels[capr.ControlPlaneRoleLabel] = "true"
		labels[capi.MachineControlPlaneLabel] = "true"
	}
	if data.Bool("role-etcd") {
		labels[capr.EtcdRoleLabel] = "true"
	}
	if data.Bool("role-worker") {
		labels[capr.WorkerRoleLabel] = "true"
	}
	if val := data.String("node-name"); val != "" {
		labels[capr.NodeNameLabel] = val
	}
	if address := data.String("address"); address != "" {
		annotations[capr.AddressAnnotation] = address
	}
	if internalAddress := data.String("internal-address"); internalAddress != "" {
		annotations[capr.InternalAddressAnnotation] = internalAddress
	}

	labels[capr.MachineIDLabel] = data.String("id")
	labels[capr.ClusterNameLabel] = capiCluster.Name
	labels[capi.ClusterNameLabel] = capiCluster.Name

	labelsMap := map[string]string{}
	for _, str := range strings.Split(data.String("labels"), ",") {
		k, v := kv.Split(str, "=")
		if k == "" {
			continue
		}
		labelsMap[k] = v
		if _, ok := labels[k]; !ok {
			labels[k] = v
		}
	}

	if len(labelsMap) > 0 {
		data, err := json.Marshal(labelsMap)
		if err != nil {
			return nil, err
		}
		annotations[capr.LabelsAnnotation] = string(data)
	}

	var coreTaints []corev1.Taint
	for _, taint := range data.StringSlice("taints") {
		coreTaints = append(coreTaints, taints.GetTaintsFromStrings(strings.Split(taint, ","))...)
	}

	if len(coreTaints) > 0 {
		data, err := json.Marshal(coreTaints)
		if err != nil {
			return nil, err
		}
		annotations[capr.TaintsAnnotation] = string(data)
	}

	return []runtime.Object{
		&rkev1.RKEBootstrap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        machineName,
				Namespace:   capiCluster.Namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: rkev1.RKEBootstrapSpec{
				ClusterName: capiCluster.Name,
			},
		},
		&rkev1.CustomMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: capiCluster.Namespace,
				Labels:    labels,
			},
		},
		&capi.Machine{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: capiCluster.Namespace,
				Labels:    labels,
			},
			Spec: capi.MachineSpec{
				ClusterName: capiCluster.Name,
				Bootstrap: capi.Bootstrap{
					ConfigRef: &corev1.ObjectReference{
						Kind:       "RKEBootstrap",
						Namespace:  capiCluster.Namespace,
						Name:       machineName,
						APIVersion: capr.RKEAPIVersion,
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					Kind:       "CustomMachine",
					Namespace:  capiCluster.Namespace,
					Name:       machineName,
					APIVersion: capr.RKEAPIVersion,
				},
			},
		},
	}, nil
}

func (h *handler) getCAPICluster(secret *corev1.Secret) (*capi.Cluster, error) {
	cluster, err := h.mgmtClusterCache.Get(secret.Namespace)
	if apierror.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	rClusters, err := h.clusterCache.GetByIndex(clusterindex.ClusterV1ByClusterV3Reference, cluster.Name)
	if err != nil || len(rClusters) == 0 {
		return nil, err
	}

	return h.capiClusterCache.Get(rClusters[0].Namespace, rClusters[0].Name)
}

func (h *handler) onUnmanagedMachineOnRemove(_ string, customMachine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	// We may want to look at using a migration to remove this finalizer.
	return customMachine, nil
}

func (h *handler) onUnmanagedMachineChange(_ string, machine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	if machine != nil {
		if !capr.Ready.IsTrue(machine) {
			// CustomMachines are provisioned already, so their Ready condition should be true.
			machine = machine.DeepCopy()
			capr.Ready.SetStatus(machine, "True")
			capr.Ready.Message(machine, "")
			return h.unmanagedMachine.UpdateStatus(machine)
		}
		if !machine.Status.Ready && machine.Spec.ProviderID != "" {
			machine = machine.DeepCopy()
			machine.Status.Ready = true
			return h.unmanagedMachine.UpdateStatus(machine)
		}
	}
	return machine, nil
}
