package podsecuritypolicy

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/util/version"
)

var errVersionIncompatible = errors.New("podsecuritypolicies are not available in Kubernetes v1.25 and above")

const clusterVersionCheckErrorString = "failed to check cluster version compatibility for podsecuritypolicy controllers: %v"

// checkClusterVersion tries to fetch a cluster by name, extract its Kubernetes version,
// and check if the version is less than v1.25.
func checkClusterVersion(clusterName string, clusterLister v3.ClusterLister) error {
	cluster, err := clusterLister.Get("", clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster %s: %w", clusterName, err)
	}
	if cluster.Status.Version == nil {
		return fmt.Errorf("cluster %s version is not available yet", clusterName)
	}
	v, err := version.ParseSemantic(cluster.Status.Version.String())
	if err != nil {
		return err
	}
	v125, err := version.ParseSemantic("v1.25.0")
	if err != nil {
		return err
	}
	if v.AtLeast(v125) {
		return errVersionIncompatible
	}
	return nil
}
