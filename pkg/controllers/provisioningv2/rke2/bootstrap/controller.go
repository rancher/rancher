package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/installer"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/wrangler"
	appcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	rkeBootstrapName = "rke.cattle.io/rkebootstrap-name"
	roleBootstrap    = "bootstrap"
	rolePlan         = "plan"
)

type handler struct {
	serviceAccountCache corecontrollers.ServiceAccountCache
	secretCache         corecontrollers.SecretCache
	machineCache        capicontrollers.MachineCache
	capiClusters        capicontrollers.ClusterCache
	deploymentCache     appcontrollers.DeploymentCache
	rkeControlPlanes    rkecontroller.RKEControlPlaneCache
	rkeBootstrapClient  rkecontroller.RKEBootstrapClient
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		serviceAccountCache: clients.Core.ServiceAccount().Cache(),
		secretCache:         clients.Core.Secret().Cache(),
		machineCache:        clients.CAPI.Machine().Cache(),
		capiClusters:        clients.CAPI.Cluster().Cache(),
		deploymentCache:     clients.Apps.Deployment().Cache(),
		rkeControlPlanes:    clients.RKE.RKEControlPlane().Cache(),
		rkeBootstrapClient:  clients.RKE.RKEBootstrap(),
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
			if name, ok := sa.Labels[rkeBootstrapName]; ok {
				return []relatedresource.Key{
					{
						Namespace: sa.Namespace,
						Name:      name,
					},
				}, nil
			}
		}
		if machine, ok := obj.(*capi.Machine); ok {
			if machine.Spec.Bootstrap.ConfigRef != nil && machine.Spec.Bootstrap.ConfigRef.Kind == "RKEBootstrap" {
				return []relatedresource.Key{{
					Namespace: machine.Namespace,
					Name:      machine.Spec.Bootstrap.ConfigRef.Name,
				}}, nil
			}
		}
		return nil, nil
	}, clients.RKE.RKEBootstrap(), clients.Core.ServiceAccount(), clients.CAPI.Machine())
}

func (h *handler) getBootstrapSecret(namespace, name string, envVars []corev1.EnvVar, machine *capi.Machine) (*corev1.Secret, error) {
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

		hasHostPort, err := h.rancherDeploymentHasHostPort()
		if err != nil {
			return nil, err
		}

		is := installer.LinuxInstallScript
		if os := machine.GetLabels()[rke2.CattleOSLabel]; os == rke2.WindowsMachineOS {
			is = installer.WindowsInstallScript
		}
		data, err := is(context.WithValue(context.Background(), tls.InternalAPI, hasHostPort), base64.URLEncoding.EncodeToString(hash[:]), envVars, "")
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

func (h *handler) assignPlanSecret(machine *capi.Machine, bootstrap *rkev1.RKEBootstrap) []runtime.Object {
	secretName := rke2.PlanSecretFromBootstrapName(bootstrap.Name)
	labels, annotations := getLabelsAndAnnotationsForPlanSecret(bootstrap, machine)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bootstrap.Namespace,
			Labels: map[string]string{
				rke2.MachineNameLabel: machine.Name,
				rkeBootstrapName:      bootstrap.Name,
				rke2.RoleLabel:        rolePlan,
				rke2.PlanSecret:       secretName,
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   bootstrap.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: rke2.SecretTypeMachinePlan,
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bootstrap.Namespace,
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
			Namespace: bootstrap.Namespace,
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

	return []runtime.Object{sa, secret, role, rolebinding}
}

func (h *handler) getEnvVar(bootstrap *rkev1.RKEBootstrap) (result []corev1.EnvVar, _ error) {
	capiCluster, err := h.capiClusters.Get(bootstrap.Namespace, bootstrap.Spec.ClusterName)
	if apierror.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	if capiCluster.Spec.ControlPlaneRef == nil || capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" {
		return nil, nil
	}

	cp, err := h.rkeControlPlanes.Get(bootstrap.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return nil, err
	}

	for _, env := range cp.Spec.AgentEnvVars {
		result = append(result, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}

	return result, nil
}

func (h *handler) assignBootStrapSecret(machine *capi.Machine, bootstrap *rkev1.RKEBootstrap) (*corev1.Secret, []runtime.Object, error) {
	if capi.MachinePhase(machine.Status.Phase) != capi.MachinePhasePending &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseDeleting &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseFailed &&
		capi.MachinePhase(machine.Status.Phase) != capi.MachinePhaseProvisioning {
		return nil, nil, nil
	}

	envVars, err := h.getEnvVar(bootstrap)
	if err != nil {
		return nil, nil, err
	}

	secretName := name.SafeConcatName(bootstrap.Name, "machine", "bootstrap")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bootstrap.Namespace,
			Labels: map[string]string{
				rke2.MachineNameLabel: machine.Name,
				rkeBootstrapName:      bootstrap.Name,
				rke2.RoleLabel:        roleBootstrap,
			},
		},
	}

	bootstrapSecret, err := h.getBootstrapSecret(sa.Namespace, sa.Name, envVars, machine)
	if err != nil {
		return nil, nil, err
	}

	return bootstrapSecret, []runtime.Object{sa}, nil
}

