package operations

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	SecurePortArgument  = "secure-port"
	CertDirArgument     = "cert-dir"
	TLSCertFileArgument = "tls-cert-file"

	ETCDProbeName                  = "etcd"
	KubeAPIServerProbeName         = "kube-apiserver"
	KubeControllerManagerProbeName = "kube-controller-manager"
	KubeSchedulerProbeName         = "kube-scheduler"
	KubeletProbeName               = "kubelet"

	SupervisorProbeName = "supervisor"

	CalicoProbeName = "calico"

	KubeControllerManagerArg            = "kube-controller-manager-arg"
	DefaultKubeControllerManagerCertDir = "server/tls/kube-controller-manager"
	DefaultKubeControllerManagerCert    = "kube-controller-manager.crt"
	DefaultKubeControllerManagerPort    = "10257"

	KubeSchedulerArg            = "kube-scheduler-arg"
	DefaultKubeSchedulerCertDir = "server/tls/kube-scheduler"
	DefaultKubeSchedulerCert    = "kube-scheduler.crt"
	DefaultKubeSchedulerPort    = "10259"

	OperationLeaderAnnotation = "rke.cattle.io/operation-leader"
)

var (
	AllProbes = map[string]plan.Probe{
		CalicoProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:9099/liveness",
			},
		},
		ETCDProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:2381/health",
			},
		},
		KubeAPIServerProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL:        "https://%s:6443/readyz",
				CACert:     "%s/server/tls/server-ca.crt",
				ClientCert: "%s/server/tls/client-kube-apiserver.crt",
				ClientKey:  "%s/server/tls/client-kube-apiserver.key",
			},
		},
		KubeSchedulerProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		KubeControllerManagerProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		KubeletProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:10248/healthz",
			},
		},
		SupervisorProbeName: {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      30,
			SuccessThreshold:    1,
			FailureThreshold:    30,
			HTTPGetAction: plan.HTTPGetAction{
				URL:        "https://%s:%d/v1-%s/readyz",
				CACert:     "%s/agent/server-ca.crt",
				ClientCert: "%s/agent/client-kubelet.crt",
				ClientKey:  "%s/agent/client-kubelet.key",
			},
		},
	}
	ErrEmptyCACert  = errors.New("cacert cannot be empty")
	ErrEmptyPort    = errors.New("port cannot be empty")
	ErrEmptyAddress = errors.New("address cannot be empty")
)

// Adapter is an interface for different types of cluster objects.
// the Adapter interface exposes all methods required for constructing a node plan for the supported types.
// The adapter currently supports v2prov, CAPR and imported clusters.
type Adapter interface {
	// BeaconRef returns the (namespace, name) where the cluster's beacon lives — and,
	// by convention, where its machine-plan secrets and etcd-snapshot CRs also live. Operation
	// controllers use this to resolve cluster-scoped state regardless of what ClusterRef the
	// user (or UI) supplied on the operation:
	//   - v2prov (CAPR): (controlPlane.Namespace, controlPlane.Name) — the provv1.Cluster's
	//     namespace (typically fleet-default) alongside the CAPI cluster of the same name.
	//   - CAPRKE2 (turtles-imported): (cluster.Namespace, cluster.Name) — the CAPI cluster's
	//     own namespace.
	//   - Imported RKE2/K3s: (cluster.Name, cluster.Name) — the mgmt v3 Cluster is
	//     cluster-scoped, so its name doubles as the namespace convention.
	BeaconRef() (namespace, name string)

	// EtcdSnapshotNamespace returns the namespace where the mgmt-side rkev1.ETCDSnapshot CRs
	// for this cluster live. For v2prov and imported this equals BeaconRef's namespace; for
	// CAPRKE2 (turtles-imported) it is the mgmt v3 Cluster's name — plan secrets and the beacon
	// stay in the CAPI Cluster's namespace, but snapshots live alongside the mgmt v3 Nodes.
	EtcdSnapshotNamespace() string

	// ClusterObject returns the cluster object for this adapter.
	// This is not necessarily the object this operation was created for - for example, for CAPRKE2 clusters, the UI
	// will create an operation for the management cluster object, but the true object is the CAPI cluster.
	ClusterObject() (*unstructured.Unstructured, error)

	// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
	// all expected nodes.
	WaitForRegister() (bool, error)

	// PauseCluster edits the related cluster object to indicate it should not be reconciled.
	// This is intended to prevent other controllers from manipulating the cluster during sensitive operations.
	PauseCluster(pause bool) error

	// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
	RuntimeCommand() string

	// ServerUnit returns the systemd unit name for a distro server node.
	ServerUnit() string

	// DistroDataDirectory returns the path to the RKE2/K3s data-dir on the host machine.
	DistroDataDirectory(secret *corev1.Secret) string

	// ProvisioningDataDirectory returns the path to the data directory used for operations.
	// Scripts created for commands are typically stored here.
	ProvisioningDataDirectory(secret *corev1.Secret) string

	ConfigFile(secret *corev1.Secret) string

	ConfigDirectory(secret *corev1.Secret) string

	// RenderProbes renders the probes for a given machine-plan secret based on its role.
	// `supervisor` controls whether the supervisor probe should be rendered.
	// Some operations may cause the controlplane to become temporarily unavailable, which will render the etcd plane's
	// supervisor probe to fail.
	RenderProbes(plan *corev1.Secret, supervisor bool) (map[string]plan.Probe, error)

	// KubectlPath returns the path to the kubectl binary on the host relative to the machine-plan secret.
	KubectlPath(secret *corev1.Secret) string

	// KubeconfigPath returns the path to the kubeconfig file on the host relative to the machine-plan secret.
	KubeconfigPath(secret *corev1.Secret) string

	// FindOrElectLeader finds an existing elected leader for the given operation or elects one
	// from candidates passing filter. The elected leader is marked with an annotation on the
	// machine-plan secret so the same node is reused across reconciles. Returns nil, nil when
	// no suitable candidate exists yet.
	FindOrElectLeader(operation string, filter Filter) (*corev1.Secret, error)

	// GetServerURL returns the server url required to join nodes to this host.
	// The URL is of the form `https://<InternalIP>:<supervisor port>`
	GetServerURL(secret *corev1.Secret) string

	GetSupervisorPort(secret *corev1.Secret) string

	LoopbackAddress(secret *corev1.Secret) string

	ToS3ArgsEnvAndFiles(secret *corev1.Secret) ([]string, []string, []plan.File)
}

