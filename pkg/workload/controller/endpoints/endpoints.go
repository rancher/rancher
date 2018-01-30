package endpoints

import (
	"context"

	"encoding/json"
	"fmt"
	"reflect"

	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	allEndpoints        = "_all_endpoints_"
	endpointsAnnotation = "field.cattle.io/publicEndpoints"
)

type NodesController struct {
	nodes             v1.NodeInterface
	nodeLister        v1.NodeLister
	serviceLister     v1.ServiceLister
	serviceController v1.ServiceController
	podLister         v1.PodLister
	podController     v1.PodController
}

type ServicesController struct {
	services       v1.ServiceInterface
	serviceLister  v1.ServiceLister
	nodeController v1.NodeController
	nodeLister     v1.NodeLister
}

type PodsController struct {
	nodeLister     v1.NodeLister
	nodeController v1.NodeController
	pods           v1.PodInterface
}

func Register(ctx context.Context, workload *config.WorkloadContext) {
	n := &NodesController{
		nodes:             workload.Core.Nodes(""),
		serviceLister:     workload.Core.Services("").Controller().Lister(),
		serviceController: workload.Core.Services("").Controller(),
		nodeLister:        workload.Core.Nodes("").Controller().Lister(),
		podLister:         workload.Core.Pods("").Controller().Lister(),
		podController:     workload.Core.Pods("").Controller(),
	}
	workload.Core.Nodes("").AddHandler("nodesEndpointsController", n.sync)

	s := &ServicesController{
		services:       workload.Core.Services(""),
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		nodeLister:     workload.Core.Nodes("").Controller().Lister(),
		nodeController: workload.Core.Nodes("").Controller(),
	}
	workload.Core.Services("").AddHandler("servicesEndpointsController", s.sync)

	p := &PodsController{
		nodeLister:     workload.Core.Nodes("").Controller().Lister(),
		nodeController: workload.Core.Nodes("").Controller(),
		pods:           workload.Core.Pods(""),
	}
	workload.Core.Pods("").AddHandler("hostPortEndpointsController", p.sync)
}