func (h *handler) OnChange(bootstrap *rkev1.RKEBootstrap, status rkev1.RKEBootstrapStatus) ([]runtime.Object, rkev1.RKEBootstrapStatus, error) {
	var (
		result []runtime.Object
	)

	if bootstrap.Spec.ClusterName == "" {
		// If the bootstrap spec cluster name is blank, we need to update the bootstrap spec to the correct value
		// This is to handle old rkebootstrap objects for unmanaged clusters that did not have the spec properly set
		if v, ok := bootstrap.Labels[capi.ClusterLabelName]; ok && v != "" {
			bootstrap = bootstrap.DeepCopy()
			bootstrap.Spec.ClusterName = v
			var err error
			bootstrap, err = h.rkeBootstrapClient.Update(bootstrap)
			if err != nil {
				return nil, bootstrap.Status, err
			}
		}
	}

	machine, err := rke2.GetMachineByOwner(h.machineCache, bootstrap)
	if err != nil {
		if errors.Is(err, rke2.ErrNoMachineOwnerRef) {
			return nil, status, generic.ErrSkip
		}
		return nil, status, err
	}

	result = append(result, h.assignPlanSecret(machine, bootstrap)...)

	bootstrapSecret, objs, err := h.assignBootStrapSecret(machine, bootstrap)
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

func (h *handler) rancherDeploymentHasHostPort() (bool, error) {
	deployment, err := h.deploymentCache.Get(namespace.System, "rancher")
	if err != nil {
		if apierror.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			if container.Name == "rancher" && port.HostPort != 0 {
				return true, nil
			}
		}
	}

	return false, nil
}

func getLabelsAndAnnotationsForPlanSecret(bootstrap *rkev1.RKEBootstrap, machine *capi.Machine) (map[string]string, map[string]string) {
	labels := make(map[string]string, len(bootstrap.Labels)+2)
	labels[rke2.MachineNameLabel] = machine.Name
	labels[rke2.ClusterNameLabel] = bootstrap.Spec.ClusterName
	for k, v := range bootstrap.Labels {
		labels[k] = v
	}

	annotations := make(map[string]string, len(bootstrap.Annotations)+1)
	annotations[rke2.JoinURLAnnotation] = getMachineJoinURL(machine)
	for k, v := range bootstrap.Annotations {
		annotations[k] = v
	}

	return labels, annotations
}

func getMachineJoinURL(machine *capi.Machine) string {
	if machine.Status.NodeInfo == nil {
		return ""
	}

	address := ""
	for _, machineAddress := range machine.Status.Addresses {
		switch machineAddress.Type {
		case capi.MachineInternalIP:
			address = machineAddress.Address
		case capi.MachineExternalIP:
			if address == "" {
				address = machineAddress.Address
			}
		}
	}

	return fmt.Sprintf("https://%s:%d", address, rke2.GetRuntimeSupervisorPort(machine.Status.NodeInfo.KubeletVersion))
}
