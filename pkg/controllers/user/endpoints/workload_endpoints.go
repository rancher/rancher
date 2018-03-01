package endpoints

import (
	"fmt"
	"strings"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring workloads
// and setting public endpoints on them based on HostPort pods
// and NodePort services backing up the workload

type WorkloadEndpointsController struct {
	serviceLister      v1.ServiceLister
	podLister          v1.PodLister
	WorkloadController workloadutil.CommonController
}

func (c *WorkloadEndpointsController) UpdateEndpoints(key string, obj *workloadutil.Workload) error {
	if obj == nil && key != workloadutil.AllWorkloads {
		return nil
	}
	// do not update endpoints for job, cronJob and for workload owned by controller (ReplicaSet)
	if strings.EqualFold(obj.Kind, "job") || strings.EqualFold(obj.Kind, "cronJob") {
		return nil
	}
	for _, o := range obj.OwnerReferences {
		if *o.Controller {
			return nil
		}
	}

	var workloads []*workloadutil.Workload
	var services []*corev1.Service
	var err error
	if strings.HasSuffix(key, workloadutil.AllWorkloads) {
		namespace := ""
		if !strings.EqualFold(key, allEndpoints) {
			namespace = strings.TrimSuffix(key, fmt.Sprintf("/%s", allEndpoints))
		}
		workloads, err = c.WorkloadController.GetAllWorkloads(namespace)
		if err != nil {
			return err
		}
		services, err = c.serviceLister.List(namespace, labels.NewSelector())

	} else {
		services, err = c.serviceLister.List(obj.Namespace, labels.NewSelector())
		workloads = append(workloads, obj)
	}
	if err != nil {
		return err
	}

	for _, w := range workloads {
		// 1. Get endpoints from services
		var newPublicEps []v3.PublicEndpoint
		for _, svc := range services {
			set := labels.Set{}
			for key, val := range svc.Spec.Selector {
				set[key] = val
			}
			selector := labels.SelectorFromSet(set)
			found := false
			if selector.Matches(labels.Set(w.Labels)) {
				// direct selector match
				found = true
			} else {
				// match based off the workload
				value, ok := svc.Annotations[workloadutil.WorkloadAnnotation]
				if !ok || value == "" {
					continue
				}
				workloadIDs := strings.Split(value, ",")
				for _, workloadID := range workloadIDs {
					splitted := strings.Split(workloadID, ":")
					if len(splitted) != 3 {
						continue
					}
					namespace := splitted[1]
					name := splitted[2]
					if w.Name == name && w.Namespace == namespace {
						found = true
						break
					}
				}
			}
			if found {
				eps, err := convertServiceToPublicEndpoints(svc, nil)
				if err != nil {
					return err
				}
				newPublicEps = append(newPublicEps, eps...)
			}
		}

		// 2. Get endpoints from HostPort pods matching the selector
		set := labels.Set{}
		for key, val := range w.SelectorLabels {
			set[key] = val
		}
		pods, err := c.podLister.List(w.Namespace, labels.SelectorFromSet(set))
		if err != nil {
			return err
		}
		for _, pod := range pods {
			eps, err := convertHostPortToEndpoint(pod)
			if err != nil {
				return err
			}
			newPublicEps = append(newPublicEps, eps...)
		}

		existingPublicEps := getPublicEndpointsFromAnnotations(w.Annotations)
		if areEqualEndpoints(existingPublicEps, newPublicEps) {
			return nil
		}
		epsToUpdate, err := publicEndpointsToString(newPublicEps)
		if err != nil {
			return err
		}

		logrus.Errorf("New eps are %v; old eps are %v", newPublicEps, existingPublicEps)

		logrus.Infof("Updating workload [%s] with public endpoints [%v]", key, epsToUpdate)

		annotations := map[string]string{
			endpointsAnnotation: epsToUpdate,
		}
		if err = c.WorkloadController.UpdateWorkload(w, annotations); err != nil {
			return err
		}
	}
	return nil
}
