package bootstrap

import (
	"bytes"
	"compress/gzip"
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
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/tls"
	"github.com/rancher/rancher/pkg/wrangler"
	appcontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/apps/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
)

const (
	rkeBootstrapName                       = "rke.cattle.io/rkebootstrap-name"
	capiMachinePreTerminateAnnotation      = "pre-terminate.delete.hook.machine.cluster.x-k8s.io/rke-bootstrap-cleanup"
	capiMachinePreTerminateAnnotationOwner = "rke-bootstrap-controller"
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

func Register(ctx context.Context, clients *wrangler.CAPIContext) {
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
			if machine.Spec.Bootstrap.ConfigRef.IsDefined() && machine.Spec.Bootstrap.ConfigRef.Kind == capr.RKEBootstrapKind {
				return []relatedresource.Key{{
					Namespace: machine.Namespace,
					Name:      machine.Spec.Bootstrap.ConfigRef.Name,
				}}, nil
			}
		}
		return nil, nil
	}, clients.RKE.RKEBootstrap(), clients.Core.ServiceAccount(), clients.CAPI.Machine())
}

func (h *handler) getBootstrapSecret(namespace, name string, envVars []corev1.EnvVar, machine *capi.Machine, bootstrap *rkev1.RKEBootstrap, dataDir string) (*corev1.Secret, error) {
	sa, err := h.serviceAccountCache.Get(namespace, name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), h.secretCache, h.k8s.CoreV1(), h.k8s.CoreV1(), sa)
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

	installScript, err := is(context.WithValue(context.Background(), tls.InternalAPI, hasHostPort), base64.URLEncoding.EncodeToString(hash[:]), envVars, "", dataDir)
	if err != nil {
		return nil, err
	}

	// For CAPR or elemental as the infrastructure provider, we only need to set the system agent
	// install script in the bootstrap secret.
	//
	// For CAPR, additional userdata is defined in the machine config and it will be merged with
	// install script from the secret by rancher-machine.
	if machine.Spec.InfrastructureRef.APIGroup == capr.RKEMachineAPIGroup ||
		machine.Spec.InfrastructureRef.APIGroup == capr.RKEAPIGroup ||
		machine.Spec.InfrastructureRef.APIGroup == "elemental.cattle.io" {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"value": installScript,
			},
			Type: capr.SecretTypeBootstrap,
		}, nil
	}

	if os := machine.GetLabels()[capr.CattleOSLabel]; os == capr.WindowsMachineOS {
		return nil, fmt.Errorf("windows is not currently supported with external capi infrastructure providers")
	}

	// For external capi infrastructure providers, we merge the user-provided
	// userdata here.
	userdata := make(map[string]any)

	if bootstrap.Spec.Userdata != nil && bootstrap.Spec.Userdata.InlineUserdata != "" {
		err = yaml.Unmarshal([]byte(bootstrap.Spec.Userdata.InlineUserdata), &userdata)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal inline userdata")
		}
	}

	var output bytes.Buffer

	// We need to gzip the system agent install script
	// because some cloud providers have a userdata size limit.
	gz := gzip.NewWriter(&output)
	if _, err = gz.Write(installScript); err != nil {
		return nil, err
	}
	if err = gz.Close(); err != nil {
		return nil, err
	}

	content := base64.StdEncoding.EncodeToString(output.Bytes())

	command := "sh"
	path := "/usr/local/custom_script/install.sh"

	// Copy system agent install script
	writeFiles := []any{
		map[string]string{
			"content":     content,
			"encoding":    "gzip+b64",
			"path":        path,
			"permissions": "0600",
		},
	}

	if userWriteFiles, ok := userdata["write_files"]; ok {
		userWriteFiles, ok := userWriteFiles.([]any)
		if !ok {
			return nil, fmt.Errorf("error parsing userdata write_files")
		}
		writeFiles = append(writeFiles, userWriteFiles...)
	}

	userdata["write_files"] = writeFiles

	// Call system agent install script
	runcmd := []any{fmt.Sprintf("%s %s", command, path)}

	if userRunCmd, ok := userdata["runcmd"]; ok {
		userRunCmd, ok := userRunCmd.([]any)
		if !ok {
			return nil, fmt.Errorf("error parsing userdata runcmd")
		}
		runcmd = append(runcmd, userRunCmd...)
	}

	userdata["runcmd"] = runcmd

	userdataBytes, err := yaml.Marshal(userdata)
	if err != nil {
		return nil, fmt.Errorf("error marshaling userdata")
	}

	userdataBytes = append([]byte("#cloud-config\n"), userdataBytes...)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": userdataBytes,
		},
		Type: capr.SecretTypeBootstrap,
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

