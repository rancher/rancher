package operations

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/rancher/channelserver/pkg/model"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func init() {
	RegisterAdapter(rkev1.SchemeGroupVersion.WithKind("RKEControlPlane"), func(clients *wrangler.CAPIContext, unstructured *unstructured.Unstructured) (Adapter, error) {
		controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(unstructured.GetNamespace(), unstructured.GetName())
		if err != nil {
			return nil, err
		}
		return &CAPRAdapter{
			controlPlane: controlPlane,
			clients:      clients,
		}, nil
	})
	RegisterAdapter(provv1.SchemeGroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, unstructured *unstructured.Unstructured) (Adapter, error) {
		controlPlane, err := clients.RKE.RKEControlPlane().Cache().Get(unstructured.GetNamespace(), unstructured.GetName())
		if err != nil {
			return nil, err
		}
		return &CAPRAdapter{
			controlPlane: controlPlane,
			clients:      clients,
		}, nil
	})
}

// CAPRAdapter is an implementation of the Adapter interface for CAPR clusters.
type CAPRAdapter struct {
	controlPlane *rkev1.RKEControlPlane
	clients      *wrangler.CAPIContext
}

func (a *CAPRAdapter) ToS3ArgsEnvAndFiles(secret *corev1.Secret) ([]string, []string, []plan.File) {
	//TODO implement me
	panic("implement me")
}

func (a *CAPRAdapter) LoopbackAddress(_ *corev1.Secret) string {
	loopbackAddress := capr.GetLoopbackAddress(a.controlPlane)

	if utils.IsPlainIPV6(loopbackAddress) {
		loopbackAddress = fmt.Sprintf("[%s]", loopbackAddress)
	}

	return loopbackAddress
}

func (a *CAPRAdapter) ConfigDirectory(_ *corev1.Secret) string {
	return fmt.Sprintf("/etc/rancher/%s/config.yaml.d", a.RuntimeCommand())
}

func (a *CAPRAdapter) GetServerURL(secret *corev1.Secret) string {
	if secret == nil {
		return ""
	}

	if !planv1alpha1.HasMachineLifecycleLabels(secret) {
		return ""
	}

	ref, err := planv1alpha1.MachineLifecycleLabelsToObjectReference(secret)
	if err != nil {
		logrus.Errorf("error getting reference for machine lifecycle labels: %v", err)
		return ""
	}

	machine, err := a.clients.CAPI.Machine().Cache().Get(ref.Namespace, ref.Name)
	if err != nil {
		logrus.Errorf("error getting machine %s for machine lifecycle: %v", ref.Name, err)
		return ""
	}

	if len(machine.Status.Addresses) == 0 {
		return ""
	}

	var address string

	for _, addr := range machine.Status.Addresses {
		if addr.Type == capi.MachineExternalIP && address == "" {
			address = addr.Address
		} else if addr.Type == capi.MachineInternalIP {
			address = addr.Address
		}
	}

	return address
}

func (a *CAPRAdapter) GetSupervisorPort(_ *corev1.Secret) string {
	if a.RuntimeCommand() == "rke2" {
		return "9345"
	}
	return "6443"
}

// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
// all expected nodes.
// All machine-plans secrets are listed and compared to the count of CAPI machines for the cluster.
func (a *CAPRAdapter) WaitForRegister() (bool, error) {
	secretList, err := a.clients.Core.Secret().List(a.controlPlane.Namespace, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, a.controlPlane.Name),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return false, err
	}

	secrets := secretList.Items

	machines, err := a.clients.CAPI.Machine().Cache().List(a.controlPlane.Namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.controlPlane.Name,
	}))
	if err != nil {
		return false, err
	}

	// If the counts don't match upfront, we already know it's not a 1:1 match
	if len(secrets) != len(machines) {
		return false, nil
	}

	// Track the names of the machines we expect to see
	// Using a map[string]bool to easily check existence and prevent duplicates
	expectedMachines := make(map[string]bool, len(machines))
	for _, machine := range machines {
		expectedMachines[machine.Name] = true
	}

	// Verify that every secret maps to a unique expected machine
	for _, secret := range secrets {
		if secret.Labels == nil {
			return false, nil
		}

		machineName, exists := secret.Labels[capr.MachineNameLabel]

		// If the label is missing, or it maps to a machine name we haven't seen/already matched
		if !exists || !expectedMachines[machineName] {
			return false, nil
		}

		// Mark this machine as "matched" by deleting it from the expected map.
		// This naturally catches duplicate secrets pointing to the same machine.
		delete(expectedMachines, machineName)
	}

	// If the map is empty, we have a perfect, duplicate-free 1:1 match
	return len(expectedMachines) == 0, nil
}

