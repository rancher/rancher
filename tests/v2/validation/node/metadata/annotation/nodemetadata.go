package node

import (
	"fmt"
	"net/url"

	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	coreV1 "k8s.io/api/core/v1"
)

func getNodeSpec(nodeList *v1.SteveCollection) (*coreV1.Node, error) {
	firstNode := nodeList.Data[0]
	nodeSpec := &coreV1.Node{}
	err := v1.ConvertToK8sType(firstNode, nodeSpec)
	if err != nil {
		return nil, err
	}
	return nodeSpec, nil
}

func verifyAnnotation(query url.Values, steveclient *v1.Client, present bool, key string, val string) (bool, *v1.SteveCollection, error) {
	nodeList, err := steveclient.SteveType("node").List(query)
	if err != nil {
		return false, nodeList, err
	}
	nodeSpec, err := getNodeSpec(nodeList)
	if err != nil {
		return false, nodeList, err
	}
	value, exists := nodeSpec.ObjectMeta.Annotations[key]
	if exists != present {
		return false, nodeList, fmt.Errorf("key [%v] must exist: %v and is present: %v", key, exists, present)
	}

	if exists && value != val {
		return false, nodeList, fmt.Errorf("key %s does not have the value %s", key, val)
	}
	return true, nodeList, nil

}

func addAnnotation(nodeSpec *coreV1.Node, nodeList *v1.SteveCollection, steveclient *v1.Client, key string, value string) error {
	nodeSpec.ObjectMeta.Annotations[key] = value
	_, err := steveclient.SteveType("node").Update(&nodeList.Data[0], nodeSpec)
	if err != nil {
		return err
	}
	return nil
}

func deleteAnnotation(nodeSpec *coreV1.Node, nodeList *v1.SteveCollection, steveclient *v1.Client, key string) error {
	delete(nodeSpec.ObjectMeta.Annotations, key)
	_, err := steveclient.SteveType("node").Update(&nodeList.Data[0], nodeSpec)
	if err != nil {
		return err
	}
	return nil
}
