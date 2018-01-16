package cluster

import (
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func GetLocalRKEConfig() *v3.RancherKubernetesEngineConfig {
	rkeLocalNode := GetLocalRKENodeConfig()
	rkeServices := v3.RKEConfigServices{
		Kubelet: v3.KubeletService{
			BaseService: v3.BaseService{
				Image:     DefaultK8sImage,
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
