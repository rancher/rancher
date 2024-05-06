package features

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/apiextensions.k8s.io/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	t = true
)

func MigrateFeatures(featuresClient managementv3.FeatureClient, crdClient v1.CustomResourceDefinitionClient, mgmtClusterClient managementv3.ClusterClient) error {
	features, err := featuresClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(features.Items) == 0 {
		return nil
	}

	hasLegacy := false
	for _, feature := range features.Items {
		switch feature.Name {
		case Legacy.Name():
			hasLegacy = true
		case MCM.Name():
			if err := enableMCMIfPreviouslyEnabled(&feature, featuresClient, crdClient); err != nil {
				return err
			}
		case RKE2.Name():
			if err := enableRKE2IfClustersExist(&feature, featuresClient, mgmtClusterClient); err != nil {
				return err
			}
		}
	}

	if !hasLegacy {
		_, err = featuresClient.Create(&v3.Feature{
			ObjectMeta: metav1.ObjectMeta{
				Name: Legacy.Name(),
			},
			Spec: v3.FeatureSpec{
				Value: &t,
			},
		})
		return err
	}

	return nil
}

func enableMCMIfPreviouslyEnabled(feature *v3.Feature, featuresClient managementv3.FeatureClient, crdClient v1.CustomResourceDefinitionClient) error {
	if feature.Spec.Value == nil || *feature.Spec.Value {
		return nil
	}

	_, err := crdClient.Get("nodes.management.cattle.io", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return SetFeature(featuresClient, MCM.Name(), true)
}

func enableRKE2IfClustersExist(feature *v3.Feature, featuresClient managementv3.FeatureClient, mgmtClusterClient managementv3.ClusterClient) error {
	if feature.Spec.Value == nil || *feature.Spec.Value {
		return nil
	}

	clusters, err := mgmtClusterClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, c := range clusters.Items {
		if imported.IsAdministratedByProvisioningCluster(&c) {
			feature = feature.DeepCopy()
			feature.Spec.Value = &[]bool{true}[0]
			feature.Status.LockedValue = feature.Spec.Value

			_, err = featuresClient.Update(feature)
			return err
		}
	}

	return nil
}
