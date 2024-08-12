package registries

import (
	"fmt"
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// CheckAllClusterPodsForRegistryPrefix checks the pods of a cluster and checks to see if they're coming from the
// expected registry fqdn.
func CheckAllClusterPodsForRegistryPrefix(client *rancher.Client, clusterID, registryPrefix string) (bool, error) {
	downstreamClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return false, err
	}

	steveClient := downstreamClient.SteveType(pods.PodResourceSteveType)
	podsList, err := steveClient.List(nil)
	if err != nil {
		return false, err
	}

	for _, pod := range podsList.Data {
		podSpec := &corev1.PodSpec{}
		err := v1.ConvertToK8sType(pod.Spec, podSpec)
		if err != nil {
			return false, err
		}
		for _, container := range podSpec.Containers {
			if !strings.Contains(container.Image, registryPrefix) {
				log.Warnf("pod/containerImage %s/%s is not using the correct registry prefix", pod.Name, container.Image)
				return false, nil
			}
			log.Infoln(container.Image)
		}
	}
	return true, nil
}

// CheckPodStatusImageSource is an extension that will check if the pod images are pulled from the
// correct registry and checks to see if pod status are in a ready nonerror state.
// Func will return a true if both checks are successful
func CheckPodStatusImageSource(client *rancher.Client, clusterName, registryFQDN string) (bool, []error) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return false, []error{err}
	}

	podErrors := pods.StatusPods(client, clusterID)
	if len(podErrors) != 0 {
		return false, []error{fmt.Errorf("error: pod(s) are in an error state  %v", podErrors)}
	}

	correctRegistryFQDN, err := CheckAllClusterPodsForRegistryPrefix(client, clusterID, registryFQDN)
	if err != nil {
		return false, []error{fmt.Errorf("error: with checking cluster pod registry prefix: %v", err)}
	}

	if !correctRegistryFQDN {
		return false, []error{fmt.Errorf("error: pod images were not pulled from the correct registry")}
	}

	return true, nil
}
