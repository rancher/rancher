package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	workloadUtil "github.com/rancher/rancher/pkg/controllers/managementagent/workload"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	managementv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	allEndpoints        = "_all_endpoints_"
	endpointsAnnotation = "field.cattle.io/publicEndpoints"
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

	s := &ServicesController{
		services:           workload.Core.Services(""),
		workloadController: workloadUtil.NewWorkloadController(ctx, workload.UserOnlyContext(), nil),
		machinesLister:     workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
		clusterName:        workload.ClusterName,
	}
	workload.Core.Services("").AddHandler(ctx, "servicesEndpointsController", s.sync)

	p := &PodsController{
		podLister:          workload.Core.Pods("").Controller().Lister(),
		workloadController: workloadUtil.NewWorkloadController(ctx, workload.UserOnlyContext(), nil),
	}
	workload.Core.Pods("").AddHandler(ctx, "hostPortEndpointsController", p.sync)

	w := &WorkloadEndpointsController{
		ingressLister:  workload.Extensions.Ingresses("").Controller().Lister(),
		serviceLister:  workload.Core.Services("").Controller().Lister(),
		podLister:      workload.Core.Pods("").Controller().Lister(),
		machinesLister: workload.Management.Management.Nodes(workload.ClusterName).Controller().Lister(),
		nodeLister:     workload.Core.Nodes("").Controller().Lister(),
		clusterName:    workload.ClusterName,
		isRKE:          isRKE,
		isInternal:     cluster.Spec.Internal,
	}
	w.WorkloadController = workloadUtil.NewWorkloadController(ctx, workload.UserOnlyContext(), w.UpdateEndpoints)

	i := &IngressEndpointsController{
		workloadController: workloadUtil.NewWorkloadController(ctx, workload.UserOnlyContext(), nil),
		ingressInterface:   workload.Extensions.Ingresses(""),
		isRKE:              isRKE,
	}
	workload.Extensions.Ingresses("").AddHandler(ctx, "ingressEndpointsController", i.sync)
}

