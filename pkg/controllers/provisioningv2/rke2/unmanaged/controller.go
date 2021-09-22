package unmanaged

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/client-go/util/retry"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboard/clusterindex"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kv"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	machineRequestType = "rke.cattle.io/machine-request"
	capiClusterLabel   = "cluster.x-k8s.io/cluster-name"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		kubeconfigManager: kubeconfig.New(clients),
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
	clients.RKE.CustomMachine().OnChange(ctx, "unmanaged-machine", h.onUnmanagedMachineHealth)
	clients.RKE.CustomMachine().OnRemove(ctx, "unmanaged-machine", h.onUnmanagedMachineOnRemove)
	clients.RKE.CustomMachine().OnChange(ctx, "unmanaged-health", h.onUnmanagedMachineChange)
	clients.Core.Secret().OnChange(ctx, "unmanaged-machine", h.onSecretChange)
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
			"rke.cattle.io/machine-id": machineID,
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

func (h *handler) onSecretChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil || secret.Type != machineRequestType {
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
	if apierror.IsNotFound(err) || capiCluster == nil {
		return secret, nil
	} else if err != nil {
		return secret, err
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

	if secret.Labels[planner.MachineNamespaceLabel] != capiCluster.Namespace ||
		secret.Labels[planner.MachineNameLabel] != machineName {
		secret = secret.DeepCopy()
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[planner.MachineNamespaceLabel] = capiCluster.Namespace
		secret.Labels[planner.MachineNameLabel] = machineName

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
		labels[planner.ControlPlaneRoleLabel] = "true"
	}
	if data.Bool("role-etcd") {
		labels[planner.EtcdRoleLabel] = "true"
	}
	if data.Bool("role-worker") {
		labels[planner.WorkerRoleLabel] = "true"
	}
	if val := data.String("node-name"); val != "" {
		labels[planner.NodeNameLabel] = val
	}
	if address := data.String("address"); address != "" {
		annotations[planner.AddressAnnotation] = address
	}
	if internalAddress := data.String("internal-address"); internalAddress != "" {
		annotations[planner.InternalAddressAnnotation] = internalAddress
	}

	labels["rke.cattle.io/machine-id"] = data.String("id")

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
		annotations[planner.LabelsAnnotation] = string(data)
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
		annotations[planner.TaintsAnnotation] = string(data)
	}

	return []runtime.Object{
		&rkev1.RKEBootstrap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      machineName,
				Namespace: capiCluster.Namespace,
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
				Name:        machineName,
				Namespace:   capiCluster.Namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: capi.MachineSpec{
				ClusterName: capiCluster.Name,
				Bootstrap: capi.Bootstrap{
					ConfigRef: &corev1.ObjectReference{
						Kind:       "RKEBootstrap",
						Namespace:  capiCluster.Namespace,
						Name:       machineName,
						APIVersion: "rke.cattle.io/v1",
					},
				},
				InfrastructureRef: corev1.ObjectReference{
					Kind:       "CustomMachine",
					Namespace:  capiCluster.Namespace,
					Name:       machineName,
					APIVersion: "rke.cattle.io/v1",
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

func (h *handler) getMachine(customMachine *rkev1.CustomMachine) (*capi.Machine, error) {
	var (
		machine *capi.Machine
		err     error
	)

	for _, owner := range customMachine.OwnerReferences {
		if owner.Kind == "Machine" {
			machine, err = h.machineCache.Get(customMachine.Namespace, owner.Name)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return machine, nil
}

func (h *handler) onUnmanagedMachineHealth(key string, customMachine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	if customMachine == nil {
		return nil, nil
	}

	if customMachine.Spec.ProviderID == "" || !customMachine.Status.Ready {
		return customMachine, nil
	}

	machine, err := h.getMachine(customMachine)
	if err != nil || machine == nil {
		return customMachine, err
	}

	cluster, err := h.clusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if err != nil {
		return customMachine, err
	}

	h.unmanagedMachine.EnqueueAfter(customMachine.Namespace, customMachine.Name, 15*time.Second)

	if machine.Status.NodeRef == nil {
		return customMachine, nil
	}

	restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)
	if err != nil {
		return customMachine, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return customMachine, err
	}

	_, err = clientset.CoreV1().Nodes().Get(context.Background(), machine.Status.NodeRef.Name, metav1.GetOptions{})
	if apierror.IsNotFound(err) {
		err = h.machineClient.Delete(machine.Namespace, machine.Name, nil)
		return customMachine, err
	}

	return customMachine, nil
}

func (h *handler) onUnmanagedMachineOnRemove(key string, customMachine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	clusterName := customMachine.Labels[capiClusterLabel]
	if clusterName == "" {
		return customMachine, nil
	}

	cluster, err := h.clusterCache.Get(customMachine.Namespace, clusterName)
	if apierror.IsNotFound(err) {
		return customMachine, nil
	} else if err != nil {
		return customMachine, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return customMachine, nil
	}

	machine, err := h.getMachine(customMachine)
	if err != nil || machine == nil {
		return customMachine, err
	}

	if _, ok := machine.Labels[planner.EtcdRoleLabel]; !ok {
		return customMachine, nil // If we are not dealing with an unmanaged etcd node, we can go ahead and allow removal
	}

	if machine.Status.NodeRef == nil {
		return customMachine, nil
	}
	// Check status of v1 node
	restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)
	if err != nil {
		return customMachine, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return customMachine, err
	}

	removeAnnotation := "etcd." + planner.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/remove"
	removedNodeNameAnnotation := "etcd." + planner.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/removed-node-name"

	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierror.IsNotFound(err) {
			return customMachine, nil
		}
		return customMachine, err
	}

	if val, ok := node.Annotations[removeAnnotation]; ok {
		// check val to see if it's true, if not, continue
		if val == "true" {
			// check the status of the removal
			if removedNodeName, ok := node.Annotations[removedNodeNameAnnotation]; ok {
				// There is the possibility the annotation is defined, but empty.
				if removedNodeName != "" {
					return customMachine, nil
				}
			}
		}
	}
	// The remove annotation has not been set to true, so we'll go ahead and set it on the node.
	err = retry.RetryOnConflict(retry.DefaultRetry,
		func() error {
			node.Annotations[removeAnnotation] = "true"
			node, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			return err
		})
	if err != nil {
		// there was an error updating the node
		return customMachine, err
	}

	return customMachine, fmt.Errorf("waiting for etcd member removal")
}

func (h *handler) onUnmanagedMachineChange(key string, machine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	if machine != nil && !machine.Status.Ready && machine.Spec.ProviderID != "" {
		machine = machine.DeepCopy()
		machine.Status.Ready = true
		return h.unmanagedMachine.UpdateStatus(machine)
	}
	return machine, nil
}
