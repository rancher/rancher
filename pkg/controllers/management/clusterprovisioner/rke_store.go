package clusterprovisioner

import (
	"strings"

	kontainerengine "github.com/rancher/kontainer-engine/drivers/rke"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rkeStore struct {
	AddonLister        v3.RKEAddonLister
	Addons             v3.RKEAddonInterface
	SvcOptionLister    v3.RKEK8sServiceOptionLister
	SvcOptions         v3.RKEK8sServiceOptionInterface
	SystemImagesLister v3.RKEK8sSystemImageLister
	SystemImages       v3.RKEK8sSystemImageInterface
}

func NewDataStore(addonLister v3.RKEAddonLister, addons v3.RKEAddonInterface,
	svcOptionLister v3.RKEK8sServiceOptionLister, svcOptions v3.RKEK8sServiceOptionInterface,
	sysImageLister v3.RKEK8sSystemImageLister, sysImages v3.RKEK8sSystemImageInterface) kontainerengine.Store {
	return &rkeStore{
		AddonLister:        addonLister,
		Addons:             addons,
		SvcOptionLister:    svcOptionLister,
		SvcOptions:         svcOptions,
		SystemImagesLister: sysImageLister,
		SystemImages:       sysImages,
	}
}

func (a *rkeStore) GetAddonTemplates(k8sVersion string) map[string]interface{} {
	data := map[string]interface{}{}
	sysImage, err := a.SystemImagesLister.Get(namespace.GlobalNamespace, k8sVersion)
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("getAddonTemplates: error finding system image for %s %v", k8sVersion, err)
			return data
		}
		sysImage, err = a.SystemImages.GetNamespaced(namespace.GlobalNamespace, k8sVersion, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("getAddonTemplates: error finding system image for %s %v", k8sVersion, err)
			return data
		}
	}
	for k, v := range sysImage.Labels {
		if strings.HasPrefix(k, "cattle.io") || strings.HasPrefix(k, "io.cattle") {
			continue
		}
		template, err := kd.GetRKEAddonTemplate(v, a.AddonLister, a.Addons)
		if err != nil {
			logrus.Errorf("getAddonTemplates: k8sVersion %s addon %s [%v]", k8sVersion, v, err)
		}
		if template != "" {
			data[k] = template
		}
	}
	return data
}

func (a *rkeStore) GetServiceOptions(k8sVersion string) *v3.KubernetesServicesOptions {
	svcOptions, err := kd.GetRKEK8sServiceOptions(k8sVersion, a.SvcOptionLister, a.SvcOptions)
	if err != nil {
		logrus.Errorf("getK8sServiceOptions: k8sVersion %s [%v]", k8sVersion, err)
	}
	return svcOptions
}
