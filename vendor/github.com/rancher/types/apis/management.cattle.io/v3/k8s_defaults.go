package v3

const (
	K8sV1_8 = "v1.8.7-rancher1-1"
)

var (
	K8sVersionToRKESystemImages = map[string]RKESystemImages{
		"v1.8.7-rancher1-1": v187SystemImages,
	}

	RancherK8sVersionToSystemImages = map[string]RancherSystemImages{
		"v1.9.3-rancher1-1": v193SystemImages,
	}

	// v187SystemImages defaults for rke and rancher 2.0
	v187SystemImages = RKESystemImages{
		Etcd:                      "rancher/coreos-etcd:v3.0.17",
		Kubernetes:                "rancher/k8s:v1.8.7-rancher1-1",
		Alpine:                    "alpine:latest",
		NginxProxy:                "rancher/rke-nginx-proxy:v0.1.1",
		CertDownloader:            "rancher/rke-cert-deployer:v0.1.1",
		KubernetesServicesSidecar: "rancher/rke-service-sidekick:v0.1.0",
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
		Ingress:                   "rancher/nginx-ingress-controller:0.10.2",
		IngressBackend:            "rancher/nginx-ingress-controller-defaultbackend:1.4",
	}

	// v193SystemImages defaults for 1.6 with k8s v1.9
	v193SystemImages = RancherSystemImages{
		RKESystemImages: RKESystemImages{
			Etcd:           "rancher/etcd:v2.3.7-13",
			Kubernetes:     "rancher/k8s:v1.9.3-rancher1-1",
			KubeDNS:        "rancher/k8s-dns-kube-dns-amd64:1.14.7",
			DNSmasq:        "rancher/k8s-dns-dnsmasq-nanny-amd64:1.14.7",
			KubeDNSSidecar: "rancher/k8s-dns-sidecar-amd64:1.14.7",
			Ingress:        "rancher/lb-service-rancher:v0.7.17",
		},
		Kubectld:       "rancher/kubectld:v0.8.6",
		EtcHostUpdater: "rancher/etc-host-updater:v0.0.3",
		K8sAgent:       "rancher/kubernetes-agent:v0.6.6",
		K8sAuth:        "rancher/kubernetes-auth:v0.0.8",
		Heapster:       "rancher/heapster-grafana-amd64:v4.4.3",
		Grafana:        "rancher/heapster-amd64:v1.5.0",
		Influxdb:       "rancher/heapster-influxdb-amd64:v1.3.3",
		Tiller:         "rancher/tiller:v2.7.2",
	}
)
