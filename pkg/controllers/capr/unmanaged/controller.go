package unmanaged

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/data"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const UnmanagedMachineKind = "CustomMachine"

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
	h := handler{
		kubeconfigManager: kubeconfigManager,
		unmanagedMachine:  clients.RKE.CustomMachine(),
		rkeClusterCache:   clients.RKE.RKECluster().Cache(),
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

	relatedresource.Watch(ctx, "unmanaged-machine", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if rkeCluster, ok := obj.(*rkev1.RKECluster); ok {
			var relatedResources []relatedresource.Key
			if rkeCluster.Annotations[capr.DeleteMissingCustomMachinesAfterAnnotation] != "" {
				logrus.Tracef("[unmanaged] handling related resource for RKECluster %s/%s", rkeCluster.Namespace, rkeCluster.Name)
				machines, err := clients.CAPI.Machine().List(rkeCluster.Namespace, metav1.ListOptions{LabelSelector: capi.ClusterNameLabel + "=" + rkeCluster.Annotations[capi.ClusterNameLabel]})
				if err != nil {
					return nil, err
				}
				for _, m := range machines.Items {
					if m.Spec.InfrastructureRef.Kind == UnmanagedMachineKind && m.Spec.InfrastructureRef.APIVersion == capr.RKEAPIVersion && machineHasNodeNotFoundCondition(&m) {
						relatedResources = append(relatedResources, relatedresource.Key{
							Namespace: m.Spec.InfrastructureRef.Namespace,
							Name:      m.Spec.InfrastructureRef.Name,
						})
					}
				}
			}
			return relatedResources, nil
		} else if m, ok := obj.(*capi.Machine); ok {
			if m.Spec.InfrastructureRef.Kind == UnmanagedMachineKind && m.Spec.InfrastructureRef.APIVersion == capr.RKEAPIVersion {
				logrus.Tracef("[unmanaged] handling related resource for CAPI machine %s/%s", m.Namespace, m.Name)
				rkeCluster, err := clients.RKE.RKECluster().Get(m.Namespace, m.Spec.ClusterName, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				if rkeCluster.Annotations[capr.DeleteMissingCustomMachinesAfterAnnotation] != "" && machineHasNodeNotFoundCondition(m) {
					logrus.Tracef("[unmanaged] machine %s/%s has node not found condition", m.Namespace, m.Name)
					return []relatedresource.Key{{
						Namespace: m.Spec.InfrastructureRef.Namespace,
						Name:      m.Spec.InfrastructureRef.Name,
					}}, nil
				}
			}
		}
		return nil, nil
	}, clients.RKE.CustomMachine(), clients.RKE.RKECluster(), clients.CAPI.Machine())
}

