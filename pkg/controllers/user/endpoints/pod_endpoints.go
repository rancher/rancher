package endpoints

import (
	"github.com/rancher/types/apis/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring pods
// and setting public endpoints on them based on HostPort pods
// and NodePort services

type PodsController struct {
	nodeLister     v1.NodeLister
	nodeController v1.NodeController
	pods           v1.PodInterface
	serviceLister  v1.ServiceLister
}

func (s *PodsController) sync(key string, obj *corev1.Pod) error {
	if obj == nil {
		return nil
	}

	if obj.Spec.NodeName == "" {
		return nil
	}

	if obj.DeletionTimestamp != nil {
		s.nodeController.Enqueue("", obj.Spec.NodeName)
		return nil
	}

	var ports []int32
	for _, c := range obj.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort != 0 {
				ports = append(ports, p.HostPort)
			}
		}
	}

	// 1. update pod with endpoints
	// a) from HostPort
	newPublicEps, err := convertHostPortToEndpoint(obj)
	if err != nil {
		return err
	}
	// b) from NodePort services
	services, err := s.serviceLister.List(obj.Namespace, labels.NewSelector())
	if err != nil {
		return err
	}

	for _, svc := range services {
		set := labels.Set{}
		for key, val := range svc.Spec.Selector {
			set[key] = val
		}
		selector := labels.SelectorFromSet(set)
		if selector.Matches(labels.Set(obj.Labels)) {
			eps, err := convertServiceToPublicEndpoints(svc, nil)
			if err != nil {
				return err
			}
			newPublicEps = append(newPublicEps, eps...)
		}
	}

	existingPublicEps := getPublicEndpointsFromAnnotations(obj.Annotations)
	if areEqualEndpoints(existingPublicEps, newPublicEps) {
		return nil
	}
	toUpdate := obj.DeepCopy()
	epsToUpdate, err := publicEndpointsToString(newPublicEps)
	if err != nil {
		return err
	}

	logrus.Infof("Updating pod [%s] with public endpoints [%v]", key, epsToUpdate)
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = s.pods.Update(toUpdate)
	if err != nil {
		return err
	}
	// 2. push changes to host (only when pod got updates)
	s.nodeController.Enqueue("", obj.Spec.NodeName)

	return nil
}
