package nodes

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NodeGroupVersionResource is the required Group Version Resource for accessing nodes in a cluster,
// using the dynamic client.
var NodeGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "nodes",
}

// GetNodes returns nodes with metav1.TypeMeta, metav1.ObjectMeta, NodeSpec, and NodeStatus to be used to gather more information from nodes
func GetNodes(client *rancher.Client, clusterID string, listOpts metav1.ListOptions) ([]corev1.Node, error) {
	var nodesList []corev1.Node

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	nodeResource := dynamicClient.Resource(NodeGroupVersionResource)
	nodes, err := nodeResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredNode := range nodes.Items {
		newNode := &corev1.Node{}
		err := scheme.Scheme.Convert(&unstructuredNode, newNode, unstructuredNode.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		nodesList = append(nodesList, *newNode)
	}

	return nodesList, err
}