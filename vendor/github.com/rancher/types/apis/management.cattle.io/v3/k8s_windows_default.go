package v3

var (
	// K8sVersionWindowsSystemImages is dynamically populated on initWindows() with the latest versions
	K8sVersionWindowsSystemImages map[string]WindowsSystemImages

	// K8sVersionWindowsServiceOptions - service options per k8s version
	K8sVersionWindowsServiceOptions = map[string]KubernetesServicesOptions{
		"v1.8": {
			Kubelet: map[string]string{
				"feature-gates":            "MountPropagation=false",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.9": {
			Kubelet: map[string]string{
				"feature-gates":            "MountPropagation=false",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.10": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "MountPropagation=false,HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.11": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "MountPropagation=false,HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
	}

	// AllK8sWindowsVersions - images map for 2.0
	allK8sWindowsVersions = map[string]WindowsSystemImages{
		"v1.8.10-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.10-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.8.11-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.11-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.8.11-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.11-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher2-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.0-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.0-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.1-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.1-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.3-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.3-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.5-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.1-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.2-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.2-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.2-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.2-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
	}
)

func initWindows() {
	badVersions := map[string]bool{
		"v1.8.11-rancher2-1": true,
		"v1.8.11-rancher1":   true,
		"v1.8.10-rancher1-1": true,
	}

	if K8sVersionWindowsSystemImages != nil {
		panic("Do not initialize or add values to K8sVersionWindowsSystemImages")
	}

	K8sVersionWindowsSystemImages = map[string]WindowsSystemImages{}

	for version := range K8sVersionToRKESystemImages {
		if badVersions[version] {
			continue
		}

		images, ok := allK8sWindowsVersions[version]
		if !ok {
			panic("K8s version " + " is not found in AllK8sWindowsVersions map")
		}

		K8sVersionWindowsSystemImages[version] = images
	}

	if _, ok := K8sVersionWindowsSystemImages[DefaultK8s]; !ok {
		panic("Default K8s version " + DefaultK8s + " is not found in k8sVersionsCurrent list")
	}
}
