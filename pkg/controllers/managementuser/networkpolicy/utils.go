package networkpolicy

import (
	"fmt"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
)

func isNetworkPolicyDisabled(clusterNamespace string, clusterLister v3.ClusterLister) (bool, error) {
	cluster, err := clusterLister.Get("", clusterNamespace)
	if err != nil {
		return false, fmt.Errorf("error getting cluster %v", err)
	}
	return !convert.ToBool(cluster.Annotations[netPolAnnotation]), nil
}

func isNamespaceMoved(namespace string, nsLister v1.NamespaceLister) (bool, error) {
	ns, err := nsLister.Get("", namespace)
	if err != nil {
		return false, fmt.Errorf("error getting ns %v", err)
	}
	if _, ok := ns.Annotations[nslabels.ProjectIDFieldLabel]; !ok {
		return true, nil
	}
	return ns.Annotations[nslabels.ProjectIDFieldLabel] == "", nil
}

func nodePortService(service *corev1.Service) bool {
	for _, port := range service.Spec.Ports {
		if port.NodePort != 0 {
			return true
		}
	}
	return false
}

func hostPortPod(pod *corev1.Pod) bool {
	for _, c := range pod.Spec.Containers {
		for _, port := range c.Ports {
			if port.HostPort != 0 {
				return true
			}
		}
	}
	return false
}
