package rke

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func loadK8sVersionWindowsServiceOptions() map[string]v3.KubernetesServicesOptions {
	// since 1.14, windows has been supported
	return map[string]v3.KubernetesServicesOptions{
		"v1.15": {
			Kubelet:   getWindowsKubeletOptions115(),
			Kubeproxy: getWindowsKubeProxyOptions(),
		},
		"v1.14": {
			Kubelet:   getWindowsKubeletOptions(),
			Kubeproxy: getWindowsKubeProxyOptions(),
		},
	}
}

func getWindowsKubeletOptions() map[string]string {
	kubeletOptions := getKubeletOptions()

	// doesn't support cgroups
	kubeletOptions["cgroups-per-qos"] = "false"
	kubeletOptions["enforce-node-allocatable"] = "''"
	// doesn't support dns
	kubeletOptions["resolv-conf"] = "''"
	// add prefix path for directory options
	kubeletOptions["cni-bin-dir"] = "[PREFIX_PATH]/opt/cni/bin"
	kubeletOptions["cni-conf-dir"] = "[PREFIX_PATH]/etc/cni/net.d"
	kubeletOptions["cert-dir"] = "[PREFIX_PATH]/var/lib/kubelet/pki"
	kubeletOptions["volume-plugin-dir"] = "[PREFIX_PATH]/var/lib/kubelet/volumeplugins"
	// add reservation for kubernetes components
	kubeletOptions["kube-reserved"] = "cpu=500m,memory=500Mi,ephemeral-storage=1Gi"
	// add reservation for system
	kubeletOptions["system-reserved"] = "cpu=1000m,memory=2Gi,ephemeral-storage=2Gi"
	// increase image pulling deadline
	kubeletOptions["image-pull-progress-deadline"] = "30m"
	// enable some windows features
	kubeletOptions["feature-gates"] = "HyperVContainer=true,WindowsGMSA=true"

	return kubeletOptions
}

func getWindowsKubeletOptions115() map[string]string {
	kubeletOptions := getWindowsKubeletOptions()

	// doesn't support `allow-privileged`
	delete(kubeletOptions, "allow-privileged")

	return kubeletOptions
}

func getWindowsKubeProxyOptions() map[string]string {
	kubeProxyOptions := getKubeProxyOptions()

	// use kernelspace proxy mode
	kubeProxyOptions["proxy-mode"] = "kernelspace"
	// enable Windows Overlay support
	kubeProxyOptions["feature-gates"] = "WinOverlay=true"
	// disable Windows DSR support explicitly
	kubeProxyOptions["enable-dsr"] = "false"

	return kubeProxyOptions
}