// RuntimeCommand returns the command used to interact with the distro CLI (RKE2/K3s).
func (a *CAPRAdapter) RuntimeCommand() string {
	if strings.Contains(a.controlPlane.Spec.KubernetesVersion, "rke2") {
		return "rke2"
	}
	return "k3s"
}

// ServerUnit returns the systemd unit name for a distro server node.
func (a *CAPRAdapter) ServerUnit() string {
	if strings.Contains(a.controlPlane.Spec.KubernetesVersion, "rke2") {
		return "rke2-server"
	}
	return "k3s"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// If the cluster is using a custom data directory or secure probes, this information is extracted from the cluster object and rendered in.
func (a *CAPRAdapter) RenderProbes(secret *corev1.Secret, supervisor bool) (map[string]plan.Probe, error) {
	var (
		runtime    = a.RuntimeCommand()
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if runtime != capr.RuntimeK3S && IsEtcd(secret) {
		probeNames = append(probeNames, ETCDProbeName)
	}
	if IsControlPlane(secret) {
		probeNames = append(probeNames, KubeAPIServerProbeName)
		probeNames = append(probeNames, KubeControllerManagerProbeName)
		probeNames = append(probeNames, KubeSchedulerProbeName)
	}
	if !(And(IsEtcd, Not(IsControlPlane))(secret) && runtime == capr.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, KubeletProbeName)
	}
	if !And(IsEtcd, Not(IsControlPlane))(secret) && isCalico(a.controlPlane, runtime) && Not(IsWindows)(secret) {
		probeNames = append(probeNames, CalicoProbeName)
	}

	for _, probeName := range probeNames {
		probes[probeName] = AllProbes[probeName]
	}

	dataDir := capr.GetDistroDataDir(a.controlPlane)

	loopbackAddress := capr.GetLoopbackAddress(a.controlPlane)

	config, err := a.renderConfig(secret)
	if err != nil {
		return nil, err
	}

	// render this probe separately because it has a specific format
	if supervisor && (IsEtcd(secret) || IsControlPlane(secret)) {
		supervisorProbe := AllProbes[SupervisorProbeName]
		port := 9345
		if runtime == capr.RuntimeK3S {
			port = 6443
		}
		supervisorProbe.HTTPGetAction.URL = fmt.Sprintf(supervisorProbe.HTTPGetAction.URL, loopbackAddress, port, runtime)
		probes[SupervisorProbeName] = supervisorProbe
	}

	probes = InsertDataDirForProbes(dataDir, probes)

	if IsControlPlane(secret) {
		kcmProbe, err := renderSecureProbe(config[KubeControllerManagerArg], probes[KubeControllerManagerProbeName], dataDir, loopbackAddress, DefaultKubeControllerManagerPort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeControllerManagerProbeName] = kcmProbe

		ksProbe, err := renderSecureProbe(config[KubeSchedulerArg], probes[KubeSchedulerProbeName], dataDir, loopbackAddress, DefaultKubeSchedulerPort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeSchedulerProbeName] = ksProbe
	}

	probes = ReplaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

// isSuitableLeader returns true when the CAPI Machine backing the plan secret exists,
// is not deleting, has a NodeRef, and is Ready.
func (a *CAPRAdapter) isSuitableLeader(s *corev1.Secret) (bool, error) {
	machineName := MachineName(s)
	machine, err := a.clients.CAPI.Machine().Cache().Get(a.controlPlane.Namespace, machineName)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return machine.DeletionTimestamp == nil &&
		machine.Status.NodeRef.IsDefined() &&
		capr.Ready.IsTrue(machine), nil
}

// FindOrElectLeader finds or elects a machine-plan secret to lead the given operation.
// Candidates are collected from the control-plane namespace, filtered by filter, and sorted
// deterministically. An existing leader annotation is reused if the leader is still suitable;
// otherwise a new leader is elected and the annotation written with retry-on-conflict.
// Returns nil, nil when no suitable candidate exists yet.
func (a *CAPRAdapter) FindOrElectLeader(operation string, filter Filter) (*corev1.Secret, error) {
	secrets := a.clients.Core.Secret()
	candidates, err := plan.NewCollector(secrets, a.controlPlane, a.controlPlane.Namespace).
		WithFilter(plan.FilterFunc(filter)).
		WithSorter(plan.DefaultSorter()).
		Collect()
	if err != nil {
		return nil, err
	}

	var (
		marked        *corev1.Secret
		markedCount   int
		markedReady   bool
		initCandidate *corev1.Secret
		fallback      *corev1.Secret
	)
	for _, secret := range candidates {
		if secret.Annotations[OperationLeaderAnnotation] == operation {
			marked = secret
			markedCount++
			if markedCount > 1 {
				return nil, fmt.Errorf("multiple machine-plan secrets marked as operation leader for %s", operation)
			}
		}

		ok, err := a.isSuitableLeader(secret)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if marked != nil && secret.Namespace == marked.Namespace && secret.Name == marked.Name {
			markedReady = true
		}
		if initCandidate == nil && IsInitNode(secret) {
			initCandidate = secret
		}
		if fallback == nil {
			fallback = secret
		}
	}

	if marked != nil {
		if markedReady {
			return marked, nil
		}
		logrus.Warnf("[operations] %s/%s: elected leader %s is no longer suitable, re-electing", a.controlPlane.Namespace, a.controlPlane.Name, marked.Name)
		if err := a.clearLeaderAnnotation(marked, operation); err != nil {
			return nil, err
		}
	}
	if initCandidate != nil {
		return a.markLeader(initCandidate, operation)
	}
	if fallback != nil {
		return a.markLeader(fallback, operation)
	}
	return nil, nil
}

func (a *CAPRAdapter) markLeader(secret *corev1.Secret, operation string) (*corev1.Secret, error) {
	var updated *corev1.Secret
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations[OperationLeaderAnnotation] == operation {
			updated = s
			return nil
		}
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations[OperationLeaderAnnotation] = operation
		updated, err = a.clients.Core.Secret().Update(s)
		return err
	})
	return updated, err
}

