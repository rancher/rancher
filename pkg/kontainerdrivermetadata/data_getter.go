package kontainerdrivermetadata

import (
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	rketypes "github.com/rancher/rke/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRKESystemImages(k8sVersion string, sysImageLister v3.RkeK8sSystemImageLister, sysImages v3.RkeK8sSystemImageInterface) (rketypes.RKESystemImages, error) {
	sysImage, err := sysImageLister.Get(namespace.GlobalNamespace, k8sVersion)
	if err != nil {
		sysImage, err = sysImages.GetNamespaced(namespace.GlobalNamespace, k8sVersion, metav1.GetOptions{})
		if err != nil {
			return rketypes.RKESystemImages{}, err
		}
	}
	return sysImage.SystemImages, nil
}

func GetRKEAddonTemplate(addonName string, addonLister v3.RkeAddonLister, addons v3.RkeAddonInterface) (string, error) {
	addon, err := addonLister.Get(namespace.GlobalNamespace, addonName)
	if err != nil {
		addon, err = addons.GetNamespaced(namespace.GlobalNamespace, addonName, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	return addon.Template, nil
}
