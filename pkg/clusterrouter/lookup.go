package clusterrouter

import (
	"net/http"
	"strings"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

type ClusterLookup interface {
	Lookup(req *http.Request) (*v3.Cluster, error)
}

func GetClusterID(req *http.Request) string {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) > 3 &&
		parts[0] == "" &&
		(parts[1] == "k8s" || parts[1] == "v3") &&
		(parts[2] == "clusters" || parts[2] == "cluster") {
		return parts[3]
	}

	return ""
}
