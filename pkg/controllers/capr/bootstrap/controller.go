package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/installer"
	"github.com/rancher/rancher/pkg/controllers/capr/etcdmgmt"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/wrangler"
	appcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/secret"
)

const (
	rkeBootstrapName                   = "rke.cattle.io/rkebootstrap-name"
	capiMachinePreDrainAnnotation      = "pre-drain.delete.hook.machine.cluster.x-k8s.io/rke-bootstrap-cleanup"
	capiMachinePreDrainAnnotationOwner = "rke-bootstrap-controller"
	capiMachinePreTerminateAnnotation  = "pre-terminate.delete.hook.machine.cluster.x-k8s.io/rke-bootstrap-cleanup"
)

type handler struct {
	serviceAccountCache corecontrollers.ServiceAccountCache
	secretCache         corecontrollers.SecretCache
	secretClient        corecontrollers.SecretClient
	machineCache        capicontrollers.MachineCache
	machineClient       capicontrollers.MachineClient
	capiClusterCache    capicontrollers.ClusterCache
	deploymentCache     appcontrollers.DeploymentCache
	rkeControlPlanes    rkecontroller.RKEControlPlaneCache
	rkeBootstrap        rkecontroller.RKEBootstrapController
	k8s                 kubernetes.Interface
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		serviceAccountCache: clients.Core.ServiceAccount().Cache(),
		secretCache:         clients.Core.Secret().Cache(),
		secretClient:        clients.Core.Secret(),
		machineCache:        clients.CAPI.Machine().Cache(),
		machineClient:       clients.CAPI.Machine(),
		capiClusterCache:    clients.CAPI.Cluster().Cache(),
		deploymentCache:     clients.Apps.Deployment().Cache(),
		rkeControlPlanes:    clients.RKE.RKEControlPlane().Cache(),
		rkeBootstrap:        clients.RKE.RKEBootstrap(),
		k8s:                 clients.K8s,
	}

	clients.RKE.RKEBootstrap().OnChange(ctx, "rke-bootstrap-cluster-name", h.OnChange)
	clients.RKE.RKEBootstrap().OnRemove(ctx, "rke-bootstrap-etcd-removal", h.OnRemove)
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
		"rke-bootstrap",
		h.GeneratingHandler,
		nil)

	relatedresource.Watch(ctx, "rke-bootstrap-trigger", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
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
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), h.secretCache.Get, h.k8s, sa)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(secret.Data["token"])

	hasHostPort, err := h.rancherDeploymentHasHostPort()
	if err != nil {
		return nil, err
	}

	is := installer.LinuxInstallScript
	if os := machine.GetLabels()[capr.CattleOSLabel]; os == capr.WindowsMachineOS {
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

func (h *handler) assignPlanSecret(machine *capi.Machine, bootstrap *rkev1.RKEBootstrap) []runtime.Object {
	planSecretName := capr.PlanSecretFromBootstrapName(bootstrap.Name)
	labels, annotations := getLabelsAndAnnotationsForPlanSecret(bootstrap, machine)

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: bootstrap.Namespace,
			Labels: map[string]string{
				capr.MachineNameLabel: machine.Name,
				rkeBootstrapName:      bootstrap.Name,
				capr.RoleLabel:        capr.RolePlan,
				capr.PlanSecret:       planSecretName,
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        planSecretName,
			Namespace:   bootstrap.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Type: capr.SecretTypeMachinePlan,
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: bootstrap.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"watch", "get", "update", "list"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{planSecretName},
			},
		},
	}
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
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
			Name:     planSecretName,
		},
	}

	return []runtime.Object{sa, secret, role, roleBinding}
}

func (h *handler) getEnvVar(bootstrap *rkev1.RKEBootstrap, capiCluster *capi.Cluster) ([]corev1.EnvVar, error) {
	if capiCluster.Spec.ControlPlaneRef == nil || capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" {
		return nil, nil
	}

	cp, err := h.rkeControlPlanes.Get(bootstrap.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return nil, err
	}

	var result []corev1.EnvVar
	for _, env := range cp.Spec.AgentEnvVars {
		result = append(result, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}

	return result, nil
}

// shouldCreateBootstrapSecret returns true if the generated handler should create/ensure the bootstrap secret's
// existence, otherwise it wil be cleaned up. The bootstrap secret is created immediately in the Pending phase and
// should be present until machine deletion.
func shouldCreateBootstrapSecret(phase capi.MachinePhase) bool {
	return phase != capi.MachinePhaseDeleting && phase != capi.MachinePhaseDeleted && phase != capi.MachinePhaseFailed
}

