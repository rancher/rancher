package endpoints

import (
	"context"
	"sort"

	"encoding/json"
	"fmt"
	"reflect"

	workloadUtil "github.com/rancher/rancher/pkg/controllers/user/workload"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/types/apis/core/v1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	allEndpoints              = "_all_endpoints_"
	endpointsAnnotation       = "field.cattle.io/publicEndpoints"
	ExternalAddressAnnotation = "rke.cattle.io/external-ip"
)

func Register(ctx context.Context, workload *config.UserContext) {
	isRKE := false
	cluster, err := workload.Management.Management.Clusters("").Get(workload.ClusterName, metav1.GetOptions{})
	if err != nil {
		logrus.WithError(err).Warnf("Can not get cluster %s when registering endpoint controller", workload.ClusterName)
	}
	if cluster != nil {
		//assume that cluster always has a spec
		isRKE = cluster.Spec.RancherKubernetesEngineConfig != nil
	}
	n := &NodesController{
		nodes:          workload.Core.Nodes(""),
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		nodeLister:     workload.Core.Nodes("").Controller().Lister(),
		podLister:      workload.Core.Pods("").Controller().Lister(),
		machinesLister: workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
		clusterName:    workload.ClusterName,
	}
	workload.Core.Nodes("").AddHandler("nodesEndpointsController", n.sync)

	s := &ServicesController{
		services:           workload.Core.Services(""),
		serviceLister:      workload.Core.Services("").Controller().Lister(),
		nodeLister:         workload.Core.Nodes("").Controller().Lister(),
		nodeController:     workload.Core.Nodes("").Controller(),
		podLister:          workload.Core.Pods("").Controller().Lister(),
		podController:      workload.Core.Pods("").Controller(),
		workloadController: workloadUtil.NewWorkloadController(workload.UserOnlyContext(), nil),
		machinesLister:     workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
	}
	workload.Core.Services("").AddHandler("servicesEndpointsController", s.sync)

	p := &PodsController{
		nodeLister:         workload.Core.Nodes("").Controller().Lister(),
		nodeController:     workload.Core.Nodes("").Controller(),
		pods:               workload.Core.Pods(""),
		serviceLister:      workload.Core.Services("").Controller().Lister(),
		podLister:          workload.Core.Pods("").Controller().Lister(),
		workloadController: workloadUtil.NewWorkloadController(workload.UserOnlyContext(), nil),
		machinesLister:     workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
		clusterName:        workload.ClusterName,
	}
	workload.Core.Pods("").AddHandler("hostPortEndpointsController", p.sync)

	w := &WorkloadEndpointsController{
		ingressLister:  workload.Extensions.Ingresses("").Controller().Lister(),
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		podLister:      workload.Core.Pods("").Controller().Lister(),
		machinesLister: workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
		nodeLister:     workload.Core.Nodes("").Controller().Lister(),
		clusterName:    workload.ClusterName,
		isRKE:          isRKE,
	}
	w.WorkloadController = workloadUtil.NewWorkloadController(workload.UserOnlyContext(), w.UpdateEndpoints)

	i := &IngressEndpointsController{
		workloadController: workloadUtil.NewWorkloadController(workload.UserOnlyContext(), nil),
		ingressInterface:   workload.Extensions.Ingresses(""),
		isRKE:              isRKE,
	}
	workload.Extensions.Ingresses("").AddHandler("ingressEndpointsController", i.sync)
}

