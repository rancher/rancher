package operations

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/wrangler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Adapter is an interface for different types of cluster objects.
// the Adapter interface exposes all methods required for constructing a node plan for the supported types.
// The adapter currently supports v2prov, CAPR and imported clusters.
type Adapter interface {
	// WaitForRegister waits for all machine-plan secrets to be created, ensuring the system-agent has checked in for
	// all expected nodes.
	WaitForRegister() (bool, error)

	// RuntimeCommand returns the command used to interact with the distro CLI (RKe2/K3s).
	RuntimeCommand() string

	// ServerUnit returns the systemd unit name for a distro server node.
	ServerUnit() string

	// RenderProbes renders the probes for a given machine-plan secret based on its role.
	// `supervisor` controls whether the supervisor probe should be rendered.
	// Some operations may cause the controlplane to become temporarily unavailable, which will render the etcd plane's
	// supervisor probe to fail.
	RenderProbes(plan *corev1.Secret, supervisor bool) (map[string]plan.Probe, error)
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

	adapter, ok := adapterFactory[gvk.GroupKind().String()]
	if !ok {
		return nil, fmt.Errorf("unsupported cluster type: %s", gvk.GroupKind().String())
	}
	return adapter(clients, ustr)
}

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
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL:        "http://%s:%d/v1-%s/readyz",
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

type AdapterFactory func(*wrangler.CAPIContext, *unstructured.Unstructured) (Adapter, error)

var adapterFactory = map[string]AdapterFactory{}

func RegisterAdapter(gvk schema.GroupVersionKind, factory AdapterFactory) {
	adapterFactory[gvk.String()] = factory
}