// assignBootStrapSecret is utilized by the bootstrap controller's GeneratingHandler method to designate the lifecycle
// of both the bootstrap secret and related service account. The bootstrap secret and service account must be valid
// until the corresponding CAPI Machine object's Machine Phase is at least "Running", which indicates that the machine
// "has become a Kubernetes Node in a Ready state".
func (h *handler) assignBootStrapSecret(machine *capi.Machine, bootstrap *rkev1.RKEBootstrap, capiCluster *capi.Cluster) (*corev1.Secret, []runtime.Object, error) {
	if !shouldCreateBootstrapSecret(capi.MachinePhase(machine.Status.Phase)) {
		return nil, nil, nil
	}

	envVars, err := h.getEnvVar(bootstrap, capiCluster)
	if err != nil {
		return nil, nil, err
	}

	secretName := name.SafeConcatName(bootstrap.Name, "machine", "bootstrap")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: bootstrap.Namespace,
			Labels: map[string]string{
				capr.MachineNameLabel: machine.Name,
				rkeBootstrapName:      bootstrap.Name,
				capr.RoleLabel:        capr.RoleBootstrap,
			},
		},
	}

	bootstrapSecret, err := h.getBootstrapSecret(sa.Namespace, sa.Name, envVars, machine)
	if err != nil {
		return nil, nil, err
	}

	return bootstrapSecret, []runtime.Object{sa}, nil
}

func (h *handler) OnChange(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	if bootstrap == nil || !bootstrap.DeletionTimestamp.IsZero() {
		return bootstrap, nil
	}

	// If the bootstrap spec cluster name is blank, we need to update the bootstrap spec to the correct value
	// This is to handle old rkebootstrap objects for unmanaged clusters that did not have the spec properly set
	if v, ok := bootstrap.Labels[capi.ClusterLabelName]; ok && v != "" && bootstrap.Spec.ClusterName != v {
		logrus.Debugf("[rkebootstrap] %s/%s: setting cluster name", bootstrap.Namespace, bootstrap.Name)
		bootstrap = bootstrap.DeepCopy()
		bootstrap.Spec.ClusterName = v
		return h.rkeBootstrap.Update(bootstrap)
	}

	return h.reconcileMachinePreDrainAnnotation(bootstrap)
}

func (h *handler) GeneratingHandler(bootstrap *rkev1.RKEBootstrap, status rkev1.RKEBootstrapStatus) ([]runtime.Object, rkev1.RKEBootstrapStatus, error) {
	var (
		result []runtime.Object
	)

	machine, err := capr.GetOwnerCAPIMachine(bootstrap, h.machineCache)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[rkebootstrap] %s/%s: waiting: machine to be set as owner reference", bootstrap.Namespace, bootstrap.Name)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 10*time.Second)
		return result, status, generic.ErrSkip
	}
	if err != nil {
		logrus.Errorf("[rkebootstrap] %s/%s: error getting machine by owner reference %v", bootstrap.Namespace, bootstrap.Name, err)
		return nil, status, err
	}

	capiCluster, err := h.capiClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[rkebootstrap] %s/%s: waiting: CAPI cluster does not exist", bootstrap.Namespace, bootstrap.Name)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 10*time.Second)
		return result, status, generic.ErrSkip
	}
	if err != nil {
		logrus.Errorf("[rkebootstrap] %s/%s: error getting CAPI cluster %v", bootstrap.Namespace, bootstrap.Name, err)
		return result, status, err
	}

	if capiannotations.IsPaused(capiCluster, bootstrap) {
		logrus.Debugf("[rkebootstrap] %s/%s: waiting: CAPI cluster or RKEBootstrap is paused", bootstrap.Namespace, bootstrap.Name)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 10*time.Second)
		return result, status, generic.ErrSkip
	}

	if !capiCluster.Status.InfrastructureReady {
		logrus.Debugf("[rkebootstrap] %s/%s: waiting: CAPI cluster infrastructure is not ready", bootstrap.Namespace, bootstrap.Name)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 10*time.Second)
		return result, status, generic.ErrSkip
	}

	// The plan secret is used by the planner to deliver plans to the system-agent (and receive feedback)
	result = append(result, h.assignPlanSecret(machine, bootstrap)...)

	// The bootstrap secret contains the system-agent install script with corresponding information to bootstrap the node
	bootstrapSecret, objs, err := h.assignBootStrapSecret(machine, bootstrap, capiCluster)
	if err != nil {
		return nil, status, err
	}

	if bootstrapSecret != nil {
		if status.DataSecretName == nil {
			status.DataSecretName = &bootstrapSecret.Name
			status.Ready = true
			logrus.Debugf("[rkebootstrap] %s/%s: setting dataSecretName: %s", bootstrap.Namespace, bootstrap.Name, *status.DataSecretName)
		}
		result = append(result, bootstrapSecret)
	}

	result = append(result, objs...)
	return result, status, nil
}

