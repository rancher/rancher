package clusterprovisioner

import (
	"github.com/rancher/kontainer-driver-metadata/rke/templates"
	kontainerengine "github.com/rancher/kontainer-engine/drivers/rke"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

var addonsList = []string{
	templates.Calico,
	templates.Flannel,
	templates.Weave,
	templates.Canal,
	templates.CoreDNS,
	templates.KubeDNS,
	templates.MetricsServer,
	templates.NginxIngress}

type rkeStore struct {
	AddonLister     v3.RKEAddonLister
	Addons          v3.RKEAddonInterface
	SvcOptionLister v3.RKEK8sServiceOptionLister
	SvcOptions      v3.RKEK8sServiceOptionInterface
}

func NewDataStore(addonLister v3.RKEAddonLister, addons v3.RKEAddonInterface, svcOptionLister v3.RKEK8sServiceOptionLister, svcOptions v3.RKEK8sServiceOptionInterface) kontainerengine.Store {
	return &rkeStore{
		AddonLister:     addonLister,
		Addons:          addons,
		SvcOptionLister: svcOptionLister,
		SvcOptions:      svcOptions,
	}
}

func (a *rkeStore) GetAddonTemplates(k8sVersion string) map[string]interface{} {
	data := map[string]interface{}{}
	logrus.Infof("getAddonTemplates from rkeStore, k8sVersion %s", k8sVersion)
	for _, addonName := range addonsList {
		template, err := kd.GetRKEAddonTemplate(k8sVersion, addonName, a.AddonLister, a.Addons)
		if err != nil {
			logrus.Errorf("getAddonTemplates: k8sVersion %s [%v]", k8sVersion, err)
		}
		if template != "" {
			data[addonName] = template
		}
	}
	return data
}

func (a *rkeStore) GetServiceOptions(k8sVersion string) v3.KubernetesServicesOptions {
	logrus.Infof("KINARA entered getting service options; getting service options; getting service options;")
	svcOptions, err := kd.GetRKEK8sServiceOptions(k8sVersion, a.SvcOptionLister, a.SvcOptions)
	if err != nil {
		logrus.Errorf("getK8sServiceOptions: k8sVersion %s [%v]", k8sVersion, err)
	}
	logrus.Infof("KINARA svcOptions %v", svcOptions)
	return *svcOptions
}
