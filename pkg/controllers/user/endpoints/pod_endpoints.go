package endpoints

import (
	"strings"

	"fmt"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring pods
// and setting public endpoints on them based on HostPort pods
// and NodePort/LoadBalancer services backing up the pod

type PodsController struct {
	nodeLister         v1.NodeLister
	nodeController     v1.NodeController
	pods               v1.PodInterface
	podLister          v1.PodLister
	serviceLister      v1.ServiceLister
	workloadController workloadutil.CommonController
	machinesLister     v3.NodeLister
	clusterName        string
}

func (c *PodsController) sync(key string, obj *corev1.Pod) error {
	if obj == nil && !strings.HasSuffix(key, allEndpoints) {
		return nil
	}

	var pods []*corev1.Pod
	var services []*corev1.Service
	var err error
	if strings.HasSuffix(key, allEndpoints) {
		namespace := ""
		if !strings.EqualFold(key, allEndpoints) {
			namespace = strings.TrimSuffix(key, fmt.Sprintf("/%s", allEndpoints))
		}
		pods, err = c.podLister.List(namespace, labels.NewSelector())
		if err != nil {
			return err
		}
		services, err = c.serviceLister.List(namespace, labels.NewSelector())
		if err != nil {
			return err
		}
	} else {
		services, err = c.serviceLister.List(obj.Namespace, labels.NewSelector())
		if err != nil {
			return err
		}
		pods = append(pods, obj)
	}

	nodesToUpdate := map[string]bool{}
	workloadsToUpdate := map[string]*workloadutil.Workload{}
	nodeNameToMachine, err := getNodeNameToMachine(c.clusterName, c.machinesLister, c.nodeLister)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		updated, err := c.updatePodEndpoints(pod, services, nodeNameToMachine)
		if err != nil {
			return err
		}
		if updated {
			if pod.Spec.NodeName != "" && podHasHostPort(pod) {
				nodesToUpdate[pod.Spec.NodeName] = true
				workloads, err := c.workloadController.GetWorkloadsMatchingLabels(pod.Namespace, pod.Labels)
				if err != nil {
					return err
				}
				for _, w := range workloads {
					workloadsToUpdate[key] = w
				}
			}
		}
	}

	// push changes to hosts
	for nodeName := range nodesToUpdate {
		c.nodeController.Enqueue("", nodeName)
	}
	// push changes to workload
	for _, w := range workloadsToUpdate {
		c.workloadController.EnqueueWorkload(w)
	}

	return nil
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

func (c *PodsController) updatePodEndpoints(obj *corev1.Pod, services []*corev1.Service, nodeNameToMachine map[string]*v3.Node) (updated bool, err error) {
	if obj.Spec.NodeName == "" {
		return false, nil
	}

	if obj.DeletionTimestamp != nil {
		return true, nil
	}

	// 1. update pod with endpoints
	// a) from HostPort
	newPublicEps, err := convertHostPortToEndpoint(obj, c.clusterName, nodeNameToMachine[obj.Spec.NodeName])
	if err != nil {
		return false, err
	}
	// b) from services
	for _, svc := range services {
		if svc.Namespace != obj.Namespace {
			continue
		}
		set := labels.Set{}
		for key, val := range svc.Spec.Selector {
			set[key] = val
		}
		selector := labels.SelectorFromSet(set)
		if selector.Matches(labels.Set(obj.Labels)) {
			eps, err := convertServiceToPublicEndpoints(svc, "", nil)
			if err != nil {
				return false, err
			}
			newPublicEps = append(newPublicEps, eps...)
		}
	}

	existingPublicEps := getPublicEndpointsFromAnnotations(obj.Annotations)
	if areEqualEndpoints(existingPublicEps, newPublicEps) {
		return false, nil
	}
	toUpdate := obj.DeepCopy()
	epsToUpdate, err := publicEndpointsToString(newPublicEps)
	if err != nil {
		return false, err
	}

	logrus.Infof("Updating pod [%s/%s] with public endpoints [%v]", obj.Namespace, obj.Name, epsToUpdate)
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = c.pods.Update(toUpdate)
	if err != nil {
		return false, err
	}
	return true, nil
}