func (s *PodsController) sync(key string, obj *corev1.Pod) error {
	if obj == nil {
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

	if len(ports) == 0 {
		return nil
	}

	// 1. update pod with endpoints
	newPublicEps, err := convertHostPortToEndpoint(obj)
	if err != nil {
		return err
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

	enqueueAll := false
	var err error
	for _, svc := range nodePortSvcs {
		if svc.DeletionTimestamp != nil {
			enqueueAll = true
			continue
		}
		if enqueueAll, err = s.reconcileEndpointsForService(&svc); err != nil {
			return err
		}
	}

	if enqueueAll {
		s.nodeController.Enqueue("", allEndpoints)
	}
	return nil
}

func (s *ServicesController) reconcileEndpointsForService(svc *corev1.Service) (bool, error) {
	nodes, err := s.nodeLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}

	// 1. update service with endpoints
	newPublicEps, err := convertServiceToPublicEndpoints(nodes, svc)
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
	// 2. push changes to all the nodes (only in case service got an update!)
	return true, nil
}

func (n *NodesController) sync(key string, obj *corev1.Node) error {
	var nodesToSync []corev1.Node
	if key == allEndpoints {
		nodes, err := n.nodeLister.List("", labels.NewSelector())
		if err != nil {
			return err
		}
		for _, node := range nodes {
			if node.DeletionTimestamp == nil {
				nodesToSync = append(nodesToSync, *node)
			}
		}
	} else {
		if obj == nil {
			return nil
		}
		nodesToSync = append(nodesToSync, *obj)
	}

	enqueueAll := false
	var err error
	for _, node := range nodesToSync {
		if node.DeletionTimestamp != nil {
			enqueueAll = true
			continue
		}
		if enqueueAll, err = n.reconcileEndpontsForNode(&node); err != nil {
			return err
		}
	}
	if enqueueAll {
		n.serviceController.Enqueue("", allEndpoints)
	}
	return nil
}

func (n *NodesController) reconcileEndpontsForNode(node *corev1.Node) (bool, error) {
	var newPublicEps []v3.PublicEndpoint

	// Get endpoints from Node port services
	svcs, err := n.serviceLister.List("", labels.NewSelector())
	if err != nil {
		return false, err
	}
	for _, svc := range svcs {
		if svc.DeletionTimestamp != nil {
			continue
		}
		if svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer" {
			pEps, err := convertServiceToPublicEndpoints([]*corev1.Node{node}, svc)
			if err != nil {
				return false, err
			}
			if len(pEps) == 0 {
				continue
			}
			newPublicEps = append(newPublicEps, pEps...)
		}
	}

	// Get endpoint from hostPort pods
	pods, err := n.podLister.List("", labels.NewSelector())
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil || pod.Spec.NodeName != node.Name {
			continue
		}
		for _, c := range pod.Spec.Containers {
			found := false
			for _, p := range c.Ports {
				if p.HostPort != 0 {
					pEps, err := convertHostPortToEndpoint(pod)
					if err != nil {
						return false, err
					}
					if len(pEps) == 0 {
						continue
					}
					newPublicEps = append(newPublicEps, pEps...)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}

	// 1. update node with endpoints
	existingPublicEps := getPublicEndpointsFromAnnotations(node.Annotations)
	if areEqualEndpoints(existingPublicEps, newPublicEps) {
		return false, nil
	}
	toUpdate := node.DeepCopy()
	epsToUpdate, err := publicEndpointsToString(newPublicEps)
	if err != nil {
		return false, err
	}
	logrus.Infof("Updating node [%s] with public endpoints [%v]", node.Name, epsToUpdate)
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[endpointsAnnotation] = epsToUpdate
	_, err = n.nodes.Update(toUpdate)
	if err != nil {
		return false, err
	}

	return true, nil
}

func areEqualEndpoints(one []v3.PublicEndpoint, two []v3.PublicEndpoint) bool {
	oneMap := make(map[string]bool)
	twoMap := make(map[string]bool)
	for _, value := range one {
		oneMap[publicEndpointToString(value)] = true
	}
	for _, value := range two {
		twoMap[publicEndpointToString(value)] = true
	}
	return reflect.DeepEqual(oneMap, twoMap)
}

func publicEndpointsToString(eps []v3.PublicEndpoint) (string, error) {
	b, err := json.Marshal(eps)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func getPublicEndpointsFromAnnotations(annotations map[string]string) []v3.PublicEndpoint {
	var eps []v3.PublicEndpoint
	if annotations == nil {
		return eps
	}
	if val, ok := annotations[endpointsAnnotation]; ok {
		err := json.Unmarshal([]byte(val), &eps)
		if err != nil {
			logrus.Errorf("Failed to read public endpoints from annotation %v", err)
			return eps
		}
	}
	return eps
}

func convertServiceToPublicEndpoints(nodes []*corev1.Node, svc *corev1.Service) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	nodeNameToIP := make(map[string]string)

	for _, node := range nodes {
		if val, ok := node.Annotations["alpha.kubernetes.io/provided-node-ip"]; ok {
			nodeIP := string(val)
			if nodeIP == "" {
				logrus.Warnf("Node [%s] has no ip address set", node.Name)
			} else {
				nodeNameToIP[node.Name] = nodeIP
			}
		}
	}

	for nodeName, nodeIP := range nodeNameToIP {
		for _, port := range svc.Spec.Ports {
			if port.NodePort == 0 {
				continue
			}
			p := v3.PublicEndpoint{
				Node:     nodeName,
				Address:  nodeIP,
				Port:     port.NodePort,
				Protocol: string(port.Protocol),
				Service:  fmt.Sprintf("%s/%s", svc.Namespace, svc.Name),
			}
			eps = append(eps, p)
		}
	}

	return eps, nil
}

func convertHostPortToEndpoint(pod *corev1.Pod) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	nodeName := pod.Spec.NodeName

	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort == 0 {
				continue
			}
			p := v3.PublicEndpoint{
				Node:     nodeName,
				Address:  pod.Status.HostIP,
				Port:     p.HostPort,
				Protocol: string(p.Protocol),
				Pod:      fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
			}
			eps = append(eps, p)
		}
	}

	return eps, nil
}

func publicEndpointToString(p v3.PublicEndpoint) string {
	return fmt.Sprintf("%s_%s_%v_%s_%s_%s", p.Node, p.Address, p.Port, p.Protocol, p.Service, p.Pod)
}
