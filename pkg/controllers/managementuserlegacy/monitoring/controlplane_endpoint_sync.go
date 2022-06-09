package monitoring

import (
	"reflect"
	"sort"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/node"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type ExporterEndpointController struct {
	Endpoints           v1.EndpointsInterface
	EndpointLister      v1.EndpointsLister
	EndpointsController v1.EndpointsController
	NodeLister          v1.NodeLister
	ServiceLister       v1.ServiceLister
}

const (
	nodeMetricsWindowsEndpointName = "expose-node-metrics-windows"
)

var (
	etcdLabel         = labels.Set(map[string]string{"node-role.kubernetes.io/etcd": "true"}).AsSelector()
	controlPlaneLabel = labels.Set(map[string]string{"node-role.kubernetes.io/controlplane": "true"}).AsSelector()
	masterLabel       = labels.Set(map[string]string{"node-role.kubernetes.io/master": "true"}).AsSelector()
	windowNodeLabel   = labels.Set(map[string]string{"kubernetes.io/os": "windows"}).AsSelector()
	selectorMap       = map[string][]labels.Selector{
		"expose-kube-etcd-metrics":      {etcdLabel, masterLabel},
		"expose-kube-cm-metrics":        {controlPlaneLabel, masterLabel},
		"expose-kube-scheduler-metrics": {controlPlaneLabel, masterLabel},
		nodeMetricsWindowsEndpointName:  {windowNodeLabel},
	}
)

func (controller *ExporterEndpointController) sync(key string, obj *corev1.Node) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		for key := range selectorMap {
			controller.EndpointsController.Enqueue("cattle-prometheus", key)
		}
		return obj, nil
	}

	if obj.Labels == nil {
		return obj, nil
	}

	endpointMap := make(map[string]struct{})
	for endpointNames, selectors := range selectorMap {
		for _, selector := range selectors {
			if selector.Matches(labels.Set(obj.Labels)) {
				endpointMap[endpointNames] = struct{}{}
				break
			}
		}
	}

	endpoints, err := getTargetEndpoints(controller.EndpointLister, endpointMap)
	if err != nil {
		return obj, err
	}
	for _, endpoint := range endpoints {
		controller.EndpointsController.Enqueue(endpoint.Namespace, endpoint.Name)
	}

	return obj, nil
}

func (controller *ExporterEndpointController) syncEndpoints(key string, obj *corev1.Endpoints) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}
	selectors, ok := selectorMap[obj.Name]
	if !ok {
		return obj, nil
	}

	nodes, err := getNodes(controller.NodeLister, selectors)
	if err != nil {
		return nil, err
	}

	svc, err := controller.ServiceLister.Get(obj.Namespace, obj.Name)
	if err != nil {
		return obj, err
	}

	newObj := obj.DeepCopy()
	injectAddressToEndpoint(svc, newObj, nodes)
	if !reflect.DeepEqual(obj, newObj) {
		return controller.Endpoints.Update(newObj)
	}

	return obj, nil
}

func injectAddressToEndpoint(svc *corev1.Service, endpoint *corev1.Endpoints, nodes []*corev1.Node) {
	subsetIndex := -1
	var ports []corev1.EndpointPort
	for _, servicePort := range svc.Spec.Ports {
		ports = append(ports, getEndpointPort(servicePort))
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})

	for i, subset := range endpoint.Subsets {
		var subsetPorts []corev1.EndpointPort
		copy(subsetPorts, subset.Ports)
		sort.Slice(subsetPorts, func(i, j int) bool {
			return subsetPorts[i].Port < subsetPorts[j].Port
		})
		if reflect.DeepEqual(subsetPorts, ports) {
			subsetIndex = i
			break
		}
	}

	if subsetIndex == -1 {
		endpoint.Subsets = []corev1.EndpointSubset{{
			Ports: ports,
		}}
		subsetIndex = 0
	}

	endpoint.Subsets[subsetIndex].Addresses = []corev1.EndpointAddress{}

	for _, n := range nodes {
		address := node.GetNodeInternalAddress(n)
		endpoint.Subsets[subsetIndex].Addresses = append(endpoint.Subsets[subsetIndex].Addresses,
			corev1.EndpointAddress{
				IP:       address,
				NodeName: &n.Name,
				TargetRef: &corev1.ObjectReference{
					Kind: "Node",
					Name: n.Name,
					UID:  n.UID,
				},
			},
		)
	}
}

func getEndpointPort(servicePort corev1.ServicePort) corev1.EndpointPort {
	portName := servicePort.Name
	portProto := servicePort.Protocol
	portNum := servicePort.TargetPort.IntValue()
	return corev1.EndpointPort{Name: portName, Port: int32(portNum), Protocol: portProto}
}

func getTargetEndpoints(client v1.EndpointsLister, endpointMap map[string]struct{}) ([]*corev1.Endpoints, error) {
	var rtn []*corev1.Endpoints
	for name := range endpointMap {
		endpoint, err := client.Get("cattle-prometheus", name)
		if err != nil && !apierrors.IsNotFound(err) {
			return rtn, err
		}
		if endpoint != nil {
			rtn = append(rtn, endpoint)
		}
	}
	return rtn, nil
}

func getNodes(client v1.NodeLister, selectors []labels.Selector) ([]*corev1.Node, error) {
	var rtn []*corev1.Node
	nodes, err := client.List("", labels.NewSelector())
	if err != nil {
		return rtn, err
	}
	for _, n := range nodes {
		for _, selector := range selectors {
			if selector.Matches(labels.Set(n.Labels)) {
				rtn = append(rtn, n)
				break
			}
		}
	}
	return rtn, nil
}

func endpointHasAddress(endpoint *corev1.Endpoints, addr string) bool {
	for _, addressSet := range endpoint.Subsets {
		for _, address := range addressSet.Addresses {
			if address.IP == addr && address.NodeName != nil {
				return true
			}
		}
	}
	return false
}