func (h *handler) getEnvVars(controlPlane *rkev1.RKEControlPlane) ([]corev1.EnvVar, error) {
	var result []corev1.EnvVar
	for _, env := range controlPlane.Spec.AgentEnvVars {
		// Disallow user supplied system agent var dir env var in favor of spec.systemAgent
		if env.Name == capr.SystemAgentDataDirEnvVar {
			continue
		}
		result = append(result, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}
	if dir := controlPlane.Spec.DataDirectories.SystemAgent; dir != "" {
		result = append(result, corev1.EnvVar{
			Name:  capr.SystemAgentDataDirEnvVar,
			Value: dir,
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

	if !capiCluster.Spec.ControlPlaneRef.IsDefined() || capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" {
		return nil, nil, nil
	}
	controlPlane, err := h.rkeControlPlanes.Get(bootstrap.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return nil, nil, err
	}

	envVars, err := h.getEnvVars(controlPlane)
	if err != nil {
		return nil, nil, err
	}

	dataDir := capr.GetDistroDataDir(controlPlane)

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

	bootstrapSecret, err := h.getBootstrapSecret(sa.Namespace, sa.Name, envVars, machine, bootstrap, dataDir)
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
	if v, ok := bootstrap.Labels[capi.ClusterNameLabel]; ok && v != "" && bootstrap.Spec.ClusterName != v {
		logrus.Debugf("[rkebootstrap] %s/%s: setting cluster name", bootstrap.Namespace, bootstrap.Name)
		bootstrap = bootstrap.DeepCopy()
		bootstrap.Spec.ClusterName = v
		return h.rkeBootstrap.Update(bootstrap)
	}

	return h.reconcileMachinePreTerminateAnnotation(bootstrap)
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

	if !ptr.Deref(capiCluster.Status.Initialization.InfrastructureProvisioned, false) {
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
			status.Initialization.DataSecretCreated = ptr.To(true)
			logrus.Debugf("[rkebootstrap] %s/%s: setting dataSecretName: %s", bootstrap.Namespace, bootstrap.Name, *status.DataSecretName)
		}
		result = append(result, bootstrapSecret)
	}

	result = append(result, objs...)
	return result, status, nil
}

// rancherDeploymentHasHostPort returns true if the rancher deployment exposes a host port,
// which is the case when the local cluster is provisioned via rancherd.
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
	labels[capr.BackupLabel] = "true"
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
	return h.reconcileMachinePreTerminateAnnotation(bootstrap)
}

// reconcileMachinePreTerminateAnnotation reconciles the machine object that owns the bootstrap.
// Its primary purpose is to manage the pre-terminate.delete.hook.machine.x-k8s.io annotation on
// etcd machines. That hook is used to stop CAPI from tearing down the backing infrastructure
// before Rancher has safely removed the etcd member.
//
// The hook is normally added before delete starts. Once the machine is deleting, Rancher keeps
// the hook in place until it is safe to let delete continue.
//
// The hook is removed to allow infrastructure cleanup in the following cases:
//   - force remove is requested, or the machine is not an etcd machine
//   - the cluster/control plane objects needed for safe removal are missing or already deleting
//   - Rancher has confirmed no machine is still joined to the deleting machine, a replacement etcd
//     machine is ready, and the old etcd member has been safely removed
//   - the deleting machine no longer has a downstream NodeRef after replacement readiness has already
//     been confirmed
//
// Notably, CAPI controllers do not trigger deletion of the RKEBootstrap object while the
// corresponding machine still has the pre-terminate hook. Because of that, Rancher relies on
// this reconciliation path to perform safe etcd removal and decide when the hook can be removed.
func (h *handler) reconcileMachinePreTerminateAnnotation(bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	machine, err := capr.GetMachineByOwner(h.machineCache, bootstrap)
	if err != nil {
		if errors.Is(err, capr.ErrNoMachineOwnerRef) || apierrors.IsNotFound(err) {
			// If the bootstrap no longer has an owning machine, there is nothing left to manage here.
			return bootstrap, nil
		}
		return bootstrap, err
	}

	_, isEtcd := machine.Labels[capr.EtcdRoleLabel]
	logrus.Tracef("[rkebootstrap] %s/%s: evaluating machine %s/%s for pre-terminate hook reconciliation (etcd=%t, machineDeleting=%t, bootstrapDeleting=%t, nodeRef=%t)",
		bootstrap.Namespace, bootstrap.Name,
		machine.Namespace, machine.Name,
		isEtcd,
		!machine.DeletionTimestamp.IsZero(),
		!bootstrap.DeletionTimestamp.IsZero(),
		machine.Status.NodeRef.IsDefined(),
	)

	forceRemove, ok := bootstrap.Annotations[capr.ForceRemoveEtcdAnnotation]
	if (ok && strings.ToLower(forceRemove) == "true") || !isEtcd {
		// This delete path only protects etcd membership. If this machine is not etcd, or the caller asked
		// for force removal, we should not hold the machine behind the pre-terminate hook.
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because etcd protection does not apply (etcd=%t, forceRemove=%q)",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
			isEtcd,
			forceRemove,
		)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	// Add the hook before delete starts. Later, when CAPI begins deleting the machine, delete will pause
	// here and Rancher will get one last chance to do etcd-safe removal first.
	if machine.DeletionTimestamp.IsZero() && bootstrap.DeletionTimestamp.IsZero() {
		logrus.Tracef("[rkebootstrap] %s/%s: ensuring pre-terminate hook on machine %s/%s before delete starts",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
		)
		if val, ok := machine.GetAnnotations()[capiMachinePreTerminateAnnotation]; !ok || val != capiMachinePreTerminateAnnotationOwner {
			machine = machine.DeepCopy()
			if machine.Annotations == nil {
				machine.Annotations = make(map[string]string)
			}
			machine.Annotations[capiMachinePreTerminateAnnotation] = capiMachinePreTerminateAnnotationOwner
			_, err = h.machineClient.Update(machine)
			if err != nil {
				return bootstrap, err
			}
		}
		return bootstrap, nil
	}

	// Start of safe removal validations

	if bootstrap.Spec.ClusterName == "" {
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because cluster name is empty",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
		)
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster label %s was not found in bootstrap labels, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capi.ClusterNameLabel)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	capiCluster, err := h.capiClusterCache.Get(bootstrap.Namespace, bootstrap.Spec.ClusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the CAPI cluster is missing",
				bootstrap.Namespace, bootstrap.Name,
				machine.Namespace, machine.Name,
			)
			logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s was not found, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, bootstrap.Namespace, bootstrap.Spec.ClusterName)
			return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if !capiCluster.Spec.ControlPlaneRef.IsDefined() {
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the control plane reference is missing",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
		)
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s controlplane object reference was nil, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Name)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	cp, err := h.rkeControlPlanes.Get(capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the RKEControlPlane is missing",
				bootstrap.Namespace, bootstrap.Name,
				machine.Namespace, machine.Name,
			)
			logrus.Warnf("[rkebootstrap] %s/%s: RKEControlPlane %s/%s was not found, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
			return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if !cp.DeletionTimestamp.IsZero() || !capiCluster.DeletionTimestamp.IsZero() {
		// If the whole control plane or whole cluster is being deleted, this machine is no longer being
		// removed as a single etcd member change. In that case we should not keep this hook.
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the cluster or control plane is deleting (controlPlaneDeleting=%t, clusterDeleting=%t)",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
			!cp.DeletionTimestamp.IsZero(),
			!capiCluster.DeletionTimestamp.IsZero(),
		)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	// Read plan secrets now because they carry the most current join state. During rollout, another node
	// may still be in bootstrap and still depend on this machine even if the higher-level objects already exist.
	planSecret, err := h.secretCache.Get(bootstrap.Namespace, capr.PlanSecretFromBootstrapName(bootstrap.Name))
	if err != nil && !apierrors.IsNotFound(err) {
		return bootstrap, fmt.Errorf("error retrieving plan secret to validate it was not an init node: %v", err)
	}

	planSecrets, err := h.secretCache.List(bootstrap.Namespace, labels.SelectorFromSet(map[string]string{
		capi.ClusterNameLabel: bootstrap.Spec.ClusterName,
	}))
	if err != nil {
		return bootstrap, fmt.Errorf("error encountered list plansecrets to validate etcd safe removal: %v", err)
	}
	logrus.Tracef("[rkebootstrap] %s/%s: loaded delete state for machine %s/%s (hasPlanSecret=%t, planSecrets=%d)",
		bootstrap.Namespace, bootstrap.Name,
		machine.Namespace, machine.Name,
		planSecret != nil,
		len(planSecrets),
	)

	if planSecret != nil {
		// Wait until no other machine still says it is joined to this one. This check matters early in the
		// delete path: if a replacement machine is still joining through this node, deleting the node now
		// can break that replacement before it becomes stable.
		joinURL := planSecret.Annotations[capr.JoinURLAnnotation]
		if joinedMachine, joined := machineStillJoinedToJoinURL(planSecrets, joinURL); joined {
			logrus.Debugf("[rkebootstrap] %s/%s: waiting: deleting etcd machine %s/%s is still the join target for machine %s",
				bootstrap.Namespace, bootstrap.Name,
				machine.Namespace, machine.Name,
				joinedMachine,
			)
			h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 5*time.Second)
			return bootstrap, generic.ErrSkip
		}
	}

	// Rancher removes the member by talking to the downstream cluster. If that kubeconfig is already gone,
	// there is nothing left to check or update, so we must release the hook.
	kcSecret, err := h.secretCache.Get(bootstrap.Namespace, secret.Name(bootstrap.Spec.ClusterName, secret.Kubeconfig))
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the downstream kubeconfig secret is missing",
				bootstrap.Namespace, bootstrap.Name,
				machine.Namespace, machine.Name,
			)
			return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kcSecret.Data["value"])
	if err != nil {
		return bootstrap, err
	}

	// Before we remove the old member, wait for a real replacement. This is the main timing gate in this
	// function: Rancher should not remove the old etcd member until another etcd machine has joined and
	// proved it is stable enough to carry the cluster forward.
	replacementReady, err := h.replacementEtcdMachineReady(bootstrap, machine, planSecrets)
	if err != nil {
		return bootstrap, err
	}
	if !replacementReady {
		logrus.Debugf("[rkebootstrap] %s/%s: waiting: deleting etcd machine %s/%s does not yet have a replacement machine with NodeReady=True and passed plan probes",
			bootstrap.Namespace, bootstrap.Name, machine.Namespace, machine.Name)
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 5*time.Second)
		return bootstrap, generic.ErrSkip
	}
	logrus.Debugf("[rkebootstrap] %s/%s: deleting etcd machine %s/%s has a replacement machine ready for safe removal",
		bootstrap.Namespace, bootstrap.Name,
		machine.Namespace, machine.Name,
	)

	if !machine.Status.NodeRef.IsDefined() {
		// At this point, a missing NodeRef means there is no downstream Node object left for Rancher to mark
		// for safe removal. There is nothing more this hook can do, so release it.
		logrus.Infof("[rkebootstrap] No associated node found for machine %s/%s in cluster %s, ensuring machine pre-terminate annotation is removed", machine.Namespace, machine.Name, bootstrap.Spec.ClusterName)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	// The actual etcd-member removal is async. Rancher first marks the downstream Node for removal, then
	// waits for the downstream controller to confirm the member is gone. Only after that is it safe to let
	// CAPI delete the backing infrastructure.
	removed, err := etcdmgmt.SafelyRemoved(restConfig, capr.GetRuntimeCommand(cp.Spec.KubernetesVersion), machine.Status.NodeRef.Name)
	if err != nil {
		return bootstrap, err
	}
	logrus.Debugf("[rkebootstrap] %s/%s: safe remove for machine %s/%s returned %t for node %s",
		bootstrap.Namespace, bootstrap.Name,
		machine.Namespace, machine.Name,
		removed,
		machine.Status.NodeRef.Name,
	)
	if !removed {
		h.rkeBootstrap.EnqueueAfter(bootstrap.Namespace, bootstrap.Name, 5*time.Second)
		return bootstrap, generic.ErrSkip
	}
	return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
}

