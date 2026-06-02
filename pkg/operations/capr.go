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
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

// Note: most of this functionally has been copied from the planner 1:1.
// The intention is to split 100% of the planner code to both the planapi package and the operations package.
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

// ElectLeader picks a leader from the CAPR cluster's machine-plan secrets. Eligibility excludes
// secrets whose CAPI machine has a non-nil DeletionTimestamp (or whose machine is gone). Init
// detection uses capr.InitNodeLabel, which the planner sets during init-node election.
func (a *CAPRAdapter) ElectLeader(role LeaderRole, namespace string) (*corev1.Secret, error) {
	secrets, err := a.clients.Core.Secret().Cache().List(namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.controlPlane.Name,
	}))
	if err != nil {
		return nil, err
	}

	candidates := make([]LeaderCandidate, 0, len(secrets))
	for _, secret := range secrets {
		c := LeaderCandidate{Secret: secret, Eligible: true}
		c.Init = secret.Labels[capr.InitNodeLabel] == "true"

		machineName := secret.Labels[capr.MachineNameLabel]
		machineNamespace := secret.Labels[capr.MachineNamespaceLabel]
		if machineName != "" && machineNamespace != "" {
			machine, err := a.clients.CAPI.Machine().Cache().Get(machineNamespace, machineName)
			switch {
			case apierrors.IsNotFound(err):
				// The plan secret outlives its machine briefly during deletion; treat as ineligible.
				c.Eligible = false
			case err != nil:
				return nil, err
			case machine.DeletionTimestamp != nil:
				c.Eligible = false
			}
		}
		candidates = append(candidates, c)
	}
	return electLeader(role, candidates), nil
}
