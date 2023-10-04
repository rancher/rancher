package podsecuritypolicy

import (
	"errors"
	"fmt"

	mVersion "github.com/mcuadros/go-version"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

var errVersionIncompatible = errors.New("podsecuritypolicies are not available in Kubernetes v1.25 and above")
var clusterVersionCheckErrorString = "failed to check cluster version compatibility for podsecuritypolicy controllers: %v"

// checkClusterVersion tries to fetch a cluster by name, extract its Kubernetes version,
// and check if the version is less than v1.25.
func checkClusterVersion(clusterName string, clusterLister v3.ClusterLister) error {
	cluster, err := clusterLister.Get("", clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster [%s]: %w", clusterName, err)
	}
	if cluster.Status.Version == nil {
		return fmt.Errorf("cannot validate Kubernetes version for podsecuritypolicy capability: cluster [%s] status version is not available yet", clusterName)
	}
	if len(cluster.Status.Version.String()) < 5 {
		return fmt.Errorf("cannot validate Kubernetes version for podsecuritypolicy capability: cluster [%s] status version [%s] is too small", clusterName, cluster.Status.Version.String())
	}
	if mVersion.Compare(cluster.Status.Version.String()[0:5], "v1.25", ">=") {
		return errVersionIncompatible
	}
	return nil
}
