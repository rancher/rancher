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
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
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

	if secret.Annotations[capr.BootstrapTokenAnnotation] != "true" {
		secret = secret.DeepCopy()
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}

		secret.Annotations[capr.BootstrapTokenAnnotation] = "true"

		secret, err = h.secretClient.Update(secret)
		if err != nil {
			return nil, fmt.Errorf("could not set bootstrap annotation on sa secret %s/%s: %w",
				secret.Namespace, secret.Name, err)
		}
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

	userdataBytes = append([]byte("## template: jinja\n#cloud-config\n"), userdataBytes...)

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

func (h *handler) assignPlanSecret(machine *capi.Machine, bootstrap *rkev1.RKEBootstrap, cluster *capi.Cluster) ([]runtime.Object, error) {
	planSecretName := capr.PlanSecretFromBootstrapName(bootstrap.Name)
	labels, annotations, err := getLabelsAndAnnotationsForPlanSecret(bootstrap, machine, cluster)
	if err != nil {
		return nil, err
	}

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

	return []runtime.Object{sa, secret, role, roleBinding}, nil
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
	objs, err := h.assignPlanSecret(machine, bootstrap, capiCluster)
	if err != nil {
		return nil, status, err
	}
	result = append(result, objs...)

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

func getLabelsAndAnnotationsForPlanSecret(bootstrap *rkev1.RKEBootstrap, machine *capi.Machine, cluster *capi.Cluster) (map[string]string, map[string]string, error) {
	labels := make(map[string]string, len(bootstrap.Labels)+3)
	labels[capr.MachineNameLabel] = machine.Name
	labels[capr.ClusterNameLabel] = bootstrap.Spec.ClusterName
	labels[capr.BackupLabel] = "true"

	lifecycleLabels, err := planv1alpha1.ObjToMachineLifecycleLabels(machine)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range lifecycleLabels {
		labels[k] = v
	}

	lifecycleLabels, err = planv1alpha1.ObjToClusterLifecycleLabels(cluster)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range lifecycleLabels {
		labels[k] = v
	}

	for k, v := range bootstrap.Labels {
		labels[k] = v
	}

	annotations := make(map[string]string, len(bootstrap.Annotations))
	for k, v := range bootstrap.Annotations {
		annotations[k] = v
	}

	return labels, annotations, nil
}

// OnRemove adds finalizer handling to the RKEBootstrap object, and is used to prevent deletion of the RKE Bootstrap
// when it is deleting and bootstrap is for an etcd node.
func (h *handler) OnRemove(_ string, bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	logrus.Debugf("[rkebootstrap] %s/%s: OnRemove invoked", bootstrap.Namespace, bootstrap.Name)
	return h.reconcileMachinePreTerminateAnnotation(bootstrap)
}

// reconcileMachinePreTerminateAnnotation reconciles the machine object that owns the bootstrap. It only reconciles the machine if it is an
// etcd machine. Its primary purpose is to manage the pre-terminate.delete.hook.machine.x-k8s.io annotation on the machine
// object, which is used to prevent premature tear down of infrastructure before it is ready to be teared down, i.e.
// allowing removal of an etcd member without causing quorum loss.
// The pre-terminate hook will be set on the machine object if the machine and bootstrap are not deleting, the corresponding
// CAPI cluster and RKEControlPlane are not deleting, and the force remove annotation is not set on the bootstrap.
// The annotation will be removed from the machine to allow infrastructure cleanup in the following cases:
// * The machine is deleting and the "safe remove" logic has fired and removed the etcd member from the etcd cluster
// * The bootstrap is missing the CAPI cluster label || the CAPI cluster controlPlaneRef is nil || the machine noderef is nil
// * Any of the following: CAPI kubeconfig secret, CAPI cluster object, RKEControlPlane object are not found
//
// Notably, CAPI controllers do not trigger a deletion of the RKEBootstrap object if a pre-terminate annotation exists on the corresponding machine object.
// This means we rely on the OnChange handler to perform node safe removal, when it sees that the corresponding machine is deleting.
func (h *handler) reconcileMachinePreTerminateAnnotation(bootstrap *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
	machine, err := capr.GetMachineByOwner(h.machineCache, bootstrap)
	if err != nil {
		if errors.Is(err, capr.ErrNoMachineOwnerRef) || apierrors.IsNotFound(err) {
			// If we did not find the machine by owner ref or the cache returned a not found, then noop.
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
		// If the force remove annotation is "true" or the node is not an etcd node, then ensure the machine pre terminate annotation is removed.
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because etcd protection does not apply (etcd=%t, forceRemove=%q)",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
			isEtcd,
			forceRemove,
		)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	// Only add the pre-terminate hook annotation if the corresponding machine and bootstrap are NOT deleting
	if machine.DeletionTimestamp.IsZero() && bootstrap.DeletionTimestamp.IsZero() {
		// annotate the CAPI machine with the pre-terminate.delete.hook.machine.cluster.x-k8s.io annotation if it is an etcd machine
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

	// Safe removal requires the deleting machine's downstream node name. Without a NodeRef, there is no
	// known downstream node to remove, so release the hook.
	if !machine.Status.NodeRef.IsDefined() {
		logrus.Infof("[rkebootstrap] No associated node found for machine %s/%s in cluster %s, ensuring machine pre-terminate annotation is removed", machine.Namespace, machine.Name, bootstrap.Spec.ClusterName)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	if bootstrap.Spec.ClusterName == "" {
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster label %s was not found in bootstrap labels, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capi.ClusterNameLabel)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	capiCluster, err := h.capiClusterCache.Get(bootstrap.Namespace, bootstrap.Spec.ClusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s was not found, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, bootstrap.Namespace, bootstrap.Spec.ClusterName)
			return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if !capiCluster.Spec.ControlPlaneRef.IsDefined() {
		logrus.Warnf("[rkebootstrap] %s/%s: CAPI cluster %s/%s controlplane object reference was nil, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Name)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	cp, err := h.rkeControlPlanes.Get(capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Warnf("[rkebootstrap] %s/%s: RKEControlPlane %s/%s was not found, ensuring machine pre-terminate annotation is removed", bootstrap.Namespace, bootstrap.Name, capiCluster.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
			return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
		}
		return bootstrap, err
	}

	if !cp.DeletionTimestamp.IsZero() || !capiCluster.DeletionTimestamp.IsZero() {
		// Cluster or control plane deletion does not require per-member protection.
		logrus.Tracef("[rkebootstrap] %s/%s: releasing pre-terminate hook for machine %s/%s because the cluster or control plane is deleting (controlPlaneDeleting=%t, clusterDeleting=%t)",
			bootstrap.Namespace, bootstrap.Name,
			machine.Namespace, machine.Name,
			!cp.DeletionTimestamp.IsZero(),
			!capiCluster.DeletionTimestamp.IsZero(),
		)
		return h.ensureMachinePreTerminateAnnotationRemoved(bootstrap, machine)
	}

	// Plan secrets record whether another machine still depends on this machine's join URL.
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
		// Do not remove a machine while another plan secret still references its join URL.
		joinURL := planSecret.Annotations[capr.JoinURLAnnotation]
		if joinURL != "" {
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
	}

	// Without the downstream kubeconfig, Rancher cannot perform member removal, so release the hook.
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

	// Wait for the elected replacement etcd machine to complete its plan and report NodeReady.
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

	// Member removal is asynchronous; keep the hook until the downstream controller confirms completion.
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

// replacementEtcdMachineReady reports whether another elected init etcd machine has completed its plan
// and reports NodeReady in CAPI.
func (h *handler) replacementEtcdMachineReady(bootstrap *rkev1.RKEBootstrap, deletingMachine *capi.Machine, planSecrets []*corev1.Secret) (bool, error) {
	if deletingMachine == nil {
		return false, nil
	}

	for _, ps := range planSecrets {
		// Only consider live machine-plan secrets.
		if !ps.DeletionTimestamp.IsZero() {
			continue
		}
		if ps.Type != capr.SecretTypeMachinePlan {
			continue
		}

		// Only consider the elected init etcd machine as a replacement.
		if ps.GetLabels()[capr.EtcdRoleLabel] != "true" {
			continue
		}
		if ps.GetLabels()[capr.InitNodeLabel] != "true" {
			continue
		}

		// Require a Machine for local cluster and readiness validation.
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

		// A join URL indicates that the candidate can accept joining members.
		if ps.GetAnnotations()[capr.JoinURLAnnotation] == "" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because join URL is not set", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}
		// Passed probes show that the candidate's current plan was healthy at least once.
		if ps.GetAnnotations()[planapi.PlanProbesPassedAnnotation] == "" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because plan probes have not passed yet", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
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

		if machine.Spec.ClusterName != deletingMachine.Spec.ClusterName {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because it belongs to a different cluster", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}
		if machine.GetLabels()[capr.EtcdRoleLabel] != "true" {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because the machine object does not carry the etcd role label", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}
		if !machine.DeletionTimestamp.IsZero() {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because the machine is deleting", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}
		if !machine.Status.NodeRef.IsDefined() {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because it has no NodeRef yet", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}

		// The machine controller mirrors node readiness onto the local machine object, so we can
		// validate replacement readiness without querying the downstream cluster directly.
		if !conditions.IsTrue(machine, capi.MachineNodeReadyCondition) {
			logrus.Tracef("[rkebootstrap] %s/%s: replacement candidate %s/%s skipped because NodeReady is not true", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
			continue
		}

		logrus.Debugf("[rkebootstrap] %s/%s: replacement candidate %s/%s is ready", bootstrap.Namespace, bootstrap.Name, machineNamespace, machineName)
		return true, nil
	}

	return false, nil
}

// machineStillJoinedToJoinURL reports whether any plan secret in planSecrets records joinURL as its
// "joined-to" target, meaning that machine joined the cluster through the machine advertising joinURL.
// It returns the machine-name label of the first matching plan secret and true if found.
func machineStillJoinedToJoinURL(planSecrets []*corev1.Secret, joinURL string) (string, bool) {
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
