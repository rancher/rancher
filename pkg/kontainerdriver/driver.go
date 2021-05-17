package kontainerdriver

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

func GetDriver(cluster *v3.Cluster, driverLister v3.KontainerDriverLister) (string, error) {
	var driver *v3.KontainerDriver
	var err error

	if cluster.Spec.GenericEngineConfig != nil {
		kontainerDriverName := (*cluster.Spec.GenericEngineConfig)["driverName"].(string)
		driver, err = driverLister.Get("", kontainerDriverName)
		if err != nil {
			return "", err
		}
	}

	if cluster.Spec.AKSConfig != nil {
		return apimgmtv3.ClusterDriverAKS, nil
	}

	if cluster.Spec.EKSConfig != nil {
		return apimgmtv3.ClusterDriverEKS, nil
	}

	if cluster.Spec.GKEConfig != nil {
		return apimgmtv3.ClusterDriverGKE, nil
	}

	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		return apimgmtv3.ClusterDriverRKE, nil
	}

	if driver == nil {
		return "", nil
	}

	return driver.Status.DisplayName, nil
}