func (h *handler) rancherDeploymentHasHostPort() (bool, error) {
	deployment, err := h.deploymentCache.Get(namespace.System, "rancher")
	if err != nil {
		if apierrors.IsNotFound(err) {
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
	labels[capr.MachineNameLabel] = machine.Name
	labels[capr.ClusterNameLabel] = bootstrap.Spec.ClusterName
	for k, v := range bootstrap.Labels {
		labels[k] = v
	}

	annotations := make(map[string]string, len(bootstrap.Annotations))
	for k, v := range bootstrap.Annotations {
		annotations[k] = v
	}

	return labels, annotations
}

// OnRemove adds finalizer handling to the RKEBootstrap object, and is used to prevent deletion of the RKE Bootstrap
// when it is deleting and bootstrap is for an etcd node.
func (h *handler) OnRemove(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	logrus.Debugf("[rkebootstrap] %s/%s: OnRemove invoked", bootstrap.Namespace, bootstrap.Name)

	return h.reconcileMachinePreDrainAnnotation(bootstrap)
}

// reconcileMachinePreDrainAnnotation reconciles the machine object that owns the bootstrap. It only reconciles the machine if it is an
// etcd machine. Its primary purpose is to manage the pre-drain.delete.hook.machine.x-k8s.io annotation on the machine
// object, which is used to prevent draining of the corresponding downstream node, since draining may include the static
// etcd pod which could cause a quorum loss or at worst inability to elect a new etcd member.
// The pre-drain hook will be set on the machine object if the machine and bootstrap are not deleting, the corresponding
// CAPI cluster and RKEControlPlane are not deleting, and the force remove annotation is not set on the bootstrap.
// The annotation will be removed from the machine to allow draining in the following cases:
// * The machine is deleting and no machines are using this one's join-url
// * The bootstrap is missing the CAPI cluster label || the CAPI cluster controlPlaneRef is nil || the machine noderef is nil
// * Any of the following: CAPI kubeconfig secret, CAPI cluster object, RKEControlPlane object are not found
//
// Notably, CAPI controllers do not trigger a deletion of the RKEBootstrap object if a pre-drain annotation exists on the corresponding machine object.
// This means we rely on the OnChange handler to perform node safe removal, when it sees that the corresponding machine is deleting.
func (h *handler) reconcileMachinePreDrainAnnotation(bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	machine, err := capr.GetMachineByOwner(h.machineCache, bootstrap)
	if err != nil {
		if errors.Is(err, capr.ErrNoMachineOwnerRef) || apierrors.IsNotFound(err) {
			// If we did not find the machine by owner ref or the cache returned a not found, then noop.
			return bootstrap, nil
		}
		return bootstrap, err
	}

	_, isEtcd := machine.Labels[capr.EtcdRoleLabel]

	forceRemove, ok := bootstrap.Annotations[capr.ForceRemoveEtcdAnnotation]
	if (ok && strings.ToLower(forceRemove) == "true") || !isEtcd {
		// If the force remove annotation is "true" or the node is not an etcd node, then ensure the machine pre drain annotation is removed.
		return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
	}

	// Only add the pre-drain hook annotation if the corresponding machine and bootstrap are NOT deleting
	if machine.DeletionTimestamp.IsZero() && bootstrap.DeletionTimestamp.IsZero() {
		// annotate the CAPI machine with the pre-drain.delete.hook.machine.cluster.x-k8s.io annotation if it is an etcd machine
		if val, ok := machine.GetAnnotations()[capiMachinePreDrainAnnotation]; !ok || val != capiMachinePreDrainAnnotationOwner {
			machine = machine.DeepCopy()
			if machine.Annotations == nil {
				machine.Annotations = make(map[string]string)
			}
			machine.Annotations[capiMachinePreDrainAnnotation] = capiMachinePreDrainAnnotationOwner
			machine, err = h.machineClient.Update(machine)
			if err != nil {
				return bootstrap, err
			}
		}
		return bootstrap, nil
	}

	if bootstrap.Spec.ClusterName == "" {
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster label %s was not found in bootstrap labels, ensuring machine pre-drain annotation is removed", bootstrap.Namespace, bootstrap.Name, capi.ClusterLabelName)
		return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
	}

	capiCluster, err := h.capiClusterCache.Get(bootstrap.Namespace, bootstrap.Spec.ClusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s was not found, ensuring machine pre-drain annotation is removed", bootstrap.Namespace, bootstrap.Name, bootstrap.Namespace, bootstrap.Spec.ClusterName)
			return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if capiCluster.Spec.ControlPlaneRef == nil {
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s controlplane object reference was nil, ensuring machine pre-drain annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Name)
		return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
	}

	cp, err := h.rkeControlPlanes.Get(capiCluster.Spec.ControlPlaneRef.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("[rkebootstrap] %s/%s: RKEControlPlane %s/%s was not found, ensuring machine pre-drain annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Spec.ControlPlaneRef.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
			return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if !cp.DeletionTimestamp.IsZero() || !capiCluster.DeletionTimestamp.IsZero() {
		return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
	}

	if machine.Status.NodeRef == nil {
		logrus.Infof("[rkebootstrap] No associated node found for machine %s/%s in cluster %s, ensuring machine pre-drain annotation is removed", machine.Namespace, machine.Name, bootstrap.Spec.ClusterName)
		return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
	}

	// If the RKEControlPlane is not deleting, then make sure this node is not being used as an init node.
	if cp.DeletionTimestamp.IsZero() {
		planSecret, err := h.secretCache.Get(bootstrap.Namespace, capr.PlanSecretFromBootstrapName(bootstrap.Name))
		if err != nil && !apierrors.IsNotFound(err) {
			return bootstrap, fmt.Errorf("error retrieving plan secret to validate it was not an init node: %v", err)
		}

		if planSecret != nil {
			// validate that no other nodes are joined to this node, otherwise removing it will cause a bunch of nodes to start crashing.
			joinURL := planSecret.Annotations[capr.JoinURLAnnotation]
			planSecrets, err := h.secretCache.List(bootstrap.Namespace, labels.SelectorFromSet(map[string]string{
				capi.ClusterLabelName: bootstrap.Spec.ClusterName,
			}))
			if err != nil {
				return bootstrap, fmt.Errorf("error encountered list plansecrets to ensure node was not joined: %v", err)
			}
			for _, ps := range planSecrets {
				if ps.GetAnnotations()[capr.JoinedToAnnotation] == joinURL {
					logrus.Errorf("[rkebootstrap] %s/%s: cluster %s/%s machine %s/%s was still joined to deleting etcd machine %s/%s", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Name, bootstrap.Namespace, ps.GetLabels()[capr.MachineNameLabel], machine.Namespace, machine.Name)
					h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 5*time.Second)
					return bootstrap, generic.ErrSkip
				}
			}
		}
	}

	kcSecret, err := h.secretCache.Get(bootstrap.Namespace, secret.Name(bootstrap.Spec.ClusterName, secret.Kubeconfig))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kcSecret.Data["value"])
	if err != nil {
		return bootstrap, err
	}

	removed, err := etcdmgmt.SafelyRemoved(restConfig, capr.GetRuntimeCommand(cp.Spec.KubernetesVersion), machine.Status.NodeRef.Name)
	if err != nil {
		return bootstrap, err
	}
	if !removed {
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 5*time.Second)
		return bootstrap, generic.ErrSkip
	}

	return h.ensureMachinePreDrainAnnotationRemoved(bootstrap, machine)
}

// ensureMachinePreDrainAnnotationRemoved removes the pre-drain annotation from a CAPI machine when we remove
// the rkebootstrap, indicating the node can be drained. This also removes the legacy capiMachinePreTerminateAnnotation
// if it exists.
func (h *handler) ensureMachinePreDrainAnnotationRemoved(bootstrap *rkev1.RKEBootstrap, machine *capi.Machine) (*rkev1.RKEBootstrap, error) {
	if machine == nil || machine.Annotations == nil {
		return bootstrap, nil
	}

	var err error
	if _, ok := machine.GetAnnotations()[capiMachinePreDrainAnnotation]; ok {
		machine = machine.DeepCopy()
		delete(machine.Annotations, capiMachinePreDrainAnnotation)
		_, err = h.machineClient.Update(machine)
	}
	if err != nil {
		return bootstrap, err
	}
	if _, ok := machine.GetAnnotations()[capiMachinePreTerminateAnnotation]; ok {
		machine = machine.DeepCopy()
		delete(machine.Annotations, capiMachinePreTerminateAnnotation)
		_, err = h.machineClient.Update(machine)
	}
	return bootstrap, err
}
