package rotatecerts

import (
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
)

// RotateCertificates is a helper function to rotate the certificates on an RKE2 or k3s cluster. Returns error if any.
func RotateCertificates(client *rancher.Client, clustername string, namespace string) error {

	clusterObj, existingSteveAPIObj, err := clusters.GetProvisioningClusterByName(client, clustername, namespace)
	if err != nil {
		return err
	}
	clusterSpec := &apisV1.ClusterSpec{}
	err = v1.ConvertToK8sType(clusterObj.Spec, clusterSpec)
	if err != nil {
		return err
	}
	if clusterSpec.RKEConfig.RotateCertificates != nil {
		clusterObj.Spec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
			Generation: clusterObj.Spec.RKEConfig.RotateCertificates.Generation + 1,
		}
	} else {
		clusterObj.Spec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
			Generation: 1,
		}
	}

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}
