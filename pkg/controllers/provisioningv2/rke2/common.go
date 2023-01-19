package rke2

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rancher/channelserver/pkg/model"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/channelserver"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	AddressAnnotation = "rke.cattle.io/address"
	ClusterNameLabel  = "rke.cattle.io/cluster-name"
	// ClusterSpecAnnotation is used to define the cluster spec used to generate the rkecontrolplane object as an annotation on the object
	ClusterSpecAnnotation     = "rke.cattle.io/cluster-spec"
	ControlPlaneRoleLabel     = "rke.cattle.io/control-plane-role"
	DrainAnnotation           = "rke.cattle.io/drain-options"
	DrainDoneAnnotation       = "rke.cattle.io/drain-done"
	DrainErrorAnnotation      = "rke.cattle.io/drain-error"
	EtcdRoleLabel             = "rke.cattle.io/etcd-role"
	InitNodeLabel             = "rke.cattle.io/init-node"
	InitNodeMachineIDLabel    = "rke.cattle.io/init-node-machine-id"
	InternalAddressAnnotation = "rke.cattle.io/internal-address"
	JoinURLAnnotation         = "rke.cattle.io/join-url"
	LabelsAnnotation          = "rke.cattle.io/labels"
	MachineIDLabel            = "rke.cattle.io/machine-id"
	MachineNameLabel          = "rke.cattle.io/machine-name"
	MachineTemplateHashLabel  = "rke.cattle.io/machine-template-hash"
	RKEMachinePoolNameLabel   = "rke.cattle.io/rke-machine-pool-name"
	MachineNamespaceLabel     = "rke.cattle.io/machine-namespace"
	MachineRequestType        = "rke.cattle.io/machine-request"
	MachineUIDLabel           = "rke.cattle.io/machine"
	NodeNameLabel             = "rke.cattle.io/node-name"
	PlanSecret                = "rke.cattle.io/plan-secret-name"
	PostDrainAnnotation       = "rke.cattle.io/post-drain"
	PreDrainAnnotation        = "rke.cattle.io/pre-drain"
	RoleLabel                 = "rke.cattle.io/service-account-role"
	SecretTypeMachinePlan     = "rke.cattle.io/machine-plan"
	TaintsAnnotation          = "rke.cattle.io/taints"
	UnCordonAnnotation        = "rke.cattle.io/uncordon"
	WorkerRoleLabel           = "rke.cattle.io/worker-role"

	MachineTemplateClonedFromGroupVersionAnn = "rke.cattle.io/cloned-from-group-version"
	MachineTemplateClonedFromKindAnn         = "rke.cattle.io/cloned-from-kind"
	MachineTemplateClonedFromNameAnn         = "rke.cattle.io/cloned-from-name"

	CattleOSLabel    = "cattle.io/os"
	DefaultMachineOS = "linux"
	WindowsMachineOS = "windows"

	DefaultMachineConfigAPIVersion = "rke-machine-config.cattle.io/v1"
	RKEMachineAPIVersion           = "rke-machine.cattle.io/v1"
	RKEAPIVersion                  = "rke.cattle.io/v1"

	Provisioned         = condition.Cond("Provisioned")
	Updated             = condition.Cond("Updated")
	Reconciled          = condition.Cond("Reconciled")
	Ready               = condition.Cond("Ready")
	Waiting             = condition.Cond("Waiting")
	Pending             = condition.Cond("Pending")
	Removed             = condition.Cond("Removed")
	PlanApplied         = condition.Cond("PlanApplied")
	InfrastructureReady = condition.Cond(capi.InfrastructureReadyCondition)

	RuntimeK3S  = "k3s"
	RuntimeRKE2 = "rke2"

	RoleBootstrap = "bootstrap"
	RolePlan      = "plan"
)

var (
	ErrNoMachineOwnerRef            = errors.New("no machine owner ref")
	ErrNoMatchingControllerOwnerRef = errors.New("no matching controller owner ref")
	labelAnnotationMatch            = regexp.MustCompile(`^((rke\.cattle\.io)|((?:machine\.)?cluster\.x-k8s\.io))/`)
	windowsDrivers                  = map[string]struct{}{
		"vmwarevsphere": {},
	}
)

