package clusterrouter

import (
	"net/http"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

type ClusterLookup interface {
	Lookup(req *http.Request) (*v3.Cluster, error)
}

// GetClusterID extracts the cluster ID from the request URL path. It expects the path to be in the format:
//
// /k8s/clusters/{clusterID}/... or /v3/clusters/{clusterID}/... or /v3/cluster/{clusterID}/... or /v3/proxy/{clusterID}/...
func GetClusterID(req *http.Request) string {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) > 3 &&
		parts[0] == "" &&
		(parts[1] == "k8s" || parts[1] == "v3") &&
		(parts[2] == "clusters" || parts[2] == "cluster" || parts[2] == "proxy") {
		return parts[3]
	}

	return ""
}
