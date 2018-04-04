package node

import (
	"context"

	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func (m *Lifecycle) checkLabels(node *v3.Node) (*v3.Node, error) {
	if !hasCheckedIn(node) ||
		!isWorkerOnlyNode(node) {
		return node, nil
	}

	cluster, err := m.clusterLister.Get("", node.Namespace)
	if err != nil {
		return node, err
	}

	if cluster.Status.Driver != v3.ClusterDriverRKE || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return node, nil
	}

	if len(node.Spec.DesiredNodeAnnotations) > 0 || len(node.Spec.DesiredNodeLabels) > 0 {
		return node, nil
	}

	nodePlan, err := getNodePlan(cluster, node)
	if err != nil {
		return node, err
	}

	if nodePlan == nil {
		return node, nil
	}

	update := false

	for k, v := range nodePlan.Labels {
		if node.Status.NodeLabels[k] != v {
			update = true
			break
		}
	}

	for k, v := range nodePlan.Annotations {
		if node.Status.NodeAnnotations[k] != v {
			update = true
			break
		}
	}

	if !update {
		return node, nil
	}

	node.Spec.DesiredNodeLabels = copyMap(node.Status.NodeLabels)
	node.Spec.DesiredNodeAnnotations = copyMap(node.Status.NodeAnnotations)

	for k, v := range nodePlan.Labels {
		node.Spec.DesiredNodeLabels[k] = v
	}

	for k, v := range nodePlan.Annotations {
		node.Spec.DesiredNodeAnnotations[k] = v
	}

	return node, nil
}

func copyMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func getNodePlan(cluster *v3.Cluster, node *v3.Node) (*v3.RKEConfigNodePlan, error) {
	plan, err := librke.New().GeneratePlan(context.Background(), cluster.Status.AppliedSpec.RancherKubernetesEngineConfig)
	if err != nil {
		return nil, err
	}

	for _, nodePlan := range plan.Nodes {
		if nodePlan.Address == node.Status.NodeConfig.Address {
			return &nodePlan, nil
		}
	}

	return nil, nil
}

func hasCheckedIn(node *v3.Node) bool {
	return len(node.Status.NodeAnnotations) > 0
}

func isWorkerOnlyNode(node *v3.Node) bool {
	if node.Status.NodeConfig == nil ||
		len(node.Status.NodeConfig.Role) != 1 ||
		node.Status.NodeConfig.Role[0] != services.WorkerRole {
		return false
	}
	return true
}
