package endpoints

import (
	"github.com/rancher/types/apis/core/v1"
	managementv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This controller is responsible for monitoring nodes
// and setting public endpoints on them based on HostPort pods
// and NodePort/LoadBalancer services

type NodesController struct {
	nodes          v1.NodeInterface
	nodeLister     v1.NodeLister
	serviceLister  v1.ServiceLister
	podLister      v1.PodLister
	machinesLister managementv3.NodeLister
	clusterName    string
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

	for _, node := range nodesToSync {
		if node.DeletionTimestamp != nil {
			continue
		}
		_, err := n.reconcileEndpontsForNode(&node)
		if err != nil {
			return err
		}
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
	nodeNameToMachine, err := getNodeNameToMachine(n.clusterName, n.machinesLister, n.nodeLister)
	if err != nil {
		return false, err
	}
	for _, svc := range svcs {
		if svc.DeletionTimestamp != nil {
			continue
		}
		pEps, err := convertServiceToPublicEndpoints(svc, n.clusterName, nodeNameToMachine[node.Name])
		if err != nil {
			return false, err
		}
		newPublicEps = append(newPublicEps, pEps...)
	}

	// Get endpoint from hostPort pods
	pods, err := n.podLister.List("", labels.NewSelector())
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil || pod.Spec.NodeName != node.Name {
			continue
		}

		pEps, err := convertHostPortToEndpoint(pod, n.clusterName, nodeNameToMachine[node.Name])
		if err != nil {
			return false, err
		}
		newPublicEps = append(newPublicEps, pEps...)
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
