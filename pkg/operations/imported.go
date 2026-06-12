package operations

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	RegisterAdapter(mgmtv3.SchemeGroupVersion.WithKind("Cluster"), func(clients *wrangler.CAPIContext, unstructured *unstructured.Unstructured) (Adapter, error) {
		var cluster *mgmtv3.Cluster
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, &cluster)
		if err != nil {
			return nil, err
		}

		return &ImportedAdapter{
			cluster: cluster,
			clients: clients,
		}, nil
	})
}

type ImportedAdapter struct {
	cluster *mgmtv3.Cluster
	clients *wrangler.CAPIContext
}

// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
// all expected nodes.
// All machine-plans secrets are listed and compared to the count of mgmtv3.Node objects for the cluster.
func (a *ImportedAdapter) WaitForRegister() (bool, error) {
	secretList, err := a.clients.Core.Secret().List(a.cluster.Name, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, a.cluster.Name),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return false, err
	}

	secrets := secretList.Items

	machines, err := a.clients.Mgmt.Node().Cache().List(a.cluster.Name, labels.Everything())
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

// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
func (a *ImportedAdapter) RuntimeCommand() string {
	if a.cluster.Status.Provider == "rke2" {
		return "rke2"
	}
	return "k3s"
}

// ServerUnit returns the systemd unit name for a distro server node.
func (a *ImportedAdapter) ServerUnit() string {
	if a.cluster.Status.Provider == "rke2" {
		return "rke2-server"
	}
	return "k3s"
}

func (a *ImportedAdapter) DistroDataDirectory(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "rke2" {
		return "/var/lib/rancher/rke2"
	}
	return "/var/lib/rancher/k3s"
}

func (a *ImportedAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	// Imported clusters do not expose the provisioning data directory; fall back to the default.
	return "/var/lib/rancher/capr"
}

// RenderProbes renders the probes for a given machine-plan secret based on its role.
// Currently custom data directories, probes, and using ipv4 as the primary ip family are not supported.
func (a *ImportedAdapter) RenderProbes(secret *corev1.Secret, supervisor bool) (map[string]plan.Probe, error) {
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

	for _, probeName := range probeNames {
		probes[probeName] = AllProbes[probeName]
	}

	dataDir := "/var/lib/rancher/rke2"
	if runtime == capr.RuntimeK3S {
		dataDir = "/var/lib/rancher/k3s"
	}

	// only support ipv4, need to implement per-node extraction mechanism
	loopbackAddress := "127.0.0.1"

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
		kcmProbe, err := renderSecureProbe("", probes[KubeControllerManagerProbeName], dataDir, loopbackAddress, DefaultKubeControllerManagerPort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeControllerManagerProbeName] = kcmProbe

		ksProbe, err := renderSecureProbe("", probes[KubeSchedulerProbeName], dataDir, loopbackAddress, DefaultKubeSchedulerPort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes[KubeSchedulerProbeName] = ksProbe
	}

	probes = ReplaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

func (a *ImportedAdapter) KubectlPath(secret *corev1.Secret) string {
	if a.cluster.Status.Provider == "k3s" {
		return "/usr/local/bin/kubectl"
	}
	return path.Join(a.DistroDataDirectory(secret), "bin", "kubectl")
}

func (a *ImportedAdapter) KubeconfigPath(_ *corev1.Secret) string {
	if a.cluster.Status.Provider == "k3s" {
		return "/etc/rancher/k3s/k3s.yaml"
	}
	return "/etc/rancher/rke2/rke2.yaml"
}

// ElectLeader picks a leader from the imported cluster's machine-plan secrets. Eligibility excludes
// secrets whose backing v3.Node has a non-nil DeletionTimestamp (or whose Node is gone). Init
// detection parses the distro's node-args annotation off the v3.Node status — k3s init nodes pass
// --cluster-init; rke2 servers are init when they have no --server flag pointing elsewhere.
func (a *ImportedAdapter) ElectLeader(role LeaderRole, namespace string) (*corev1.Secret, error) {
	secrets, err := a.clients.Core.Secret().Cache().List(namespace, labels.SelectorFromSet(labels.Set{
		capr.ClusterNameLabel: a.cluster.Name,
	}))
	if err != nil {
		return nil, err
	}

	runtime := a.RuntimeCommand()

	candidates := make([]LeaderCandidate, 0, len(secrets))
	for _, secret := range secrets {
		c := LeaderCandidate{Secret: secret, Eligible: true}

		nodeName := secret.Labels[capr.MachineNameLabel]
		if nodeName != "" {
			node, err := a.clients.Mgmt.Node().Cache().Get(a.cluster.Name, nodeName)
			switch {
			case apierrors.IsNotFound(err):
				c.Eligible = false
			case err != nil:
				return nil, err
			case node.DeletionTimestamp != nil:
				c.Eligible = false
			default:
				c.Init = isImportedInitNode(node, runtime)
			}
		}
		candidates = append(candidates, c)
	}
	return electLeader(role, candidates), nil
}

// isImportedInitNode infers whether the downstream Node was started as the cluster's init server,
// based on the distro's node-args annotation kept on the v3.Node status by the nodesyncer.
//
//   - k3s: the init node passes --cluster-init.
//   - rke2: there is no explicit init flag; the first server has no --server <url>, so absence of
//     --server in a server-mode args list signals init.
//
// Returns false if the annotation is missing or unparseable; downstream tiers (etcd-only, etcd+cp)
// still apply.
func isImportedInitNode(node *mgmtv3.Node, runtime string) bool {
	if node == nil || node.Status.NodeAnnotations == nil {
		return false
	}
	raw := node.Status.NodeAnnotations[runtime+".io/node-args"]
	if raw == "" {
		return false
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return false
	}

	isServer := false
	for _, arg := range args {
		if arg == "server" {
			isServer = true
			break
		}
	}
	if !isServer {
		return false
	}

	switch runtime {
	case capr.RuntimeK3S:
		for _, arg := range args {
			if arg == "--cluster-init" || strings.HasPrefix(arg, "--cluster-init=") {
				return true
			}
		}
		return false
	case capr.RuntimeRKE2:
		for _, arg := range args {
			if arg == "--server" || strings.HasPrefix(arg, "--server=") {
				return false
			}
		}
		return true
	}
	return false
}

func (a *ImportedAdapter) PauseCluster(_ bool) error {
	return nil
}
