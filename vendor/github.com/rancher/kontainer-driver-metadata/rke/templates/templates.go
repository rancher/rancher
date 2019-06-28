package templates

/*
Not including Vsphere(cloudProvider) and Authz templates
Will they change and require Rancher to pass them to RKE
depending on Kubernetes version?
*/

const (
	Calico        = "calico"
	Canal         = "canal"
	Flannel       = "flannel"
	Weave         = "weave"
	CoreDNS       = "coreDNS"
	KubeDNS       = "kubeDNS"
	MetricsServer = "metricsServer"
	NginxIngress  = "nginxIngress"
)

func LoadK8sVersionedTemplates() map[string]map[string]string {
	return map[string]map[string]string{
		Calico: {
			"v1.15":   CalicoTemplateV115,
			"v1.14":   CalicoTemplateV113,
			"v1.13":   CalicoTemplateV113,
			"default": CalicoTemplateV112,
		},
		Canal: {
			"v1.15":   CanalTemplateV115,
			"v1.14":   CanalTemplateV113,
			"v1.13":   CanalTemplateV113,
			"default": CanalTemplateV112,
		},
		Flannel: {
			"v1.15":   FlannelTemplateV115,
			"default": FlannelTemplate,
		},
		CoreDNS: {
			"default": CoreDNSTemplate,
		},
		KubeDNS: {
			"default": KubeDNSTemplate,
		},
		MetricsServer: {
			"default": MetricsServerTemplate,
		},
		Weave: {
			"default": WeaveTemplate,
		},
		NginxIngress: {
			"default": NginxIngressTemplate,
		},
	}
}
