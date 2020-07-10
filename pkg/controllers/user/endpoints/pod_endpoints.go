package endpoints

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"fmt"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring pods having host port set
// and sending an update event to the corresponding workload
// so the endpoints can be reconciled for those

type PodsController struct {
	podLister          v1.PodLister
	workloadController workloadutil.CommonController
}

func (c *PodsController) sync(key string, obj *corev1.Pod) (runtime.Object, error) {
	if obj == nil && !strings.HasSuffix(key, allEndpoints) {
		return nil, nil
	}

	var pods []*corev1.Pod
	var err error
	if strings.HasSuffix(key, allEndpoints) {
		namespace := ""
		if !strings.EqualFold(key, allEndpoints) {
			namespace = strings.TrimSuffix(key, fmt.Sprintf("/%s", allEndpoints))
		}
		pods, err = c.podLister.List(namespace, labels.NewSelector())
		if err != nil {
			return nil, err
		}
	} else {
		pods = append(pods, obj)
	}

	workloadsToUpdate := map[string]*workloadutil.Workload{}
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if pod.Spec.NodeName != "" && podHasHostPort(pod) {
			workloads, err := c.workloadController.GetWorkloadsMatchingLabels(pod.Namespace, pod.Labels)
			if err != nil {
				return nil, err
			}
			for _, w := range workloads {

				existingPublicEps := getPublicEndpointsFromAnnotations(w.Annotations)
				found := false
				for _, ep := range existingPublicEps {
					if ep.PodName == pod.Name {
						found = true
						break
					}
				}
				// push changes only when
				// a) the workload doesn't have the pod's endpoint for active pod
				// b) pod is removed
				if found == (pod.DeletionTimestamp != nil) {
					workloadsToUpdate[key] = w
				}
			}
		}
	}

	// push changes to workload
	for _, w := range workloadsToUpdate {
		c.workloadController.EnqueueWorkload(w)
	}

	return nil, nil
}

func podHasHostPort(obj *corev1.Pod) bool {
	for _, c := range obj.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort != 0 {
				return true
			}
		}
	}
	return false
}