type handler struct {
	kubeconfigManager *kubeconfig.Manager
	unmanagedMachine  rkecontroller.CustomMachineController
	rkeClusterCache   rkecontroller.RKEClusterCache
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
					Kind:       UnmanagedMachineKind,
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

func (h *handler) onUnmanagedMachineChange(_ string, customMachine *rkev1.CustomMachine) (*rkev1.CustomMachine, error) {
	if customMachine == nil {
		return customMachine, nil
	}
	if !capr.Ready.IsTrue(customMachine) {
		// CustomMachines are provisioned already, so their Ready condition should be true.
		customMachine = customMachine.DeepCopy()
		capr.Ready.SetStatus(customMachine, "True")
		capr.Ready.Message(customMachine, "")
		return h.unmanagedMachine.UpdateStatus(customMachine)
	}
	if !customMachine.Status.Ready && customMachine.Spec.ProviderID != "" {
		customMachine = customMachine.DeepCopy()
		customMachine.Status.Ready = true
		return h.unmanagedMachine.UpdateStatus(customMachine)
	}

	clusterName := customMachine.Labels[capi.ClusterNameLabel]
	rkeCluster, err := h.rkeClusterCache.Get(customMachine.Namespace, clusterName)
	if err != nil {
		return customMachine, err
	}

	if rkeCluster.Annotations[capr.DeleteMissingCustomMachinesAfterAnnotation] != "" {
		if customMachine.Spec.ProviderID == "" || !customMachine.Status.Ready {
			return customMachine, nil
		}

		capiMachine, err := capr.GetMachineByOwner(h.machineCache, customMachine)
		if err != nil {
			if errors.Is(err, capr.ErrNoMachineOwnerRef) {
				return customMachine, nil
			}
			return customMachine, err
		}

		if machineHasNodeNotFoundCondition(capiMachine) {
			if capiMachine.Status.NodeRef == nil {
				return customMachine, nil
			}
			logrus.Tracef("[unmanaged] RKECluster %s/%s related to CustomMachine %s/%s specifies deletion after duration (%s), and machine was not found per CAPI, evaluating machine for potential deletion", rkeCluster.Namespace, rkeCluster.Name, customMachine.Namespace, customMachine.Name, rkeCluster.Annotations[capr.DeleteMissingCustomMachinesAfterAnnotation])
			d, err := time.ParseDuration(rkeCluster.Annotations[capr.DeleteMissingCustomMachinesAfterAnnotation])
			if err != nil {
				return customMachine, err
			}
			lastTransition := conditions.GetLastTransitionTime(capiMachine, capi.MachineNodeHealthyCondition)
			if lastTransition == nil {
				return customMachine, fmt.Errorf("error retrieving last transition time for condition %s of Machine %s/%s related to CustomMachine %s/%s", capi.MachineNodeHealthyCondition, capiMachine.Namespace, capiMachine.Name, customMachine.Namespace, customMachine.Name)
			}
			now := time.Now()
			if now.After(lastTransition.Time.Add(d)) {
				cluster, err := h.clusterCache.Get(capiMachine.Namespace, capiMachine.Spec.ClusterName)
				if err != nil {
					return customMachine, err
				}

				restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)
				if err != nil {
					return customMachine, err
				}

				clientset, err := kubernetes.NewForConfig(restConfig)
				if err != nil {
					return customMachine, err
				}

				_, err = clientset.CoreV1().Nodes().Get(context.Background(), capiMachine.Status.NodeRef.Name, metav1.GetOptions{})
				if apierror.IsNotFound(err) {
					logrus.Infof("[unmanaged] CustomMachine %s/%s NodeNotFound condition transition time (%s) was past specified deletion duration (%s), proceeding with delete", customMachine.Namespace, customMachine.Name, lastTransition.String(), d.String())
					if err := h.machineClient.Delete(capiMachine.Namespace, capiMachine.Name, nil); err != nil {
						return customMachine, err
					}
				}
				logrus.Errorf("[unmanaged] CustomMachine %s/%s NodeNotFound condition transition time (%s) was past specified deletion duration (%s), but unable to validate node (%s) was actually missing in downstream cluster: %v", customMachine.Namespace, customMachine.Name, lastTransition.String(), d.String(), capiMachine.Status.NodeRef.Name, err)
				return customMachine, err
			}
			nextEnqueueDuration := lastTransition.Time.Add(d).Sub(now)
			logrus.Debugf("[unmanaged] CustomMachine %s/%s NodeNotFound condition last transition time (%s) was not past specified deletion duration (%s), enqueuing after %s", customMachine.Namespace, customMachine.Name, lastTransition.String(), d.String(), nextEnqueueDuration)
			h.unmanagedMachine.EnqueueAfter(customMachine.Namespace, customMachine.Name, nextEnqueueDuration)
		}
	}
	return customMachine, nil
}

func machineHasNodeNotFoundCondition(capiMachine *capi.Machine) bool {
	return conditions.IsFalse(capiMachine, capi.MachineNodeHealthyCondition) && (conditions.GetReason(capiMachine, capi.MachineNodeHealthyCondition) == capi.NodeNotFoundReason)
}