func (a *CAPRAdapter) clearLeaderAnnotation(secret *corev1.Secret, operation string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := a.clients.Core.Secret().Get(secret.Namespace, secret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if s.Annotations == nil || s.Annotations[OperationLeaderAnnotation] != operation {
			return nil
		}
		delete(s.Annotations, OperationLeaderAnnotation)
		_, err = a.clients.Core.Secret().Update(s)
		return err
	})
}

// Note: most of this functionally has been copied from the planner 1:1.
// The intention is to split 100% of the planner code to both the plan package and the operations package.
// This will ideally be performed before 2.15 is released; however, migrating piecemeal provides us with the flexibility
// to make changes as desired without putting the existing codebase at risk.

func (a *CAPRAdapter) renderConfig(secret *corev1.Secret) (map[string]any, error) {
	config := map[string]any{}
	if capr.GetRuntime(a.controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
		config["cni"] = CalicoProbeName
	}

	for k, v := range a.controlPlane.Spec.MachineGlobalConfig.Data {
		config[k] = v
	}

	for _, opts := range a.controlPlane.Spec.MachineSelectorConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return nil, err
		}
		if opts.MachineLabelSelector == nil || sel.Matches(labels.Set(secret.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
		}
	}

	if err := filterConfigData(config, a.controlPlane, secret); err != nil {
		return nil, err
	}

	// "data-dir" is explicitly not added to KDM for filtering because it is mapped to a field in the provisioning cluster
	// CRD. While technically possible to add feature gates and update KDM, there is nothing to be gained from such an
	// approach as the "data-dir" implementation will likely never change distro-side.
	if a.controlPlane.Spec.DataDirectories.K8sDistro != "" {
		config["data-dir"] = a.controlPlane.Spec.DataDirectories.K8sDistro
	}

	return config, nil
}