// ensureMachinePreTerminateAnnotationRemoved removes the pre-terminate annotation from a CAPI machine when we removing the rkebootstrap, indicating the infrastructure can be deleted.
func (h *handler) ensureMachinePreTerminateAnnotationRemoved(bootstrap *rkev1.RKEBootstrap, machine *capi.Machine) (*rkev1.RKEBootstrap, error) {
	if machine == nil || machine.Annotations == nil {
		return bootstrap, nil
	}

	var err error
	if _, ok := machine.GetAnnotations()[capiMachinePreTerminateAnnotation]; ok {
		machine = machine.DeepCopy()
		delete(machine.Annotations, capiMachinePreTerminateAnnotation)
		_, err = h.machineClient.Update(machine)
	}
	return bootstrap, err
}

// replacementEtcdMachineReady returns true only when Rancher can see a real replacement for the etcd
// machine that is being deleted.
//
// Each check here matches a later point in the replacement machine lifecycle:
// * etcd role label: only another etcd machine can replace this member
// * join URL: the machine has progressed far enough in bootstrap to act as a join target
// * plan probes passed: Rancher has seen the current plan become healthy at least once
// * machine node ready: the CAPI machine controller has already mirrored node readiness onto the local machine object
// * not deleting: the replacement is still expected to stay in the cluster
func (h *handler) replacementEtcdMachineReady(bootstrap *rkev1.RKEBootstrap, deletingMachine *capi.Machine, planSecrets []*corev1.Secret) (bool, error) {
	if deletingMachine == nil {
		return false, nil
	}

	for _, ps := range planSecrets {
		// The replacement must also carry the etcd role.
		if ps.GetLabels()[capr.EtcdRoleLabel] != "true" {
			continue
		}

		// We need a real machine behind the plan secret, otherwise this secret is not enough to trust.
		machineName := ps.GetLabels()[capr.MachineNameLabel]
		if machineName == "" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate skipped because the machine name label is missing", bootstrap.Namespace, bootstrap.Name)
			continue
		}
		machineNamespace := ps.GetLabels()[capr.MachineNamespaceLabel]
		if machineNamespace == "" {
			machineNamespace = bootstrap.Namespace
		}
		if machineName == deletingMachine.Name && machineNamespace == deletingMachine.Namespace {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because it is the deleting machine", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}

		// The join URL appears after bootstrap has advanced enough for this machine to act as a join target.
		// Before that point, Rancher should still treat the replacement as too early.
		if ps.GetAnnotations()[capr.JoinURLAnnotation] == "" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because join URL is not set", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}
		// This tells us Rancher has already seen the current plan become healthy at least once. That makes
		// it a better late-bootstrap signal than just "the machine object exists".
		if ps.GetAnnotations()[capr.PlanProbesPassedAnnotation] == "" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because plan probes have not passed yet (joinURL=%s)",
				bootstrap.Namespace, bootstrap.Name,
				machineNamespace, machineName,
				ps.GetAnnotations()[capr.JoinURLAnnotation],
			)
			continue
		}

		machine, err := h.machineCache.Get(machineNamespace, machineName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because the machine object was not found", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
				continue
			}
			return false, err
		}

		// The machine controller mirrors node readiness onto the local machine object, so we can
		// validate replacement readiness without querying the downstream cluster directly.
		nodeReady := conditions.IsTrue(machine, capi.MachineNodeReadyCondition)
		logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s evaluated with deleting=%t, nodeReady=%t, phase=%s, nodeRef=%t",
			bootstrap.Namespace, bootstrap.Name,
			machineNamespace, machineName,
			!machine.DeletionTimestamp.IsZero(),
			nodeReady,
			machine.Status.Phase,
			machine.Status.NodeRef.IsDefined(),
		)
		if machine.DeletionTimestamp.IsZero() && machine.Status.NodeRef.IsDefined() && nodeReady {
			logrus.Debugf("[rkebootstrap] %s/%s: replacement candidate %s/%s is ready", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			return true, nil
		}
	}

	return false, nil
}

func machineStillJoinedToJoinURL(planSecrets []*corev1.Secret, joinURL string) (string, bool) {
	if joinURL == "" {
		return "", false
	}

	for _, ps := range planSecrets {
		joinedTo := ps.GetAnnotations()[capr.JoinedToAnnotation]
		if joinedTo == "" {
			continue
		}
		if joinedTo == joinURL {
			return ps.GetLabels()[capr.MachineNameLabel], true
		}
	}

	return "", false
}
