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
	Nodelocal     = "nodelocal"
	TemplateKeys  = "templateKeys"

	calicov18            = "calico-v1.8"
	calicov113           = "calico-v1.13"
	calicov115           = "calico-v1.15"
	calicov11512         = "calico-v1.15.12"
	calicov115Privileged = "calico-v1.15-privileged"
	calicov116           = "calico-v1.16"
	calicov117           = "calico-v1.17"
	calicov117Privileged = "calico-v1.17-privileged"

	canalv18                      = "canal-v1.8"
	canalv113                     = "canal-v1.13"
	canalv115                     = "canal-v1.15"
	canalv11512                   = "canal-v1.15.12"
	canalv115Privileged           = "canal-v1.15-privileged"
	canalv115PrivilegedCalico3134 = "canal-v1.15-privileged-calico3134"
	canalv116                     = "canal-v1.16"
	canalv117                     = "canal-v1.17"
	canalv117Privileged           = "canal-v1.17-privileged"
	canalv117PrivilegedCalico3134 = "canal-v1.17-privileged-calico3134"

	flannelv18  = "flannel-v1.8"
	flannelv115 = "flannel-v1.15"
	flannelv116 = "flannel-v1.16"

	coreDnsv18  = "coredns-v1.8"
	coreDnsv116 = "coredns-v1.16"
	coreDnsv117 = "coredns-v1.17"

	kubeDnsv18  = "kubedns-v1.8"
	kubeDnsv116 = "kubedns-v1.16"

	metricsServerv18 = "metricsserver-v1.8"

	weavev18  = "weave-v1.8"
	weavev116 = "weave-v1.16"

	nginxIngressv18    = "nginxingress-v1.8"
	nginxIngressV115   = "nginxingress-v1.15"
	nginxIngressV11512 = "nginxingress-v1.15.12"

	nodelocalv115 = "nodelocal-v1.15"
)

var TemplateIntroducedRanges = map[string][]string{
	Nodelocal: {">=1.17.4-rancher1-1", ">=1.16.8-rancher1-1 <1.17.0-alpha", ">=1.15.11-rancher1-1 <1.16.0-alpha"},
}

