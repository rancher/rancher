package podsecuritypolicy

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/util/version"
)

var ErrClusterVersionIncompatible = errors.New("podsecuritypolicies are not available in Kubernetes v1.25 and above")

// CheckClusterVersion tries to fetch a management.cattle.io Cluster by name, extract its Kubernetes version,
// and check if the cluster supports PodSecurityPolicies. If the version can be parsed, the function checks if it's
// at least 1.25.x. If yes, it returns a special ErrClusterVersionIncompatible error.
func CheckClusterVersion(clusterName string, clusterLister v3.ClusterLister) error {
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
		return ErrClusterVersionIncompatible
	}
	return nil
}
