package planner

import (
	"fmt"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/wrangler/pkg/data/convert"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
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
			URL: "http://127.0.0.1:10251/healthz",
		},
	},
	"kube-controller-manager": {
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		HTTPGetAction: plan.HTTPGetAction{
			URL: "http://127.0.0.1:10252/healthz",
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
	if runtime != rancherruntime.RuntimeRKE2 {
		return false
	}
	cni := convert.ToString(controlPlane.Spec.MachineGlobalConfig.Data["cni"])
	return cni == "" ||
		cni == "calico" ||
		cni == "calico+multus"
}

func (p *Planner) addProbes(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	var (
		runtime    = rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)
		probeNames []string
	)

	nodePlan.Probes = map[string]plan.Probe{}

	if runtime != rancherruntime.RuntimeK3S && isEtcd(machine) {
		probeNames = append(probeNames, "etcd")
	}
	if isControlPlane(machine) {
		probeNames = append(probeNames, "kube-apiserver")
		probeNames = append(probeNames, "kube-controller-manager")
		probeNames = append(probeNames, "kube-scheduler")
	}
	if !(IsOnlyEtcd(machine) && runtime == rancherruntime.RuntimeK3S) {
		// k3s doesn't run the kubelet on etcd only nodes
		probeNames = append(probeNames, "kubelet")
	}
	if !IsOnlyEtcd(machine) && isCalico(controlPlane, runtime) {
		probeNames = append(probeNames, "calico")
	}

	for _, probeName := range probeNames {
		nodePlan.Probes[probeName] = allProbes[probeName]
	}

	nodePlan.Probes = replaceRuntimeForProbes(nodePlan.Probes, runtime)
	return nodePlan, nil
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