func filterConfigData(config map[string]any, controlPlane *rkev1.RKEControlPlane, secret *corev1.Secret) error {
	var (
		isServer = IsControlPlane(secret) || IsEtcd(secret)
		release  = capr.GetKDMReleaseData(context.TODO(), controlPlane)
	)

	if release == nil {
		return fmt.Errorf("could not find release data")
	}

	for k, v := range config {
		if newV, ok := filterField(isServer, k, v, *release); ok {
			config[k] = newV
		} else {
			delete(config, k)
		}
	}
	return nil
}

func filterField(isServer bool, k string, v any, release model.Release) (any, bool) {
	if v == nil {
		return nil, false
	}

	field, fieldFound := release.AgentArgs[k]
	if !fieldFound && isServer {
		field, fieldFound = release.ServerArgs[k]
	}

	// can't find arg
	if !fieldFound {
		return nil, false
	}

	switch v.(type) {
	case string:
	case bool:
	case []any:
	default:
		// unknown type
		return nil, false
	}

	if field.Type == "boolean" {
		return convert.ToBool(v), true
	}

	return v, true
}

// isCalico returns true if the cni is calico or calico+multus, and returns false otherwise.
func isCalico(controlPlane *rkev1.RKEControlPlane, runtime string) bool {
	// calico is only supported for rke2
	if runtime != capr.RuntimeRKE2 {
		return false
	}

	cni := convert.ToString(controlPlane.Spec.MachineGlobalConfig.Data["cni"])
	return cni == "" ||
		cni == CalicoProbeName ||
		cni == "calico+multus"
}

// getArgValue will search the passed in interface (arg) for a key that matches the searchArg. If a match is found, it
// returns the value of the argument, otherwise it returns an empty string.
func getArgValue(arg any, searchArg string, delim string) string {
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) type of %v is %T", searchArg, delim, arg, arg)
	switch arg := arg.(type) {
	case []any:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) encountered interface slice %v", searchArg, delim, arg)
		return getArgValue(convertInterfaceSliceToStringSlice(arg), searchArg, delim)
	case []string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string array: %v", searchArg, delim, arg)
		for _, v := range arg {
			argKey, argVal := splitArgKeyVal(v, delim)
			if argKey == searchArg {
				return argVal
			}
		}
	case string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string: %v", searchArg, delim, arg)
		argKey, argVal := splitArgKeyVal(arg, delim)
		if argKey == searchArg {
			return argVal
		}
	}
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) did not find searchArg in: %v", searchArg, delim, arg)
	return ""
}

// splitArgKeyVal takes a value and returns a pair (key, value) of the argument, or two empty strings if there was not
// a parsed key/val.
func splitArgKeyVal(val string, delim string) (string, string) {
	if splitSubArg := strings.SplitN(val, delim, 2); len(splitSubArg) == 2 {
		return splitSubArg[0], splitSubArg[1]
	}
	return "", ""
}

// convertInterfaceSliceToStringSlice converts an input interface slice to a string slice by iterating through the
// interface slice and converting each entry to a string using Sprintf.
func convertInterfaceSliceToStringSlice(input []any) []string {
	var stringArr []string
	for _, v := range input {
		stringArr = append(stringArr, fmt.Sprintf("%v", v))
	}
	return stringArr
}

func (a *CAPRAdapter) DistroDataDirectory(_ *corev1.Secret) string {
	return capr.GetDistroDataDir(a.controlPlane)
}

func (a *CAPRAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return capr.GetProvisioningDataDir(&a.controlPlane.Spec.ClusterConfiguration)
}

func (a *CAPRAdapter) KubectlPath(secret *corev1.Secret) string {
	if a.RuntimeCommand() == "k3s" {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DistroDataDirectory(secret), "bin", "kubectl")
}

func (a *CAPRAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.RuntimeCommand() == "k3s" {
		return "/etc/rancher/k3s/k3s.yaml"
	}
	return "/etc/rancher/rke2/rke2.yaml"
}

func (a *CAPRAdapter) PauseCluster(pause bool) error {
	cluster, err := a.clients.CAPI.Cluster().Cache().Get(a.controlPlane.Namespace, a.controlPlane.Name)
	if err != nil {
		return err
	}
	if ptr.Equal(cluster.Spec.Paused, &pause) {
		return nil
	}
	cluster = cluster.DeepCopy()
	cluster.Spec.Paused = &pause
	_, err = a.clients.CAPI.Cluster().Update(cluster)
	return err
}
