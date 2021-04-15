package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	serviceAccountCache corecontrollers.ServiceAccountCache
	secretCache         corecontrollers.SecretCache
	rancherClusterCache ranchercontrollers.ClusterCache
	machineCache        capicontrollers.MachineCache
	machines            capicontrollers.MachineController
	settingsCache       mgmtcontrollers.SettingCache
	rkeBootstrapCache   rkecontroller.RKEBootstrapCache
	rkeBootstrap        rkecontroller.RKEBootstrapClient
	kubeconfigManager   *kubeconfig.Manager
	dynamic             *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		serviceAccountCache: clients.Core.ServiceAccount().Cache(),
		secretCache:         clients.Core.Secret().Cache(),
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		machines:            clients.CAPI.Machine(),
		machineCache:        clients.CAPI.Machine().Cache(),
		settingsCache:       clients.Mgmt.Setting().Cache(),
		rkeBootstrapCache:   clients.RKE.RKEBootstrap().Cache(),
		rkeBootstrap:        clients.RKE.RKEBootstrap(),
		kubeconfigManager:   kubeconfig.New(clients),
		dynamic:             clients.Dynamic,
	}
	rkecontroller.RegisterRKEBootstrapGeneratingHandler(ctx,
		clients.RKE.RKEBootstrap(),
		clients.Apply.
			WithCacheTypes(
				clients.RBAC.Role(),
				clients.RBAC.RoleBinding(),
				clients.CAPI.Machine(),
				clients.Core.ServiceAccount(),
				clients.Core.Secret()).
			WithSetOwnerReference(true, true),
		"",
		"rke-machine",
		h.OnChange,
		nil)

	relatedresource.Watch(ctx, "rke-machine-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if sa, ok := obj.(*corev1.ServiceAccount); ok {
			if name, ok := sa.Labels[planner.MachineNameLabel]; ok {
				return []relatedresource.Key{
					{
						Namespace: sa.Namespace,
						Name:      name,
					},
				}, nil
			}
		}
		return nil, nil
	}, clients.CAPI.Machine(), clients.Core.ServiceAccount())

	clients.RKE.RKEBootstrap().OnChange(ctx, "machine-provider-sync", h.associateMachineWithNode)
}

func (h *handler) getBootstrapSecret(namespace, name string) (*corev1.Secret, error) {
	sa, err := h.serviceAccountCache.Get(namespace, name)
	if apierror.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err

	}
	for _, secretRef := range sa.Secrets {
		secret, err := h.secretCache.Get(sa.Namespace, secretRef.Name)
		if err != nil {
			return nil, err
		}

		hash := sha256.Sum256(secret.Data["token"])
		data, err := Bootstrap(base64.URLEncoding.EncodeToString(hash[:]))
		if err != nil {
			return nil, err
		}

		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"value": data,
			},
			Type: "rke.cattle.io/bootstrap",
		}, nil
	}

	return nil, nil
}

func (h *handler) assignPlanSecret(machine *capi.Machine, obj *rkev1.RKEBootstrap) ([]runtime.Object, error) {
	secretName := planner.PlanSecretFromBootstrapName(obj.Name)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.Namespace,
			Labels: map[string]string{
				planner.MachineNameLabel: machine.Name,
				roleLabel:                rolePlan,
				planSecret:               secretName,
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.Namespace,
			Labels: map[string]string{
				planner.MachineNameLabel: machine.Name,
			},
		},
		Type: planner.SecretTypeMachinePlan,
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"watch", "get", "update", "list"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{secretName},
			},
		},
	}
	rolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     secretName,
		},
	}

	return []runtime.Object{sa, secret, role, rolebinding}, nil
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
	return nil, fmt.Errorf("not machine associated to RKEBootstrap %s/%s", obj.Namespace, obj.Name)
}

func (h *handler) assignBootStrapSecret(machine *capi.Machine, obj *rkev1.RKEBootstrap) (*corev1.Secret, []runtime.Object, error) {
	if capi.MachinePhase(machine.Status.Phase) != capi.MachinePhasePending &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseDeleting &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseFailed &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseProvisioning {
		return nil, nil, nil
	}

	secretName := name.SafeConcatName(obj.Name, "machine", "bootstrap")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: obj.Namespace,
			Labels: map[string]string{
				planner.MachineNameLabel: machine.Name,
				roleLabel:                roleBootstrap,
			},
		},
	}

	bootstrapSecret, err := h.getBootstrapSecret(sa.Namespace, sa.Name)
	if err != nil {
		return nil, nil, err
	}

	return bootstrapSecret, []runtime.Object{sa}, nil
}

func (h *handler) OnChange(obj *rkev1.RKEBootstrap, status rkev1.RKEBootstrapStatus) ([]runtime.Object, rkev1.RKEBootstrapStatus, error) {
	var (
		result []runtime.Object
	)

	machine, err := h.getMachine(obj)
	if err != nil {
		return nil, status, err
	}

	objs, err := h.assignPlanSecret(machine, obj)
	if err != nil {
		return nil, status, err
	}

	result = append(result, objs...)

	bootstrapSecret, objs, err := h.assignBootStrapSecret(machine, obj)
	if err != nil {
		return nil, status, err
	}

	if bootstrapSecret != nil {
		if status.DataSecretName == nil {
			status.DataSecretName = &bootstrapSecret.Name
			status.Ready = true
		}
		result = append(result, bootstrapSecret)
	}

	result = append(result, objs...)
	return result, status, nil
}

func (h *handler) associateMachineWithNode(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	if bootstrap == nil || bootstrap.DeletionTimestamp != nil {
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

	secret, err := h.kubeconfigManager.GetKubeConfig(rancherCluster, rancherCluster.Status)
	if err != nil {
		return bootstrap, err
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return bootstrap, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return bootstrap, err
	}

	nodeLabelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"rke.cattle.io/machine": string(machine.GetUID())}}
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(nodeLabelSelector.MatchLabels).String()})
	if err != nil || len(nodes.Items) == 0 || nodes.Items[0].Spec.ProviderID == "" {
		h.machines.EnqueueAfter(machine.Namespace, machine.Name, nodeErrorEnqueueTime)
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

	url := fmt.Sprintf("https://%s:%d", address, getJoinURLPort(rancherCluster))
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
		data, err := data.Convert(infra.DeepCopyObject())
		if err != nil {
			return err
		}

		data.SetNested(node.Spec.ProviderID, "spec", "providerID")
		_, err = h.dynamic.Update(&unstructured.Unstructured{
			Object: data,
		})
		return err
	}

	return nil
}

func getJoinURLPort(cluster *v1.Cluster) int {
	if planner.GetRuntime(cluster.Spec.KubernetesVersion) == planner.RuntimeRKE2 {
		return 9345
	}
	return 6443
}
