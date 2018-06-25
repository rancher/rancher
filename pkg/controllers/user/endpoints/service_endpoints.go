package endpoints

import (
	workloadutil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring services
// and setting public endpoints on them (if they are of type NodePort or LoadBalancer)

type ServicesController struct {
	services           v1.ServiceInterface
	serviceLister      v1.ServiceLister
	nodeController     v1.NodeController
	nodeLister         v1.NodeLister
	podLister          v1.PodLister
	podController      v1.PodController
	workloadController workloadutil.CommonController
	machinesLister     v3.NodeLister
	clusterName        string
}

func (s *ServicesController) sync(key string, obj *corev1.Service) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		namespace := ""
		if obj != nil {
			namespace = obj.Namespace
		}
		// push changes to all the nodes and pods, so service
		// endpoints can be removed from there
		s.nodeController.Enqueue("", allEndpoints)
		//since service is removed, there is no way to narrow down the pod/workload search
		s.podController.Enqueue(namespace, allEndpoints)
		s.workloadController.EnqueueAllWorkloads(namespace)
		return nil
	}
	_, err := s.reconcileEndpointsForService(obj)
	if err != nil {
		return err
	}

	return nil
}

func (s *ServicesController) reconcileEndpointsForService(svc *corev1.Service) (bool, error) {
	// 1. update service with endpoints
	if err := s.updateServiceEndpoint(svc); err != nil {
		return false, err
	}

	// 2. Push changes for pods behind the service
	if err := s.notifyRelatedPods(svc); err != nil {
		return false, err
	}

	// 3. Push changes to workload behind the service
	if err := s.notifyRelatedWorloads(svc); err != nil {
		return false, err
	}

	return true, nil
}

func (s *ServicesController) updateServiceEndpoint(svc *corev1.Service) error {
	allNodesIP, err := getAllNodesPublicEndpointIP(s.machinesLister, s.clusterName)
	if err != nil {
		return err
	}
	newPublicEps, err := convertServiceToPublicEndpoints(svc, "", nil, allNodesIP)
	if err != nil {
		return err
	}

	existingPublicEps := getPublicEndpointsFromAnnotations(svc.Annotations)
	if areEqualEndpoints(existingPublicEps, newPublicEps) {
		return nil
	}
	toUpdate := svc.DeepCopy()
	epsToUpdate, err := publicEndpointsToString(newPublicEps)
	if err != nil {
		return err
	}

	logrus.Infof("Updating service [%s] with public endpoints [%v]", svc.Name, epsToUpdate)
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = map[string]string{}
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = s.services.Update(toUpdate)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServicesController) notifyRelatedPods(svc *corev1.Service) error {
	var pods []*corev1.Pod
	set := labels.Set{}
	for key, val := range svc.Spec.Selector {
		set[key] = val
	}
	pods, err := s.podLister.List(svc.Namespace, labels.SelectorFromSet(set))
	if err != nil {
		return err
	}
	for _, pod := range pods {
		s.podController.Enqueue(pod.Namespace, pod.Name)
	}
	return nil
}

func (s *ServicesController) notifyRelatedWorloads(svc *corev1.Service) error {
	workloads, err := s.workloadController.GetWorkloadsMatchingSelector(svc.Namespace, svc.Spec.Selector)
	if err != nil {
		return err
	}
	for _, w := range workloads {
		s.workloadController.EnqueueWorkload(w)
	}
	return nil
}