// WindowsCheck return a bool based on if the driver is marked as
// supporting Windows
func WindowsCheck(driver string) bool {
	_, ok := windowsDrivers[driver]
	return ok
}

func MachineStateSecretName(machineName string) string {
	return name.SafeConcatName(machineName, "machine", "state")
}

func GetMachineByOwner(machineCache capicontrollers.MachineCache, obj metav1.Object) (*capi.Machine, error) {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.APIVersion == capi.GroupVersion.String() && owner.Kind == "Machine" {
			return machineCache.Get(obj.GetNamespace(), owner.Name)
		}
	}

	return nil, ErrNoMachineOwnerRef
}

func GetRuntimeCommand(kubernetesVersion string) string {
	return strings.ToLower(GetRuntime(kubernetesVersion))
}

func GetRuntimeServerUnit(kubernetesVersion string) string {
	if GetRuntime(kubernetesVersion) == RuntimeK3S {
		return RuntimeK3S
	}
	return RuntimeRKE2 + "-server"
}

func GetRuntimeAgentUnit(kubernetesVersion string) string {
	return GetRuntimeCommand(kubernetesVersion) + "-agent"
}

func GetRuntimeEnv(kubernetesVersion string) string {
	return strings.ToUpper(GetRuntime(kubernetesVersion))
}

func GetRuntime(kubernetesVersion string) string {
	if strings.Contains(kubernetesVersion, RuntimeK3S) {
		return RuntimeK3S
	}
	return RuntimeRKE2
}

func GetKDMReleaseData(ctx context.Context, controlPlane *rkev1.RKEControlPlane) *model.Release {
	if controlPlane == nil || controlPlane.Spec.KubernetesVersion == "" {
		return nil
	}
	release := channelserver.GetReleaseConfigByRuntimeAndVersion(ctx, GetRuntime(controlPlane.Spec.KubernetesVersion), controlPlane.Spec.KubernetesVersion)
	return &release
}

// GetFeatureVersion retrieves a feature version (string) for a given controlPlane based on the version/runtime of the project. It will return 0.0.0 (semver) if the KDM data is valid, but the featureVersion isn't defined.
func GetFeatureVersion(ctx context.Context, controlPlane *rkev1.RKEControlPlane, featureKey string) (string, error) {
	if controlPlane == nil {
		return "", fmt.Errorf("error retrieving feature version as controlPlane was nil")
	}

	release := GetKDMReleaseData(ctx, controlPlane)
	if release == nil {
		return "", fmt.Errorf("KDM release data was nil for controlplane %s/%s", controlPlane.Namespace, controlPlane.Name)
	}

	version := release.FeatureVersions[featureKey]
	if version == "" {
		version = "0.0.0"
	}

	return version, nil
}

func GetRuntimeSupervisorPort(kubernetesVersion string) int {
	if GetRuntime(kubernetesVersion) == RuntimeRKE2 {
		return 9345
	}
	return 6443
}

