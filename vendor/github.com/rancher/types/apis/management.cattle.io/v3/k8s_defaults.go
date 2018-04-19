package v3

const (
	K8sV18  = "v1.8.10-rancher1-1"
	K8sV19  = "v1.9.5-rancher1-1"
	K8sV110 = "v1.10.0-rancher1-1"

	MajorK8sV110 = "v1.10"
)

var (
	ToolsImage = "rancher/rke-tools:v0.1.1"

	// K8sVersionToRKESystemImages - images map for 2.0
	K8sVersionToRKESystemImages = map[string]RKESystemImages{
		K8sV18:  v18SystemImages,
		K8sV19:  v19SystemImages,
		K8sV110: v110SystemImages,
	}

	// K8sVersionServiceOptions - service options per k8s version
	K8sVersionServiceOptions = map[string]KubernetesServicesOptions{
		MajorK8sV110: v110ServiceOptions,
	}

	// K8SVersionToSystemImages16 - images map for 1.6. Keeping it sepate in case we have to diverge
	K8SVersionToSystemImages16 = map[string]RKESystemImages{
		K8sV18:  v18SystemImages,
		K8sV19:  v19SystemImages,
		K8sV110: v110SystemImages,
	}

	// ToolsSystemImages default images for alert, pipeline, logging
	ToolsSystemImages = struct {
		AlertSystemImages    AlertSystemImages
		PipelineSystemImages PipelineSystemImages
		LoggingSystemImages  LoggingSystemImages
	}{
		AlertSystemImages: AlertSystemImages{
			AlertManager:       "prom/alertmanager:v0.11.0",
			AlertManagerHelper: "rancher/alertmanager-helper:v0.0.2",
		},
		PipelineSystemImages: PipelineSystemImages{
			Jenkins:       "jenkins/jenkins:lts",
			JenkinsJnlp:   "jenkins/jnlp-slave:3.10-1-alpine",
			AlpineGit:     "alpine/git",
			PluginsDocker: "plugins/docker",
		},
		LoggingSystemImages: LoggingSystemImages{
			Fluentd:                       "rancher/fluentd:v0.1.6",
			FluentdHelper:                 "rancher/fluentd-helper:v0.1.2",
			LogAggregatorFlexVolumeDriver: "rancher/log-aggregator:v0.1.2",
			Elaticsearch:                  "rancher/docker-elasticsearch-kubernetes:5.6.2",
			Kibana:                        "kibana:5.6.4",
			Busybox:                       "busybox",
		},
	}

	// v18 system images defaults
	v18SystemImages = RKESystemImages{
		Etcd:                      "rancher/coreos-etcd:v3.0.17",
		Kubernetes:                "rancher/hyperkube:v1.8.10",
		Alpine:                    ToolsImage,
		NginxProxy:                ToolsImage,
		CertDownloader:            ToolsImage,
		KubernetesServicesSidecar: ToolsImage,
		KubeDNS:                   "rancher/k8s-dns-kube-dns-amd64:1.14.5",
		DNSmasq:                   "rancher/k8s-dns-dnsmasq-nanny-amd64:1.14.5",
		KubeDNSSidecar:            "rancher/k8s-dns-sidecar-amd64:1.14.5",
		KubeDNSAutoscaler:         "rancher/cluster-proportional-autoscaler-amd64:1.0.0",
		Flannel:                   "rancher/coreos-flannel:v0.9.1",
		FlannelCNI:                "rancher/coreos-flannel-cni:v0.2.0",
		CalicoNode:                "rancher/calico-node:v3.0.2",
		CalicoCNI:                 "rancher/calico-cni:v2.0.0",
		CalicoCtl:                 "rancher/calico-ctl:v2.0.0",
		CanalNode:                 "rancher/calico-node:v2.6.2",
		CanalCNI:                  "rancher/calico-cni:v1.11.0",
		CanalFlannel:              "rancher/coreos-flannel:v0.9.1",
		WeaveNode:                 "weaveworks/weave-kube:2.1.2",
		WeaveCNI:                  "weaveworks/weave-npc:2.1.2",
		PodInfraContainer:         "rancher/pause-amd64:3.0",
		Ingress:                   "rancher/nginx-ingress-controller:0.10.2-rancher2",
		IngressBackend:            "rancher/nginx-ingress-controller-defaultbackend:1.4",
	}

	// v19 system images defaults
	v19SystemImages = RKESystemImages{
		Etcd:                      "rancher/coreos-etcd:v3.1.12",
		Kubernetes:                "rancher/hyperkube:v1.9.5",
		Alpine:                    ToolsImage,
		NginxProxy:                ToolsImage,
		CertDownloader:            ToolsImage,
		KubernetesServicesSidecar: ToolsImage,
		KubeDNS:                   "rancher/k8s-dns-kube-dns-amd64:1.14.7",
		DNSmasq:                   "rancher/k8s-dns-dnsmasq-nanny-amd64:1.14.7",
		KubeDNSSidecar:            "rancher/k8s-dns-sidecar-amd64:1.14.7",
		KubeDNSAutoscaler:         "rancher/cluster-proportional-autoscaler-amd64:1.0.0",
		Flannel:                   "rancher/coreos-flannel:v0.9.1",
		FlannelCNI:                "rancher/coreos-flannel-cni:v0.2.0",
		CalicoNode:                "rancher/calico-node:v3.0.2",
		CalicoCNI:                 "rancher/calico-cni:v2.0.0",
		CalicoCtl:                 "rancher/calico-ctl:v2.0.0",
		CanalNode:                 "rancher/calico-node:v2.6.2",
		CanalCNI:                  "rancher/calico-cni:v1.11.0",
		CanalFlannel:              "rancher/coreos-flannel:v0.9.1",
		WeaveNode:                 "weaveworks/weave-kube:2.1.2",
		WeaveCNI:                  "weaveworks/weave-npc:2.1.2",
		PodInfraContainer:         "rancher/pause-amd64:3.0",
		Ingress:                   "rancher/nginx-ingress-controller:0.10.2-rancher2",
		IngressBackend:            "rancher/nginx-ingress-controller-defaultbackend:1.4",
		Grafana:                   "rancher/heapster-grafana-amd64:v4.4.3",
		Heapster:                  "rancher/heapster-amd64:v1.5.0",
		Influxdb:                  "rancher/heapster-influxdb-amd64:v1.3.3",
		Tiller:                    "rancher/tiller:v2.7.2",
		Dashboard:                 "rancher/kubernetes-dashboard-amd64:v1.8.0",
	}

	// v110 system images defaults
	v110SystemImages = RKESystemImages{
		Etcd:                      "rancher/coreos-etcd:v3.1.12",
		Kubernetes:                "rancher/hyperkube:v1.10.1",
		Alpine:                    ToolsImage,
		NginxProxy:                ToolsImage,
		CertDownloader:            ToolsImage,
		KubernetesServicesSidecar: ToolsImage,
		KubeDNS:                   "rancher/k8s-dns-kube-dns-amd64:1.14.8",
		DNSmasq:                   "rancher/k8s-dns-dnsmasq-nanny-amd64:1.14.8",
		KubeDNSSidecar:            "rancher/k8s-dns-sidecar-amd64:1.14.8",
		KubeDNSAutoscaler:         "rancher/cluster-proportional-autoscaler-amd64:1.0.0",
		Flannel:                   "rancher/coreos-flannel:v0.9.1",
		FlannelCNI:                "rancher/coreos-flannel-cni:v0.2.0",
		CalicoNode:                "rancher/calico-node:v3.0.2",
		CalicoCNI:                 "rancher/calico-cni:v2.0.0",
		CalicoCtl:                 "rancher/calico-ctl:v2.0.0",
		CanalNode:                 "rancher/calico-node:v2.6.2",
		CanalCNI:                  "rancher/calico-cni:v1.11.0",
		CanalFlannel:              "rancher/coreos-flannel:v0.9.1",
		WeaveNode:                 "weaveworks/weave-kube:2.1.2",
		WeaveCNI:                  "weaveworks/weave-npc:2.1.2",
		PodInfraContainer:         "rancher/pause-amd64:3.1",
		Ingress:                   "rancher/nginx-ingress-controller:0.10.2-rancher2",
		IngressBackend:            "rancher/nginx-ingress-controller-defaultbackend:1.4",
		Grafana:                   "rancher/heapster-grafana-amd64:v4.4.3",
		Heapster:                  "rancher/heapster-amd64:v1.5.0",
		Influxdb:                  "rancher/heapster-influxdb-amd64:v1.3.3",
		Tiller:                    "rancher/tiller:v2.8.2",
		Dashboard:                 "rancher/kubernetes-dashboard-amd64:v1.8.3",
	}

	// v110 service options
	v110ServiceOptions = KubernetesServicesOptions{
		KubeAPI: map[string]string{
			"tls-cipher-suites": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		},
		Kubelet: map[string]string{
			"tls-cipher-suites": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		},
	}
)