func areEqualEndpoints(one []v32.PublicEndpoint, two []v32.PublicEndpoint) bool {
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

func publicEndpointsToString(eps []v32.PublicEndpoint) (string, error) {
	b, err := json.Marshal(eps)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func getPublicEndpointsFromAnnotations(annotations map[string]string) []v32.PublicEndpoint {
	var eps []v32.PublicEndpoint
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

func convertServiceToPublicEndpoints(svc *corev1.Service, clusterName string, node *managementv3.Node, allNodesIP string) ([]v32.PublicEndpoint, error) {
	var eps []v32.PublicEndpoint
	if svc.DeletionTimestamp != nil {
		return eps, nil
	}
	if !(svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer") {
		return eps, nil
	}
	address := ""
	nodeName := ""
	if node != nil {
		address = nodehelper.GetEndpointNodeIP(node)
		nodeName = fmt.Sprintf("%s:%s", clusterName, node.Name)
	}

	svcName := fmt.Sprintf("%s:%s", svc.Namespace, svc.Name)
	if svc.Spec.Type == "NodePort" {
		var addresses []string
		if allNodesIP != "" {
			addresses = append(addresses, allNodesIP)
		}
		for _, port := range svc.Spec.Ports {
			if port.NodePort == 0 {
				continue
			}
			p := v32.PublicEndpoint{
				NodeName:    nodeName,
				Port:        port.NodePort,
				Addresses:   addresses,
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
			address = ingressEp.Hostname
			if address == "" {
				address = ingressEp.IP
			}
			addresses = append(addresses, address)
		}
		if len(addresses) > 0 {
			for _, port := range svc.Spec.Ports {
				p := v32.PublicEndpoint{
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

func convertHostPortToEndpoint(pod *corev1.Pod, clusterName string, node *managementv3.Node) ([]v32.PublicEndpoint, error) {
	var eps []v32.PublicEndpoint
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
			var address string
			if p.HostIP != "" {
				address = p.HostIP
			} else {
				address = nodehelper.GetEndpointNodeIP(node)
			}
			p := v32.PublicEndpoint{
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

func publicEndpointToString(p v32.PublicEndpoint) string {
	sort.Strings(p.Addresses)
	return fmt.Sprintf("%s_%v_%v_%s_%s_%s_%s_%s_%s", p.NodeName, p.Addresses, p.Port, p.Protocol, p.ServiceName, p.PodName, p.IngressName, p.Hostname, p.Path)
}

func getNodeNameToMachine(clusterName string, machineLister managementv3.NodeLister, nodeLister v1.NodeLister) (map[string]*managementv3.Node, error) {
	machines, err := machineLister.List(clusterName, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	machineMap := map[string]*managementv3.Node{}
	for _, machine := range machines {
		var node *corev1.Node
		node, err = nodehelper.GetNodeForMachine(machine, nodeLister)
		if err != nil {
			return nil, err
		}
		if node != nil {
			machineMap[node.Name] = machine
		}
	}
	return machineMap, nil
}

func isMachineReady(machine *managementv3.Node) bool {
	for _, cond := range machine.Status.InternalNodeStatus.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func getAllNodesPublicEndpointIP(machineLister managementv3.NodeLister, clusterName string) (string, error) {
	var addresses []string
	machines, err := machineLister.List(clusterName, labels.NewSelector())
	if err != nil {
		return "", err
	}
	for _, machine := range machines {
		if machine.Spec.Worker && nodehelper.IsMachineReady(machine) {
			nodePublicIP := getEndpointNodeAddress(machine)
			if nodePublicIP != "" {
				addresses = append(addresses, nodePublicIP)
			}
		}
	}
	if len(addresses) == 0 {
		return "", nil
	}

	sort.Slice(addresses, func(i, j int) bool {
		return strings.Compare(addresses[i], addresses[j]) < 0
	})
	return addresses[0], nil
}

func convertIngressToServicePublicEndpointsMap(obj *extensionsv1beta1.Ingress, allNodes bool) map[string][]v32.PublicEndpoint {
	epsMap := map[string][]v32.PublicEndpoint{}
	if len(obj.Status.LoadBalancer.Ingress) == 0 {
		return epsMap
	}
	var addresses []string
	var ips []net.IP
	for _, address := range obj.Status.LoadBalancer.Ingress {
		addresses = append(addresses, address.IP)
		ips = append(ips, net.ParseIP(address.IP))
	}
	if allNodes {
		sort.Slice(addresses, func(i, j int) bool {
			return bytes.Compare(ips[i], ips[j]) < 0
		})
		addresses = []string{ips[0].String()}
	}

	if len(addresses) == 0 {
		return epsMap
	}

	tlsHosts := sets.NewString()
	for _, t := range obj.Spec.TLS {
		tlsHosts.Insert(t.Hosts...)
	}

	ports := map[int32]string{80: "HTTP", 443: "HTTPS"}
	ipDomain := settings.IngressIPDomain.Get()
	for _, rule := range obj.Spec.Rules {
		//If the hostname is auto-generated, the public endpoint should be shown only when the
		//hostname is done auto-generation
		if rule.Host == ipDomain {
			continue
		}
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			for port, proto := range ports {
				if port == 80 {
					if tlsHosts.Has(rule.Host) {
						continue
					}
				} else {
					if !tlsHosts.Has(rule.Host) && rule.Host != "" {
						continue
					}
				}
				p := v32.PublicEndpoint{
					Hostname:    rule.Host,
					Path:        path.Path,
					ServiceName: fmt.Sprintf("%s:%s", obj.Namespace, path.Backend.ServiceName),
					Addresses:   addresses,
					Port:        port,
					Protocol:    proto,
					AllNodes:    allNodes,
					IngressName: fmt.Sprintf("%s:%s", obj.Namespace, obj.Name),
				}
				epsMap[path.Backend.ServiceName] = append(epsMap[path.Backend.ServiceName], p)
			}
		}
	}
	return epsMap
}

func convertIngressToPublicEndpoints(obj *extensionsv1beta1.Ingress, isRKE bool) []v32.PublicEndpoint {
	epsMap := convertIngressToServicePublicEndpointsMap(obj, isRKE)
	var eps []v32.PublicEndpoint
	for _, v := range epsMap {
		eps = append(eps, v...)
	}
	return eps
}

func getEndpointNodeAddress(machine *managementv3.Node) string {
	endpointAddress := nodehelper.GetEndpointNodeIP(machine)
	if endpointAddress == "" {
		return ""
	}
	publicIP := net.ParseIP(endpointAddress)
	if publicIP != nil && publicIP.String() != "" {
		return publicIP.String()
	}
	if errs := validation.IsDNS1123Label(endpointAddress); errs != nil {
		return ""
	}
	return endpointAddress
}
