package v3

import "github.com/rancher/types/image"

const (
	DefaultK8s = "v1.10.1-rancher1"
)

var (
	m          = image.Mirror
	ToolsImage = m("rancher/rke-tools:v0.1.3")

	// K8sVersionToRKESystemImages - images map for 2.0
	K8sVersionToRKESystemImages = map[string]RKESystemImages{
		"v1.8.10-rancher1-1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.0.17"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.8.10"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.0"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
		"v1.8.11-rancher1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.0.17"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.8.11"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.5"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.5"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.0"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
		"v1.9.7-rancher1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.1.12"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.9.7"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.7"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.7"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.7"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.0"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
		"v1.9.5-rancher1-1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.1.12"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.9.5"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.7"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.7"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.7"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.0"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
		"v1.10.0-rancher1-1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.1.12"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.10.0"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.8"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.8"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.8"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.1"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
		"v1.10.1-rancher1": {
			Etcd:                      m("quay.io/coreos/etcd:v3.1.12"),
			Kubernetes:                m("gcr.io/google_containers/hyperkube:v1.10.1"),
			Alpine:                    ToolsImage,
			NginxProxy:                ToolsImage,
			CertDownloader:            ToolsImage,
			KubernetesServicesSidecar: ToolsImage,
			KubeDNS:                   m("gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.8"),
			DNSmasq:                   m("gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.8"),
			KubeDNSSidecar:            m("gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.8"),
			KubeDNSAutoscaler:         m("gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.0.0"),
			Flannel:                   m("quay.io/coreos/flannel:v0.9.1"),
			FlannelCNI:                m("quay.io/coreos/flannel-cni:v0.2.0"),
			CalicoNode:                m("quay.io/calico/node:v3.0.2"),
			CalicoCNI:                 m("quay.io/calico/cni:v2.0.0"),
			CalicoCtl:                 m("quay.io/calico/ctl:v2.0.0"),
			CanalNode:                 m("quay.io/calico/node:v2.6.2"),
			CanalCNI:                  m("quay.io/calico/cni:v1.11.0"),
			CanalFlannel:              m("quay.io/coreos/flannel:v0.9.1"),
			WeaveNode:                 m("weaveworks/weave-kube:2.1.2"),
			WeaveCNI:                  m("weaveworks/weave-npc:2.1.2"),
			PodInfraContainer:         m("gcr.io/google_containers/pause-amd64:3.1"),
			Ingress:                   m("rancher/nginx-ingress-controller:0.10.2-rancher2"),
			IngressBackend:            m("k8s.gcr.io/defaultbackend:1.4"),
		},
	}

	// K8sVersionServiceOptions - service options per k8s version
	K8sVersionServiceOptions = map[string]KubernetesServicesOptions{
		"v1.10": {
			KubeAPI: map[string]string{
				"tls-cipher-suites": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			},
			Kubelet: map[string]string{
				"tls-cipher-suites": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			},
		},
	}

	// ToolsSystemImages default images for alert, pipeline, logging
	ToolsSystemImages = struct {
		AlertSystemImages    AlertSystemImages
		PipelineSystemImages PipelineSystemImages
		LoggingSystemImages  LoggingSystemImages
	}{
		AlertSystemImages: AlertSystemImages{
			AlertManager:       m("prom/alertmanager:v0.11.0"),
			AlertManagerHelper: m("rancher/alertmanager-helper:v0.0.2"),
		},
		PipelineSystemImages: PipelineSystemImages{
			Jenkins:       m("jenkins/jenkins:lts"),
			JenkinsJnlp:   m("jenkins/jnlp-slave:3.10-1-alpine"),
			AlpineGit:     m("alpine/git"),
			PluginsDocker: m("plugins/docker"),
		},
		LoggingSystemImages: LoggingSystemImages{
			Fluentd:                       m("rancher/fluentd:v0.1.7"),
			FluentdHelper:                 m("rancher/fluentd-helper:v0.1.2"),
			LogAggregatorFlexVolumeDriver: m("rancher/log-aggregator:v0.1.3"),
			Elaticsearch:                  m("quay.io/pires/docker-elasticsearch-kubernetes:5.6.2"),
			Kibana:                        m("kibana:5.6.4"),
			Busybox:                       ToolsImage,
		},
	}
)
