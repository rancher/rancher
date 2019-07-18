package rke

import v3 "github.com/rancher/types/apis/management.cattle.io/v3"

func loadK8sVersionWindowsServiceOptions() map[string]v3.KubernetesServicesOptions {
	return map[string]v3.KubernetesServicesOptions{
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
		"v1.12": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.13": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.14": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
		"v1.15": {
			Kubelet: map[string]string{
				"tls-cipher-suites":        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"feature-gates":            "HyperVContainer=true",
				"cgroups-per-qos":          "false",
				"enforce-node-allocatable": "",
				"resolv-conf":              "",
			},
		},
	}
}

func loadK8sVersionWindowsSystemimages() map[string]v3.WindowsSystemImages {
	return map[string]v3.WindowsSystemImages{
		"v1.8.10-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.10-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.8.11-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.11-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.8.11-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.8.11-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.9.7-rancher2-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.9.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.0-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.0-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.1-rancher1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.1-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.3-rancher2-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.3-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.5-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.11-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.11-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.10.12-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.10.12-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.1-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.2-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.2-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.2-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.2-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.3-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.3-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.6-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.6-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.8-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.8-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.9-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.9-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.11.9-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.11.9-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.0-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.0-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.1-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.3-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.3-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.4-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.4-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.7-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},

		"v1.12.5-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.6-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.6-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.7-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.7-rancher1-3": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.12.9-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.12.9-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.1-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.4-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.4-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.4-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.4-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.5-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.5-rancher1-2": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.5-rancher1-3": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.5-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.7-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.7-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.13.8-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.13.8-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.14.1-rancher1-1": {
			NginxProxy:         m("rancher/nginx-proxy:v0.0.1-nanoserver-1803"),
			KubernetesBinaries: m("rancher/hyperkube:v1.14.1-nanoserver-1803"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.0.1-nanoserver-1803"),
			CalicoCNIBinaries:  m("rancher/calico-cni:v0.0.1-nanoserver-1803"),
			CanalCNIBinaries:   m("rancher/canal-cni:v0.0.1-nanoserver-1803"),
			KubeletPause:       m("rancher/kubelet-pause:v0.0.1-nanoserver-1803"),
		},
		"v1.14.1-rancher1-2": {
			// NginxProxy image is replaced by host running nginx, fixed rancher#16074
			KubernetesBinaries: m("rancher/hyperkube:v1.14.1-rancher2"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.3.0-rancher4"),
			KubeletPause:       m("rancher/kubelet-pause:v0.1.2"),
		},
		"v1.14.3-rancher1-1": {
			// NginxProxy image is replaced by host running nginx, fixed rancher#16074
			KubernetesBinaries: m("rancher/hyperkube:v1.14.3-rancher1"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.3.0-rancher4"),
			KubeletPause:       m("rancher/kubelet-pause:v0.1.2"),
		},
		"v1.14.4-rancher1-1": {
			// NginxProxy image is replaced by host running nginx, fixed rancher#16074
			KubernetesBinaries: m("rancher/hyperkube:v1.14.4-rancher1"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.3.0-rancher4"),
			KubeletPause:       m("rancher/kubelet-pause:v0.1.2"),
		},
		"v1.15.0-rancher1-1": {
			// NginxProxy image is replaced by host running nginx, fixed rancher#16074
			KubernetesBinaries: m("rancher/hyperkube:v1.15.0-rancher1"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.3.0-rancher4"),
			KubeletPause:       m("rancher/kubelet-pause:v0.1.2"),
		},
		"v1.15.0-rancher1-2": {
			// NginxProxy image is replaced by host running nginx, fixed rancher#16074
			KubernetesBinaries: m("rancher/hyperkube:v1.15.0-rancher1"),
			FlannelCNIBinaries: m("rancher/flannel-cni:v0.3.0-rancher4"),
			KubeletPause:       m("rancher/kubelet-pause:v0.1.2"),
		},
	}
}
