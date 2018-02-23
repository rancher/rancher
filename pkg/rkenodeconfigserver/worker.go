package rkenodeconfigserver

import (
	"context"

	"github.com/rancher/rancher/pkg/clusteryaml"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	copyProcesses = []string{
		services.KubeletContainerName,
		services.KubeproxyContainerName,
	}
)

func buildRKEConfig(node v3.RKEConfigNode) (*v3.RancherKubernetesEngineConfig, error) {
	rkeConfig, err := clusteryaml.LocalConfig()
	if err != nil {
		return nil, err
	}

	node.Role = []string{services.WorkerRole}
	rkeConfig.Nodes = append(rkeConfig.Nodes, node)

	return rkeConfig, err
}

func buildPlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (*v3.RKEConfigNodePlan, error) {
	myCluster, err := cluster.ParseCluster(ctx, rkeConfig, "", "", nil, nil, nil)
	if err != nil {
		return nil, err
	}

	myCluster.WorkerHosts[0].DockerInfo.DockerRootDir = "/var/lib/docker"

	nodePlan := cluster.BuildRKEConfigNodePlan(ctx, myCluster, myCluster.WorkerHosts[0])
	return &nodePlan, nil
}

func buildProcesses(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (map[string]v3.Process, error) {
	nodePlan, err := buildPlan(ctx, rkeConfig)
	if err != nil {
		return nil, err
	}

	return filterProcesses(*nodePlan), nil
}

func buildCerts(rkeConfig *v3.RancherKubernetesEngineConfig, node *v3.RKEConfigNode, server, token string) (string, error) {
	bundle, err := rkecerts.Load()
	if err != nil {
		return "", err
	}

	nodeBundle, err := bundle.ForNode(rkeConfig, node, server, token)
	if err != nil {
		return "", err
	}
	return nodeBundle.Marshal()
}

func AgentConfig(ctx context.Context, node v3.RKEConfigNode, server, token string) (*rkeworker.NodeConfig, error) {
	rkeConfig, err := buildRKEConfig(node)
	if err != nil {
		return nil, err
	}

	processes, err := buildProcesses(ctx, rkeConfig)
	if err != nil {
		return nil, err
	}

	certs, err := buildCerts(rkeConfig, &rkeConfig.Nodes[1], server, token)
	if err != nil {
		return nil, err
	}

	return &rkeworker.NodeConfig{
		Certs:     certs,
		Processes: processes,
	}, nil
}

func filterProcesses(nodePlan v3.RKEConfigNodePlan) map[string]v3.Process {
	processes := map[string]v3.Process{}
	for _, name := range copyProcesses {
		processes[name] = nodePlan.Processes[name]
	}
	return processes
}
