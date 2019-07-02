package clusterrouter

import (
	"net/http"
	"strings"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

type ClusterLookup interface {
	Lookup(req *http.Request) (*v3.Cluster, error)
}

func GetClusterID(req *http.Request) string {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) > 3 && strings.HasPrefix(parts[2], "cluster") {
		return parts[3]
	}

	return ""
}