func areEqualEndpoints(one []v3.PublicEndpoint, two []v3.PublicEndpoint) bool {
	oneMap := map[string]bool{}
	twoMap := map[string]bool{}
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

func getEndpointNodeIP(node *managementv3.Node) string {
	externalIP := ""
	internalIP := ""
	for _, ip := range node.Status.InternalNodeStatus.Addresses {
		if ip.Type == "ExternalIP" && ip.Address != "" {
			externalIP = ip.Address
			break
		} else if ip.Type == "InternalIP" && ip.Address != "" {
			internalIP = ip.Address
		}
	}

	if externalIP != "" {
		return externalIP
	}

	if node.Annotations != nil {
		externalIP = node.Status.NodeAnnotations[ExternalAddressAnnotation]
		if externalIP != "" {
			return externalIP
		}
	}

	return internalIP
}

func convertServiceToPublicEndpoints(svc *corev1.Service, clusterName string, node *managementv3.Node) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	if svc.DeletionTimestamp != nil {
		return eps, nil
	}
	if !(svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer") {
		return eps, nil
	}
	address := ""
	nodeName := ""
	if node != nil {
		address = getEndpointNodeIP(node)
		nodeName = fmt.Sprintf("%s:%s", clusterName, node.Name)
	}

	svcName := fmt.Sprintf("%s:%s", svc.Namespace, svc.Name)
	if svc.Spec.Type == "NodePort" {
		for _, port := range svc.Spec.Ports {
			if port.NodePort == 0 {
				continue
			}
			p := v3.PublicEndpoint{
				NodeName:    nodeName,
				Port:        port.NodePort,
				Addresses:   []string{},
				Protocol:    string(port.Protocol),
				ServiceName: svcName,
				AllNodes:    true,
			}
			//for getting endpoints of specific node
			if address != "" {
				p.Addresses = append(p.Addresses, address)
			}
			eps = append(eps, p)
		}
	} else {
		var addresses []string
		for _, ingressEp := range svc.Status.LoadBalancer.Ingress {
			address := ingressEp.Hostname
			if address == "" {
				address = ingressEp.IP
			}
			addresses = append(addresses, address)
		}
		if len(addresses) > 0 {
			for _, port := range svc.Spec.Ports {
				p := v3.PublicEndpoint{
					NodeName:    "",
					Addresses:   addresses,
					Port:        port.Port,
					Protocol:    string(port.Protocol),
					ServiceName: svcName,
					AllNodes:    false,
				}
				eps = append(eps, p)
			}
		}
	}

	return eps, nil
}

func convertHostPortToEndpoint(pod *corev1.Pod, clusterName string, node *managementv3.Node) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	if pod.DeletionTimestamp != nil {
		return eps, nil
	}
	if pod.Status.Phase != corev1.PodRunning {
		return eps, nil
	}
	if node == nil {
		return eps, nil
	}
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort == 0 {
				continue
			}
			address := getEndpointNodeIP(node)
			p := v3.PublicEndpoint{
				NodeName:  fmt.Sprintf("%s:%s", clusterName, node.Name),
				Addresses: []string{address},
				Port:      p.HostPort,
				Protocol:  string(p.Protocol),
				PodName:   fmt.Sprintf("%s:%s", pod.Namespace, pod.Name),
				AllNodes:  false,
			}
			eps = append(eps, p)
		}
	}

	return eps, nil
}

func publicEndpointToString(p v3.PublicEndpoint) string {
	sort.Strings(p.Addresses)
	return fmt.Sprintf("%s_%v_%v_%s_%s_%s_%s_%s_%s", p.NodeName, p.Addresses, p.Port, p.Protocol, p.ServiceName, p.PodName, p.IngressName, p.Hostname, p.Path)
}

func getNodeNameToMachine(clusterName string, machineLister managementv3.NodeLister, nodeLister v1.NodeLister) (map[string]*managementv3.Node, error) {
	machines, err := machineLister.List(clusterName, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	machineMap := map[string]*managementv3.Node{}
	nodes, err := nodeLister.List("", labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		for _, node := range nodes {
			if nodehelper.IsNodeForNode(node, machine) {
				machineMap[node.Name] = machine
			}
		}
	}
	return machineMap, nil
}

func convertIngressToServicePublicEndpointsMap(obj *extensionsv1beta1.Ingress, allNodes bool) (map[string][]v3.PublicEndpoint, error) {
	var addresses []string
	epsMap := map[string][]v3.PublicEndpoint{}
	if !allNodes {
		for _, address := range obj.Status.LoadBalancer.Ingress {
			addresses = append(addresses, address.IP)
		}
		if len(addresses) == 0 {
			return epsMap, nil
		}
	}

	ports := map[int32]string{80: "HTTP", 443: "HTTPS"}
	for _, rule := range obj.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			for port, proto := range ports {
				p := v3.PublicEndpoint{
					Hostname:    rule.Host,
					Path:        path.Path,
					ServiceName: fmt.Sprintf("%s:%s", obj.Namespace, path.Backend.ServiceName),
					Addresses:   addresses,
					Port:        port,
					Protocol:    proto,
					AllNodes:    allNodes,
					IngressName: fmt.Sprintf("%s:%s", obj.Namespace, obj.Name),
				}
				v := epsMap[path.Backend.ServiceName]
				epsMap[path.Backend.ServiceName] = append(v, p)
			}
		}
	}
	return epsMap, nil
}

func convertIngressToPublicEndpoints(obj *extensionsv1beta1.Ingress, isRKE bool) ([]v3.PublicEndpoint, error) {
	epsMap, err := convertIngressToServicePublicEndpointsMap(obj, isRKE)
	if err != nil {
		return nil, err
	}
	var eps []v3.PublicEndpoint
	for _, v := range epsMap {
		eps = append(eps, v...)
	}
	return eps, nil
}
