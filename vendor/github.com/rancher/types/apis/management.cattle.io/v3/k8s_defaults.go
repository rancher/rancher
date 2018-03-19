package v3

const (
	K8sV18 = "v1.8.9-rancher1-1"
)

var (
	K8sVersionToRKESystemImages = map[string]RKESystemImages{
		"v1.8.9-rancher1-1": v18SystemImages,
	}

	// RancherK8sVersionToSystemImages versions must match github release version
	RancherK8sVersionToSystemImages = map[string]RancherSystemImages{
		"v1.9.4-rancher1": v194SystemImages,
	}

	// v187SystemImages defaults for rke and rancher 2.0
	v18SystemImages = RKESystemImages{
		Etcd:                      "rancher/coreos-etcd:v3.0.17",
		Kubernetes:                "rancher/k8s:" + K8sV18,
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

	// v194SystemImages defaults for 1.6 with k8s v1.9
	v194SystemImages = RancherSystemImages{
		RKESystemImages: RKESystemImages{
			KubeDNS:        "k8s-dns-kube-dns-amd64:1.14.7",
			DNSmasq:        "k8s-dns-dnsmasq-nanny-amd64:1.14.7",
			KubeDNSSidecar: "k8s-dns-sidecar-amd64:1.14.7",
		},
		Grafana:   "heapster-grafana-amd64:v4.4.3",
		Heapster:  "heapster-amd64:v1.5.0",
		Influxdb:  "heapster-influxdb-amd64:v1.3.3",
		Tiller:    "tiller:v2.7.2",
		Dashboard: "kubernetes-dashboard-amd64:v1.8.0",
	}
)
