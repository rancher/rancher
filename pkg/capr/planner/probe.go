package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/wrangler/pkg/data/convert"
)

var allProbes = map[string]plan.Probe{
	"calico": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL: "http://127.0.0.1:9099/liveness",
		},
	},
	"etcd": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL: "http://127.0.0.1:2381/health",
		},
	},
	"kube-apiserver": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL:        "https://127.0.0.1:6443/readyz",
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
			URL: "https://127.0.0.1:%s/healthz",
		},
	},
	"kube-controller-manager": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL: "https://127.0.0.1:%s/healthz",
		},
	},
	"kubelet": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL: "http://127.0.0.1:10248/healthz",
		},
	},
}

func isCalico(controlPlane *rkev1.RKEControlPlane, runtime string) bool {
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
func renderSecureProbe(arg interface{}, rawProbe plan.Probe, runtime string, defaultSecurePort string, defaultCertDir string, defaultCert string) (plan.Probe, error) {
	securePort := getArgValue(arg, SecurePortArgument, "=")
	if securePort == "" {
		// If the user set a custom --secure-port, set --secure-port to an empty string so we don't override
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
	return replaceCACertAndPortForProbes(rawProbe, TLSCert, securePort)
}

// generateProbes generates probes for the machine (based on type of machine) to the nodePlan and returns the probes and an error
// if one occurred.
func (p *Planner) generateProbes(controlPlane *rkev1.RKEControlPlane, entry *planEntry, config map[string]interface{}) (map[string]plan.Probe, error) {
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

	if isControlPlane(entry) {
		kcmProbe, err := renderSecureProbe(config[KubeControllerManagerArg], probes["kube-controller-manager"], runtime, DefaultKubeControllerManagerDefaultSecurePort, DefaultKubeControllerManagerCertDir, DefaultKubeControllerManagerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-controller-manager"] = kcmProbe

		ksProbe, err := renderSecureProbe(config[KubeSchedulerArg], probes["kube-scheduler"], runtime, DefaultKubeSchedulerDefaultSecurePort, DefaultKubeSchedulerCertDir, DefaultKubeSchedulerCert)
		if err != nil {
			return probes, err
		}
		probes["kube-scheduler"] = ksProbe
	}
	return probes, nil
}

// replaceCACertAndPortForProbes adds/replaces the CACert and URL with rendered values based on the values provided.
func replaceCACertAndPortForProbes(probe plan.Probe, cacert, port string) (plan.Probe, error) {
	if cacert == "" || port == "" {
		return plan.Probe{}, fmt.Errorf("CA cert (%s) or port (%s) not defined properly", cacert, port)
	}
	probe.HTTPGetAction.CACert = cacert
	probe.HTTPGetAction.URL = fmt.Sprintf(probe.HTTPGetAction.URL, port)
	return probe, nil
}

func replaceRuntimeForProbes(probes map[string]plan.Probe, runtime string) map[string]plan.Probe {
	result := map[string]plan.Probe{}
	for k, v := range probes {
		v.HTTPGetAction.CACert = replaceRuntime(v.HTTPGetAction.CACert, runtime)
		v.HTTPGetAction.ClientCert = replaceRuntime(v.HTTPGetAction.ClientCert, runtime)
		v.HTTPGetAction.ClientKey = replaceRuntime(v.HTTPGetAction.ClientKey, runtime)
		result[k] = v
	}
	return result
}

func replaceRuntime(str string, runtime string) string {
	if !strings.Contains(str, "%s") {
		return str
	}
	return fmt.Sprintf(str, runtime)
}
