package endpoints

import (
	"encoding/json"
	"fmt"
	"strings"

	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring workloads
// and setting public endpoints on them based on HostPort pods
// and NodePort services backing up the workload

type WorkloadEndpointsController struct {
	ingressLister      v1beta1.IngressLister
	serviceLister      v1.ServiceLister
	podLister          v1.PodLister
	WorkloadController workloadutil.CommonController
	machinesLister     managementv3.NodeLister
	nodeLister         v1.NodeLister
	clusterName        string
	isRKE              bool
}

func (c *WorkloadEndpointsController) UpdateEndpoints(key string, obj *workloadutil.Workload) error {
	if obj == nil && key != allEndpoints {
		return nil
	}

	var workloads []*workloadutil.Workload
	var services []*corev1.Service
	var ingresses []*extensionsv1beta1.Ingress
	var err error
	if strings.HasSuffix(key, allEndpoints) {
		namespace := ""
		if !strings.EqualFold(key, allEndpoints) {
			namespace = strings.TrimSuffix(key, fmt.Sprintf("/%s", allEndpoints))
		}
		workloads, err = c.WorkloadController.GetAllWorkloads(namespace)
		if err != nil {
			return err
		}
		services, err = c.serviceLister.List(namespace, labels.NewSelector())
		if err != nil {
			return err
		}
		ingresses, err = c.ingressLister.List(namespace, labels.NewSelector())

	} else {
		// do not update endpoints for job, cronJob and for workload owned by controller (ReplicaSet)
		if strings.EqualFold(obj.Kind, "job") || strings.EqualFold(obj.Kind, "cronJob") {
			return nil
		}
		for _, o := range obj.OwnerReferences {
			if o.Controller != nil && *o.Controller {
				return nil
			}
		}
		ingresses, err = c.ingressLister.List(obj.Namespace, labels.NewSelector())
		if err != nil {
			return err
		}
		services, err = c.serviceLister.List(obj.Namespace, labels.NewSelector())
		workloads = append(workloads, obj)
	}
	if err != nil {
		return err
	}

	nodeNameToMachine, err := getNodeNameToMachine(c.clusterName, c.machinesLister, c.nodeLister)
	if err != nil {
		return err
	}
	allNodesIP, err := getAllNodesPublicEndpointIP(c.machinesLister, c.clusterName)
	if err != nil {
		return err
	}
	// get ingress endpoint group by service
	serviceToIngressEndpoints := make(map[string][]v3.PublicEndpoint)
	for _, ingress := range ingresses {
		epsMap := convertIngressToServicePublicEndpointsMap(ingress, c.isRKE)
		for k, v := range epsMap {
			serviceToIngressEndpoints[k] = append(serviceToIngressEndpoints[k], v...)
		}
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
			if selector.Matches(labels.Set(w.SelectorLabels)) && !selector.Empty() {
				// direct selector match
				found = true
			} else {
				// match based off the workload
				value, ok := svc.Annotations[workloadutil.WorkloadAnnotation]
				if !ok || value == "" {
					continue
				}
				var workloadIDs []string
				err := json.Unmarshal([]byte(value), &workloadIDs)
				if err != nil {
					logrus.WithError(err).Errorf("Unmarshalling %s workloadIDs of %s Error", value, svc.Name)
					continue
				}
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
				eps, err := convertServiceToPublicEndpoints(svc, "", nil, allNodesIP)
				if err != nil {
					return err
				}
				newPublicEps = append(newPublicEps, eps...)
				if ingressEndpoints, ok := serviceToIngressEndpoints[svc.Name]; ok {
					newPublicEps = append(newPublicEps, ingressEndpoints...)
				}
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
			eps, err := convertHostPortToEndpoint(pod, c.clusterName, nodeNameToMachine[pod.Spec.NodeName])
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
