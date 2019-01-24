package globaldns

import (
	"fmt"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/core/v1"
)

func gatherIngressEndpoints(ingressEps []v1.LoadBalancerIngress) []string {
	endpoints := []string{}
	for _, ep := range ingressEps {
		if ep.IP != "" {
			endpoints = append(endpoints, ep.IP)
		} else if ep.Hostname != "" {
			endpoints = append(endpoints, ep.Hostname)
		}
	}
	return endpoints
}

func getMultiClusterAppName(multiClusterAppName string) (string, error) {
	split := strings.SplitN(multiClusterAppName, ":", 2)
	if len(split) != 2 {
		return "", fmt.Errorf("error in splitting multiclusterapp ID %v", multiClusterAppName)
	}
	mcappName := split[1]
	return mcappName, nil
}

func prepareGlobalDNSForUpdate(globalDNS *v3.GlobalDNS, ingressEndpoints []string, clusterName string) {
	originalLen := len(globalDNS.Status.Endpoints)
	globalDNS.Status.Endpoints = append(globalDNS.Status.Endpoints, ingressEndpoints...)

	if originalLen > 0 {
		//dedup the endpoints
		globalDNS.Status.Endpoints = dedupEndpoints(globalDNS.Status.Endpoints)
	}

	//update clusterEndpoints for current cluster
	if len(globalDNS.Status.ClusterEndpoints) == 0 {
		globalDNS.Status.ClusterEndpoints = make(map[string][]string)
	}

	clusterEps := globalDNS.Status.ClusterEndpoints[clusterName]

	if ifEndpointsDiffer(clusterEps, ingressEndpoints) {
		clusterEps = append(clusterEps, ingressEndpoints...)

		if len(globalDNS.Status.ClusterEndpoints[clusterName]) > 0 {
			//dedup the endpoints
			clusterEps = dedupEndpoints(clusterEps)
		}
		globalDNS.Status.ClusterEndpoints[clusterName] = clusterEps
	}
}

func prepareGlobalDNSForEndpointsRemoval(globalDNS *v3.GlobalDNS, ingressEndpoints []string) {
	mapRemovedEndpoints := make(map[string]bool)
	for _, ep := range ingressEndpoints {
		mapRemovedEndpoints[ep] = true
	}

	res := []string{}
	for _, ep := range globalDNS.Status.Endpoints {
		if !mapRemovedEndpoints[ep] {
			res = append(res, ep)
		}
	}
	globalDNS.Status.Endpoints = res
}

func dedupEndpoints(endpoints []string) []string {
	mapEndpoints := make(map[string]bool)
	res := []string{}
	for _, ep := range endpoints {
		if !mapEndpoints[ep] {
			mapEndpoints[ep] = true
			res = append(res, ep)
		}
	}
	return res
}

func ifEndpointsDiffer(endpointsOne []string, endpointsTwo []string) bool {
	if len(endpointsOne) != len(endpointsTwo) {
		return true
	}

	mapEndpointsOne := make(map[string]bool)
	for _, ep := range endpointsOne {
		mapEndpointsOne[ep] = true
	}

	for _, ep := range endpointsTwo {
		if !mapEndpointsOne[ep] {
			return true
		}
	}
	return false
}