func LoadK8sVersionedTemplates() map[string]map[string]string {
	return map[string]map[string]string{
		Calico: {
			">=1.17.4-rancher0":                     calicov117Privileged,
			">=1.17.0-rancher0 <1.17.4-rancher0":    calicov117,
			">=1.16.8-rancher0 <1.17.0-rancher0":    calicov117Privileged,
			">=1.16.4-rancher1 <1.16.8-rancher0":    calicov117,
			">=1.16.0-alpha <1.16.4-rancher1":       calicov116,

			">=1.15.11-rancher1-1 <1.15.12-rancher1-1": calicov115Privileged,
			// 1.15.12-rancher1-1 comes from 2.2.13, uses calicov115 template with new key calicov11512
			// new key is to enable rancher passing template to rke for an already vendored template
			">=1.15.12-rancher1-1 <1.15.12-rancher2-2": calicov11512,
			">=1.15.12-rancher2-2 <1.16.0-alpha": calicov115Privileged,

			">=1.15.0-rancher0 <1.15.11-rancher1-1": calicov115,
			">=1.13.0-rancher0 <1.15.0-rancher0":    calicov113,
			">=1.8.0-rancher0 <1.13.0-rancher0":     calicov18,
		},
		Canal: {
			">=1.17.6-rancher2-1":                      canalv117PrivilegedCalico3134,
			">=1.17.4-rancher0 <1.17.6-rancher2-1":     canalv117Privileged,
			">=1.17.0-rancher0 <1.17.4-rancher0":       canalv117,
			">=1.16.10-rancher2-1 <1.17.0-rancher0":    canalv117PrivilegedCalico3134,
			">=1.16.8-rancher0 <1.16.10-rancher2-1":    canalv117Privileged,
			">=1.16.4-rancher1 <1.16.8-rancher0":       canalv117,
			">=1.16.0-alpha <1.16.4-rancher1":          canalv116,
			">=1.15.12-rancher2-2 <1.16.0-alpha":       canalv115PrivilegedCalico3134,
			">=1.15.11-rancher1-1 <1.15.12-rancher1-1": canalv115Privileged,
			// 1.15.12-rancher1-1 comes from 2.2.13, uses old canalv115 template with new key canalv11512
			// new key is to enable rancher passing template to rke for an already vendored template
			">=1.15.12-rancher1-1 <1.15.12-rancher2-2": canalv11512,
			">=1.15.0-rancher0 <1.15.11-rancher1-1":    canalv115,
			">=1.13.0-rancher0 <1.15.0-rancher0":       canalv113,
			">=1.8.0-rancher0 <1.13.0-rancher0":        canalv18,
		},
		Flannel: {
			">=1.16.0-alpha":                    flannelv116,
			">=1.15.0-rancher0 <1.16.0-alpha":   flannelv115,
			">=1.8.0-rancher0 <1.15.0-rancher0": flannelv18,
		},
		CoreDNS: {
			">=1.17.0-alpha":                 coreDnsv117,
			">=1.16.0-alpha <1.17.0-alpha":   coreDnsv116,
			">=1.8.0-rancher0 <1.16.0-alpha": coreDnsv18,
		},
		KubeDNS: {
			">=1.16.0-alpha":                 kubeDnsv116,
			">=1.8.0-rancher0 <1.16.0-alpha": kubeDnsv18,
		},
		MetricsServer: {
			">=1.8.0-rancher0": metricsServerv18,
		},
		Weave: {
			">=1.16.0-alpha":                 weavev116,
			">=1.8.0-rancher0 <1.16.0-alpha": weavev18,
		},
		NginxIngress: {
			">=1.8.0-rancher0 <1.13.10-rancher1-3":  nginxIngressv18,
			">=1.14.0-rancher0 <=1.14.6-rancher1-1": nginxIngressv18,
			">=1.15.0-rancher0 <=1.15.3-rancher1-1": nginxIngressv18,
			// New ingress template introduced for 1.13.10-rancher1-3, 1.14.6-rancher2, 1.15.3-rancher2
			">=1.13.10-rancher1-3 <1.14.0-rancher0":   nginxIngressV115,
			">=1.14.6-rancher2 <1.15.0-rancher0":      nginxIngressV115,
			">=1.15.3-rancher2 <1.15.12-rancher1-1":   nginxIngressV115,
			">=1.16.1-rancher1-1 <1.16.10-rancher1-1": nginxIngressV115,
			">=1.17.0-rancher1-1 <1.17.6-rancher1-1":  nginxIngressV115,
			// New ingress template introduced for 1.15.12-rancher1-1, 1.16.10-rancher1-1, 1.17.6-rancher1-1
			">=1.15.12-rancher1-1 <1.16.1-rancher1-1": nginxIngressV11512,
			">=1.16.10-rancher1-1 <1.17.0-rancher1-1": nginxIngressV11512,
			">=1.17.6-rancher1-1":                     nginxIngressV11512,
		},
		Nodelocal: {
			">=1.8.0-rancher0": nodelocalv115,
		},
		TemplateKeys: getTemplates(),
	}
}

func getTemplates() map[string]string {
	return map[string]string{
		calicov113:           CalicoTemplateV113,
		calicov115:           CalicoTemplateV115,
		calicov11512:         CalicoTemplateV115,
		calicov115Privileged: CalicoTemplateV115Privileged,
		calicov116:           CalicoTemplateV116,
		calicov117:           CalicoTemplateV117,
		calicov117Privileged: CalicoTemplateV117Privileged,
		calicov18:            CalicoTemplateV112,

		flannelv115: FlannelTemplateV115,
		flannelv116: FlannelTemplateV116,
		flannelv18:  FlannelTemplate,

		canalv113:                     CanalTemplateV113,
		canalv18:                      CanalTemplateV112,
		canalv115:                     CanalTemplateV115,
		canalv11512:                   CanalTemplateV115,
		canalv115Privileged:           CanalTemplateV115Privileged,
		canalv115PrivilegedCalico3134: CanalTemplateV115PrivilegedCalico3134,
		canalv116:                     CanalTemplateV116,
		canalv117:                     CanalTemplateV117,
		canalv117Privileged:           CanalTemplateV117Privileged,
		canalv117PrivilegedCalico3134: CanalTemplateV117PrivilegedCalico3134,

		coreDnsv18:  CoreDNSTemplate,
		coreDnsv116: CoreDNSTemplateV116,
		coreDnsv117: CoreDNSTemplateV117,

		kubeDnsv18:  KubeDNSTemplate,
		kubeDnsv116: KubeDNSTemplateV116,

		metricsServerv18: MetricsServerTemplate,

		weavev18:  WeaveTemplate,
		weavev116: WeaveTemplateV116,

		nginxIngressv18:    NginxIngressTemplate,
		nginxIngressV115:   NginxIngressTemplateV0251Rancher1,
		nginxIngressV11512: NginxIngressTemplateV0320Rancher1,

		nodelocalv115: NodelocalTemplateV115,
	}
}
