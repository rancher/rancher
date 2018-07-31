package utils

import (
	"reflect"

	nodeUtils "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	nodeDockerRootDir = "node.cattle.io/docker-root-dir"
)

func getDockerRoot(machineLister v3.NodeLister, clusterName string) (dockerRootMap map[string][]*v3.Node, kinds int, firstDockerRoot string, err error) {
	dockerRootMap = make(map[string][]*v3.Node)
	machines, err := machineLister.List(clusterName, labels.NewSelector())
	if err != nil {
		return nil, 0, "", err
	}

	for _, v := range machines {
		arr, ok := dockerRootMap[v.Status.DockerInfo.DockerRootDir]
		if ok {
			arr = append(arr, v)
			continue
		}
		if kinds == 0 {
			firstDockerRoot = v.Status.DockerInfo.DockerRootDir
		}
		kinds++
		dockerRootMap[v.Status.DockerInfo.DockerRootDir] = []*v3.Node{
			v,
		}
	}
	return dockerRootMap, kinds, firstDockerRoot, nil
}

func addLabelToNodes(machines []*v3.Node, nodes v1.NodeInterface, labelKey, labelValue string) error {
	for _, m := range machines {
		node, err := nodeUtils.GetNodeForMachine(m, nodes.Controller().Lister())
		if err != nil {
			return err
		}

		nodeNew := node.DeepCopy()
		nodeNew.Labels[labelKey] = labelValue

		if reflect.DeepEqual(node, nodeNew) {
			return nil
		}

		if _, err = nodes.Update(nodeNew); err != nil {
			return err
		}
	}
	return nil
}
