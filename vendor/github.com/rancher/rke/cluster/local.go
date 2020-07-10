package cluster

import (
	"github.com/rancher/rke/metadata"
	"github.com/rancher/rke/services"
	v3 "github.com/rancher/rke/types"
)

func GetLocalRKEConfig() *v3.RancherKubernetesEngineConfig {
	rkeLocalNode := GetLocalRKENodeConfig()
	imageDefaults := metadata.K8sVersionToRKESystemImages[metadata.DefaultK8sVersion]

	rkeServices := v3.RKEConfigServices{
		Kubelet: v3.KubeletService{
			BaseService: v3.BaseService{
				Image:     imageDefaults.Kubernetes,
				ExtraArgs: map[string]string{"fail-swap-on": "false"},
			},
		},
	}
	return &v3.RancherKubernetesEngineConfig{
		Nodes:    []v3.RKEConfigNode{*rkeLocalNode},
		Services: rkeServices,
	}

}

func GetLocalRKENodeConfig() *v3.RKEConfigNode {
	rkeLocalNode := &v3.RKEConfigNode{
		Address:          LocalNodeAddress,
		HostnameOverride: LocalNodeHostname,
		User:             LocalNodeUser,
		Role:             []string{services.ControlRole, services.WorkerRole, services.ETCDRole},
	}
	return rkeLocalNode
}