func IsOwnedByMachine(bootstrapCache rkecontroller.RKEBootstrapCache, machineName string, sa *corev1.ServiceAccount) (bool, error) {
	for _, owner := range sa.OwnerReferences {
		if owner.Kind == "RKEBootstrap" {
			bootstrap, err := bootstrapCache.Get(sa.Namespace, owner.Name)
			if err != nil {
				return false, err
			}
			for _, owner := range bootstrap.OwnerReferences {
				if owner.Kind == "Machine" && owner.Name == machineName {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// PlanSACheck checks the given plan service account to ensure that it matches the machine that is passed,
// and makes sure that the plan service account is owned by the machine in question.
func PlanSACheck(bootstrapCache rkecontroller.RKEBootstrapCache, machineName string, planSA *corev1.ServiceAccount) error {
	if planSA == nil {
		return fmt.Errorf("planSA was nil during planSA check for machineName %s", machineName)
	}
	if machineName == "" {
		return fmt.Errorf("planSA %s/%s compared machine name was blank", planSA.Namespace, planSA.Name)
	}
	if planSA.Labels[MachineNameLabel] != machineName ||
		planSA.Labels[RoleLabel] != RolePlan ||
		planSA.Labels[PlanSecret] == "" {
		return fmt.Errorf("planSA %s/%s does not have correct labels", planSA.Namespace, planSA.Name)
	}
	if foundParent, err := IsOwnedByMachine(bootstrapCache, machineName, planSA); err != nil {
		return err
	} else if !foundParent {
		return fmt.Errorf("planSA %s/%s no parent found for planSA, was not owned by machine %s", planSA.Namespace, planSA.Name, machineName)
	}
	return nil
}

// GetPlanSecretName will return the plan secret name that is assigned to the plan service account
func GetPlanSecretName(planSA *corev1.ServiceAccount) (string, error) {
	if planSA == nil {
		return "", fmt.Errorf("planSA was nil")
	}
	if planSA.Labels[PlanSecret] == "" {
		return "", fmt.Errorf("planSA %s/%s plan secret label was not set", planSA.Namespace, planSA.Name)
	}
	return planSA.Labels[PlanSecret], nil
}

// GetPlanServiceAccountTokenSecret retrieves the secret that corresponds to the plan service account that is passed in. It will create a secret if one does not
// already exist for the plan service account.
func GetPlanServiceAccountTokenSecret(secretClient corecontrollers.SecretController, planSA *corev1.ServiceAccount) (*corev1.Secret, bool, error) {
	if planSA == nil {
		return nil, false, fmt.Errorf("planSA was nil")
	}
	sName := serviceaccounttoken.ServiceAccountSecretName(planSA)
	secret, err := secretClient.Cache().Get(planSA.Namespace, sName)
	if err != nil {
		if !apierror.IsNotFound(err) {
			return nil, false, err
		}
		sc := serviceaccounttoken.SecretTemplate(planSA)
		secret, err = secretClient.Create(sc)
		if err != nil {
			if !apierror.IsAlreadyExists(err) {
				return nil, false, err
			}
			secret, err = secretClient.Cache().Get(planSA.Namespace, sName)
			if err != nil {
				return nil, false, err
			}
		}
	}
	// wait for token to be populated
	if !PlanServiceAccountTokenReady(planSA, secret) {
		return secret, true, fmt.Errorf("planSA %s/%s token secret %s/%s was not ready for consumption yet", planSA.Namespace, planSA.Name, secret.Namespace, secret.Name)
	}
	return secret, true, nil
}

func PlanServiceAccountTokenReady(planSA *corev1.ServiceAccount, tokenSecret *corev1.Secret) bool {
	if planSA == nil || tokenSecret == nil {
		return false
	}
	if tokenSecret.Name != serviceaccounttoken.ServiceAccountSecretName(planSA) {
		return false
	}
	if v, ok := tokenSecret.Data[corev1.ServiceAccountTokenKey]; ok {
		if len(v) == 0 {
			return false
		}
	} else {
		return false
	}
	return true
}

func PlanSecretFromBootstrapName(bootstrapName string) string {
	return name.SafeConcatName(bootstrapName, "machine", "plan")
}

func DoRemoveAndUpdateStatus(obj metav1.Object, doRemove func() (string, error), enqueueAfter func(string, string, time.Duration)) error {
	if !Provisioned.IsTrue(obj) || !Waiting.IsTrue(obj) || !Pending.IsTrue(obj) {
		// Ensure the Removed obj appears in the UI.
		Provisioned.SetStatus(obj, "True")
		Waiting.SetStatus(obj, "True")
		Pending.SetStatus(obj, "True")
	}
	message, err := doRemove()
	if errors.Is(err, generic.ErrSkip) {
		// If generic.ErrSkip is returned, we don't want to update the status.
		return err
	}

	if err != nil {
		Removed.SetError(obj, "", err)
	} else if message == "" {
		Removed.SetStatusBool(obj, true)
		Removed.Reason(obj, "")
		Removed.Message(obj, "")
	} else {
		Removed.SetStatus(obj, "Unknown")
		Removed.Reason(obj, "Waiting")
		Removed.Message(obj, message)
		enqueueAfter(obj.GetNamespace(), obj.GetName(), 5*time.Second)
		// generic.ErrSkip will mark the cluster as reconciled, but not remove the finalizer.
		// The finalizer shouldn't be removed until other objects have all been removed.
		err = generic.ErrSkip
	}

	return err
}

func GetMachineDeletionStatus(machines []*capi.Machine) (string, error) {
	sort.Slice(machines, func(i, j int) bool {
		return machines[i].Name < machines[j].Name
	})
	for _, machine := range machines {
		if machine.Status.FailureReason != nil && *machine.Status.FailureReason == capierrors.DeleteMachineError {
			return "", fmt.Errorf("error deleting machine [%s], machine must be deleted manually", machine.Name)
		}
		return fmt.Sprintf("waiting for machine [%s] to delete", machine.Name), nil
	}

	return "", nil
}

// GetMachineFromNode attempts to find the corresponding machine for an etcd snapshot that is found in the configmap. If the machine list is successful, it will return true on the boolean, otherwise, it can be assumed that a false, nil, and defined error indicate the machine does not exist.
func GetMachineFromNode(machineCache capicontrollers.MachineCache, nodeName string, cluster *provv1.Cluster) (bool, *capi.Machine, error) {
	ls, err := labels.Parse(fmt.Sprintf("%s=%s", capi.ClusterLabelName, cluster.Name))
	if err != nil {
		return false, nil, err
	}
	machines, err := machineCache.List(cluster.Namespace, ls)
	if err != nil {
		return false, nil, err
	}
	for _, machine := range machines {
		if machine.Status.NodeRef != nil && machine.Status.NodeRef.Name == nodeName {
			return true, machine, nil
		}
	}
	return true, nil, fmt.Errorf("unable to find node %s in machines", nodeName)
}

// GetMachineByID attempts to find the corresponding machine for an etcd snapshot that is found in the configmap. If the machine list is successful, it will return true on the boolean, otherwise, it can be assumed that a false, nil, and defined error indicate the machine does not exist.
func GetMachineByID(machineCache capicontrollers.MachineCache, machineID, clusterNamespace, clusterName string) (bool, *capi.Machine, error) {
	machines, err := machineCache.List(clusterNamespace, labels.SelectorFromSet(map[string]string{
		ClusterNameLabel: clusterName,
		MachineIDLabel:   machineID,
	}))
	if err != nil || len(machines) > 1 {
		return false, nil, err
	}
	if len(machines) == 1 {
		return true, machines[0], nil
	}
	return true, nil, fmt.Errorf("unable to find machine by ID %s for cluster %s", machineID, clusterName)
}

func CopyPlanMetadataToSecret(secret *corev1.Secret, metadata *plan.Metadata) {
	if metadata == nil {
		return
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}

	CopyMapWithExcludes(secret.Labels, metadata.Labels, nil)
	CopyMapWithExcludes(secret.Annotations, metadata.Annotations, nil)
}

// CopyMap will copy the items from source to destination. It will only copy items that have keys that start with
// rke.cattle.io/, cluster.x-k8s.io/. or machine.cluster.x-k8s.io/.
func CopyMap(destination map[string]string, source map[string]string) {
	CopyMapWithExcludes(destination, source, nil)
}

// CopyMapWithExcludes will copy the items from source to destination, excluding all items whose keys are in excludes.
// It will only copy items that have keys that start with rke.cattle.io/, cluster.x-k8s.io/. or
// machine.cluster.x-k8s.io/.
func CopyMapWithExcludes(destination map[string]string, source map[string]string, excludes map[string]struct{}) {
	for k, v := range source {
		if !labelAnnotationMatch.MatchString(k) {
			continue
		}
		if _, ok := excludes[k]; !ok {
			destination[k] = v
		}
	}
}

func SortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var errNilObject = errors.New("cannot get capi cluster for nil object")

// GetCAPIClusterFromLabel takes a runtime.Object and will attempt to find the label denoting which capi cluster it
// belongs to.
// If the object is nil, it cannot access to object or type metas, or the label is not present, it returns an error.
// If the object has the expected label, it will return the capi cluster object.
func GetCAPIClusterFromLabel(obj runtime.Object, cache capicontrollers.ClusterCache) (*capi.Cluster, error) {
	if obj == nil {
		return nil, errNilObject
	}
	data, err := data.Convert(obj)
	if err != nil {
		return nil, err
	}
	clusterName := data.String("metadata", "labels", capi.ClusterLabelName)
	if clusterName != "" {
		return cache.Get(data.String("metadata", "namespace"), clusterName)
	}
	return nil, fmt.Errorf("%s label not present on %s: %s/%s", capi.ClusterLabelName, obj.GetObjectKind().GroupVersionKind().Kind, data.String("metadata", "namespace"), data.String("metadata", "name"))
}

// GetOwnerCAPICluster takes an obj and will attempt to find the capi cluster owner reference.
// If the object is nil, it cannot access to object or type metas, the owner reference Kind or APIVersion do not match,
// or the object could not be found, it returns an error.
// If the owner reference exists and is valid, it will return the owning capi cluster object.
func GetOwnerCAPICluster(obj runtime.Object, cache capicontrollers.ClusterCache) (*capi.Cluster, error) {
	ref, namespace, err := GetOwnerFromGVK(capi.GroupVersion.String(), "Cluster", obj)
	if err != nil {
		return nil, err
	}
	return cache.Get(namespace, ref.Name)
}

// GetOwnerCAPIMachine takes an obj and will attempt to find the capi machine owner reference.
// If the object is nil, it cannot access to object or type metas, the owner reference Kind or APIVersion do not match,
// or the object could not be found, it returns an error.
// If the owner reference exists and is valid, it will return the owning capi machine object.
func GetOwnerCAPIMachine(obj runtime.Object, cache capicontrollers.MachineCache) (*capi.Machine, error) {
	ref, namespace, err := GetOwnerFromGVK(capi.GroupVersion.String(), "Machine", obj)
	if err != nil {
		return nil, err
	}
	return cache.Get(namespace, ref.Name)
}

// GetOwnerCAPIMachineSet takes an obj and will attempt to find the capi machine set owner reference.
// If the object is nil, it cannot access to object or type metas, the owner reference Kind or APIVersion do not match,
// or the object could not be found, it returns an error.
// If the owner reference exists and is valid, it will return the owning capi machine object.
func GetOwnerCAPIMachineSet(obj runtime.Object, cache capicontrollers.MachineSetCache) (*capi.MachineSet, error) {
	ref, namespace, err := GetOwnerFromGVK(capi.GroupVersion.String(), "MachineSet", obj)
	if err != nil {
		return nil, err
	}
	return cache.Get(namespace, ref.Name)
}

// GetOwnerFromGVK takes a runtime.Object, and will search for a controlling owner reference of kind apiVersion.
// If the object is nil, it cannot access to object or type metas, the owner reference Kind or APIVersion do not match,
// or the object could not be found, it returns an ErrNoMatchingControllerOwnerRef error.
// If the owner reference exists and is valid, it will return the owner reference and the namespace it belongs to.
func GetOwnerFromGVK(groupVersion, kind string, obj runtime.Object) (*metav1.OwnerReference, string, error) {
	if obj == nil {
		return nil, "", errNilObject
	}
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, "", err
	}
	ref := metav1.GetControllerOf(objMeta)
	if ref == nil || ref.Kind != kind || ref.APIVersion != groupVersion {
		return nil, "", ErrNoMatchingControllerOwnerRef
	}
	return ref, objMeta.GetNamespace(), nil
}
