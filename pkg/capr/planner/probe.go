package planner

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/pkg/data/convert"
)

var (
	allProbes = map[string]plan.Probe{
		"calico": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:9099/liveness",
			},
		},
		"etcd": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:2381/health",
			},
		},
		"kube-apiserver": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL:        "https://%s:6443/readyz",
				CACert:     "/var/lib/rancher/%s/server/tls/server-ca.crt",
				ClientCert: "/var/lib/rancher/%s/server/tls/client-kube-apiserver.crt",
				ClientKey:  "/var/lib/rancher/%s/server/tls/client-kube-apiserver.key",
			},
		},
		"kube-scheduler": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		"kube-controller-manager": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "https://%s:%s/healthz",
			},
		},
		"kubelet": {
			InitialDelaySeconds: 1,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    2,
			HTTPGetAction: plan.HTTPGetAction{
				URL: "http://%s:10248/healthz",
			},
		},
	}
	errEmptyCACert  = errors.New("cacert cannot be empty")
	errEmptyPort    = errors.New("port cannot be empty")
	errEmptyAddress = errors.New("address cannot be empty")
)

// isCalico returns true if the cni is calico or calico+multus, and returns false otherwise.
func isCalico(controlPlane *rkev1.RKEControlPlane, runtime string) bool {
	// calico is only supported for rke2
	if runtime != capr.RuntimeRKE2 {
		return false
	}

	cni := convert.ToString(controlPlane.Spec.MachineGlobalConfig.Data["cni"])
	return cni == "" ||
		cni == "calico" ||
		cni == "calico+multus"
}

// renderSecureProbe takes the existing argument value and renders a secure probe using the argument values and an error
// if one occurred.
func renderSecureProbe(arg any, rawProbe plan.Probe, runtime string, loopbackAddress, defaultSecurePort string, defaultCertDir string, defaultCert string) (plan.Probe, error) {
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
			// If --cert-dir was not set, we use defaultCertDir value that was passed in, but must render it to replace
			// the %s for runtime
			certDir = fmt.Sprintf(defaultCertDir, runtime)
		}
		// Our goal here is to generate the tlsCert. If we get to this point, we know we will be using the defaultCert
		TLSCert = certDir + "/" + defaultCert
	}
	return replaceCACertAndPortForProbes(rawProbe, TLSCert, loopbackAddress, securePort)
}

// generateProbes generates probes for the machine (based on type of machine) to the nodePlan and returns the probes and
// an error if one occurred.
func (p *Planner) generateProbes(controlPlane *rkev1.RKEControlPlane, entry *planEntry, config map[string]any) (map[string]plan.Probe, error) {
	var (
		runtime    = capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
		probeNames []string
		probes     = map[string]plan.Probe{}
	)

	if runtime != capr.RuntimeK3S && isEtcd(entry) {
		probeNames = append(probeNames, "etcd")
	}
	if isControlPlane(entry) {
		probeNames = append(probeNames, "kube-apiserver")
		probeNames = append(probeNames, "kube-controller-manager")
		probeNames = append(probeNames, "kube-scheduler")
	}
	if !(IsOnlyEtcd(entry) && runtime == capr.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, "kubelet")
	}
	if !IsOnlyEtcd(entry) && isCalico(controlPlane, runtime) && roleNot(windows)(entry) {
		probeNames = append(probeNames, "calico")
	}

	for _, probeName := range probeNames {
		probes[probeName] = allProbes[probeName]
	}

	probes = replaceRuntimeForProbes(probes, runtime)

	loopbackAddress := capr.GetLoopbackAddress(controlPlane)

	if isControlPlane(entry) {
		kcmProbe, err := renderSecureProbe(config[KubeControllerManagerArg], probes["kube-controller-manager"], runtime, loopbackAddress, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-controller-manager"] = kcmProbe

		ksProbe, err := renderSecureProbe(config[KubeSchedulerArg], probes["kube-scheduler"], runtime, loopbackAddress, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-scheduler"] = ksProbe
	}

	probes = replaceURLForProbes(probes, loopbackAddress)

	return probes, nil
}

// replaceCACertAndPortForProbes adds/replaces the CACert and URL with rendered values based on the values provided.
func replaceCACertAndPortForProbes(probe plan.Probe, cacert, host, port string) (plan.Probe, error) {
	if cacert == "" {
		return plan.Probe{}, errEmptyCACert
	}
	if port == "" {
		return plan.Probe{}, errEmptyPort
	}
	if host == "" {
		return plan.Probe{}, errEmptyAddress
	}
	probe.HTTPGetAction.CACert = cacert
	probe.HTTPGetAction.URL = fmt.Sprintf(probe.HTTPGetAction.URL, host, port)
	return probe, nil
}

// replaceRuntimeForProbes will insert the k8s runtime for all probes based on the runtime provider.
func replaceRuntimeForProbes(probes map[string]plan.Probe, runtime string) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.CACert = replaceIfFormatSpecifier(v.HTTPGetAction.CACert, runtime)
		v.HTTPGetAction.ClientCert = replaceIfFormatSpecifier(v.HTTPGetAction.ClientCert, runtime)
		v.HTTPGetAction.ClientKey = replaceIfFormatSpecifier(v.HTTPGetAction.ClientKey, runtime)
		result[k] = v
	}
	return result
}

// replaceURLForProbes will insert the loopback host for all probes based on stack preference.
func replaceURLForProbes(probes map[string]plan.Probe, loopbackAddress string) map[string]plan.Probe {
	result := make(map[string]plan.Probe, len(probes))
	for k, v := range probes {
		v.HTTPGetAction.URL = replaceIfFormatSpecifier(v.HTTPGetAction.URL, loopbackAddress)
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