// NewAdapter returns an Adapter for the given cluster object.
// For Provisioning clusters the controlPlane object is extracted and then a CAPR Adapter is used to prevent duplication.
// The wrangler.CAPIContext is used in order to allow the adapter to access specific typed caches for ease of use.
func NewAdapter(clients *wrangler.CAPIContext, ustr *unstructured.Unstructured) (Adapter, error) {
	if ustr == nil {
		return nil, errors.New("nil unstructured")
	}
	// controlplane and provisioning cluster always have the same name
	gvk := schema.FromAPIVersionAndKind(ustr.GetAPIVersion(), ustr.GetKind())

	adapter, ok := adapterFactory[gvk.String()]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", gvk.String())
	}
	return adapter(clients, ustr)
}

type AdapterFactory func(*wrangler.CAPIContext, *unstructured.Unstructured) (Adapter, error)

var adapterFactory = map[string]AdapterFactory{}

func RegisterAdapter(gvk schema.GroupVersionKind, factory AdapterFactory) {
	adapterFactory[gvk.String()] = factory
}

// ReplaceCACertAndPortForProbes adds/replaces the CACert and URL with rendered values based on the values provided.
func ReplaceCACertAndPortForProbes(probe plan.Probe, cacert, host, port string) (plan.Probe, error) {
	if cacert == "" {
		return plan.Probe{}, ErrEmptyCACert
	}
	if port == "" {
		return plan.Probe{}, ErrEmptyPort
	}
	if host == "" {
		return plan.Probe{}, ErrEmptyAddress
	}
	probe.HTTPGetAction.CACert = cacert
	probe.HTTPGetAction.URL = fmt.Sprintf(probe.HTTPGetAction.URL, host, port)
	return probe, nil
}

// ReplaceURLForProbes will insert the loopback host for all probes based on stack preference.
func ReplaceURLForProbes(probes map[string]plan.Probe, loopbackAddress string) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.URL = replaceIfFormatSpecifier(v.HTTPGetAction.URL, loopbackAddress)
		result[k] = v
	}
	return result
}

// InsertDataDirForProbes will insert the data-dir for all probes based on the controlplane object.
func InsertDataDirForProbes(dataDir string, probes map[string]plan.Probe) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.CACert = replaceIfFormatSpecifier(v.HTTPGetAction.CACert, dataDir)
		v.HTTPGetAction.ClientCert = replaceIfFormatSpecifier(v.HTTPGetAction.ClientCert, dataDir)
		v.HTTPGetAction.ClientKey = replaceIfFormatSpecifier(v.HTTPGetAction.ClientKey, dataDir)
		result[k] = v
	}
	return result
}

// replaceIfFormatSpecifier will insert the runtime of the k8s engine if the string str has a string format specifier.
func replaceIfFormatSpecifier(str string, runtime string) string {
	if !strings.Contains(str, "%s") {
		return str
	}
	return fmt.Sprintf(str, runtime)
}

// renderSecureProbe takes the existing argument value and renders a secure probe using the argument values and an error
// if one occurred.
func renderSecureProbe(arg any, probe plan.Probe, dataDir string, loopbackAddress, defaultSecurePort string, defaultCertDir string, defaultCert string) (plan.Probe, error) {
	securePort := getArgValue(arg, SecurePortArgument, "=")
	if securePort == "" {
		// If the user set a custom --secure-port, set --secure-port to an empty string, so we don't override
		// their custom value
		securePort = defaultSecurePort
	}
	TLSCert := getArgValue(arg, TLSCertFileArgument, "=")
	if TLSCert == "" {
		// If the --tls-cert-file Argument was not set in the config for this component, we can look to see if
		// the --cert-dir was set. --tls-cert-file (if set) will take precedence over --cert-dir
		certDir := getArgValue(arg, CertDirArgument, "=")
		if certDir == "" {
			// If --cert-dir was not set, we use defaultCertDir value that was passed in, but must prefix the data-dir
			certDir = path.Join(dataDir, defaultCertDir)
		}
		// Our goal here is to generate the tlsCert. If we get to this point, we know we will be using the defaultCert
		TLSCert = certDir + "/" + defaultCert
	}
	return ReplaceCACertAndPortForProbes(probe, TLSCert, loopbackAddress, securePort)
}

func MachineName(secret *corev1.Secret) string {
	if secret == nil || secret.Labels == nil {
		return ""
	}
	return secret.Labels[planv1alpha1.MachineLifecycleNameLabel]
}
