package endpoints

import (
	"context"

	"encoding/json"
	"fmt"
	"reflect"

	workloadUtil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	allEndpoints        = "_all_endpoints_"
	endpointsAnnotation = "field.cattle.io/publicEndpoints"
)

func Register(ctx context.Context, workload *config.UserOnlyContext) {
	n := &NodesController{
		nodes:         workload.Core.Nodes(""),
		serviceLister: workload.Core.Services("").Controller().Lister(),
		nodeLister:    workload.Core.Nodes("").Controller().Lister(),
		podLister:     workload.Core.Pods("").Controller().Lister(),
	}
	workload.Core.Nodes("").AddHandler("nodesEndpointsController", n.sync)

	s := &ServicesController{
		services:           workload.Core.Services(""),
		serviceLister:      workload.Core.Services("").Controller().Lister(),
		nodeLister:         workload.Core.Nodes("").Controller().Lister(),
		nodeController:     workload.Core.Nodes("").Controller(),
		podLister:          workload.Core.Pods("").Controller().Lister(),
		podController:      workload.Core.Pods("").Controller(),
		workloadController: workloadUtil.NewWorkloadController(workload, nil),
	}
	workload.Core.Services("").AddHandler("servicesEndpointsController", s.sync)

	p := &PodsController{
		nodeLister:         workload.Core.Nodes("").Controller().Lister(),
		nodeController:     workload.Core.Nodes("").Controller(),
		pods:               workload.Core.Pods(""),
		serviceLister:      workload.Core.Services("").Controller().Lister(),
		podLister:          workload.Core.Pods("").Controller().Lister(),
		workloadController: workloadUtil.NewWorkloadController(workload, nil),
	}
	workload.Core.Pods("").AddHandler("hostPortEndpointsController", p.sync)

	w := &WorkloadEndpointsController{
		serviceLister: workload.Core.Services("").Controller().Lister(),
		podLister:     workload.Core.Pods("").Controller().Lister(),
	}
	w.WorkloadController = workloadUtil.NewWorkloadController(workload, w.UpdateEndpoints)
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

func convertServiceToPublicEndpoints(svc *corev1.Service, node *corev1.Node) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	if svc.DeletionTimestamp != nil {
		return eps, nil
	}
	if !(svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer") {
		return eps, nil
	}
	var address string
	var nodeName string

	nodePort := svc.Spec.Type == "NodePort"

	if node != nil {
		if val, ok := node.Annotations["alpha.kubernetes.io/provided-node-ip"]; ok {
			nodeIP := string(val)
			if nodeIP == "" {
				logrus.Warnf("Node [%s] has no ip address set", node.Name)
			} else {
				address = nodeIP
			}
		}
		nodeName = node.Name
	} else if nodePort {
		address = ""
	}

	svcName := fmt.Sprintf("%s:%s", svc.Namespace, svc.Name)
	if nodePort {
		for _, port := range svc.Spec.Ports {
			if port.NodePort == 0 {
				continue
			}
			p := v3.PublicEndpoint{
				NodeName:    nodeName,
				Address:     address,
				Port:        port.NodePort,
				Protocol:    string(port.Protocol),
				ServiceName: svcName,
			}
			eps = append(eps, p)
		}
	} else {
		for _, port := range svc.Spec.Ports {
			for _, ingressEp := range svc.Status.LoadBalancer.Ingress {
				address := ingressEp.Hostname
				if address == "" {
					address = ingressEp.IP
				}
				p := v3.PublicEndpoint{
					NodeName:    "",
					Address:     address,
					Port:        port.Port,
					Protocol:    string(port.Protocol),
					ServiceName: svcName,
				}
				eps = append(eps, p)
			}
		}
	}

	return eps, nil
}

func convertHostPortToEndpoint(pod *corev1.Pod) ([]v3.PublicEndpoint, error) {
	var eps []v3.PublicEndpoint
	if pod.DeletionTimestamp != nil {
		return eps, nil
	}
	if pod.Status.Phase != corev1.PodRunning {
		return eps, nil
	}
	nodeName := pod.Spec.NodeName

	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort == 0 {
				continue
			}
			address := p.HostIP
			if address == "" {
				address = pod.Status.HostIP
			}
			p := v3.PublicEndpoint{
				NodeName: nodeName,
				Address:  address,
				Port:     p.HostPort,
				Protocol: string(p.Protocol),
				PodName:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
			}
			eps = append(eps, p)
		}
	}

	return eps, nil
}

func publicEndpointToString(p v3.PublicEndpoint) string {
	return fmt.Sprintf("%s_%s_%v_%s_%s_%s", p.NodeName, p.Address, p.Port, p.Protocol, p.ServiceName, p.PodName)
}
