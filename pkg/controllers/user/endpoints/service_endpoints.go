package endpoints

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring NodePort services
// and setting public endpoints on them.

type ServicesController struct {
	services       v1.ServiceInterface
	serviceLister  v1.ServiceLister
	nodeController v1.NodeController
	nodeLister     v1.NodeLister
	podLister      v1.PodLister
	podController  v1.PodController
}

func (s *ServicesController) sync(key string, obj *corev1.Service) error {
	if obj == nil {
		return nil
	}

	var nodePortSvcs []corev1.Service
	if key == allEndpoints {
		svcs, err := s.serviceLister.List("", labels.NewSelector())
		if err != nil {
			return err
		}
		for _, svc := range svcs {
			if svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer" {
				nodePortSvcs = append(nodePortSvcs, *svc)
			}
		}
	} else {
		nodePortSvcs = append(nodePortSvcs, *obj)
	}

	enqueueAllNodes := false
	for _, svc := range nodePortSvcs {
		if svc.DeletionTimestamp != nil {
			enqueueAllNodes = true
			continue
		}
		changed, err := s.reconcileEndpointsForService(&svc)
		if err != nil {
			return err
		}
		if changed {
			enqueueAllNodes = true
		}
	}

	if enqueueAllNodes {
		s.nodeController.Enqueue("", allEndpoints)
	}
	return nil
}

func (s *ServicesController) reconcileEndpointsForService(svc *corev1.Service) (bool, error) {
	// 1. update service with endpoints
	newPublicEps, err := convertServiceToPublicEndpoints(svc, nil)
	if err != nil {
		return false, err
	}

	existingPublicEps := getPublicEndpointsFromAnnotations(svc.Annotations)
	if areEqualEndpoints(existingPublicEps, newPublicEps) {
		return false, nil
	}
	toUpdate := svc.DeepCopy()
	epsToUpdate, err := publicEndpointsToString(newPublicEps)
	if err != nil {
		return false, err
	}

	logrus.Infof("Updating service [%s] with public endpoints [%v]", svc.Name, epsToUpdate)
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = s.services.Update(toUpdate)
	if err != nil {
		return false, err
	}

	// 2. Push changes for pods behind nodePort service
	set := labels.Set{}
	for key, val := range svc.Spec.Selector {
		set[key] = val
	}
	pods, err := s.podLister.List(svc.Namespace, labels.SelectorFromSet(set))
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		s.podController.Enqueue(pod.Namespace, pod.Name)
	}

	return true, nil
}
