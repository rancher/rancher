package clusteryaml

import (
	"io/ioutil"

	"context"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"gopkg.in/yaml.v2"
)

const (
	localCluster = "/etc/rancher/cluster.yml"
)

func LocalConfig() (*v3.RancherKubernetesEngineConfig, error) {
	rkeConfig := &v3.RancherKubernetesEngineConfig{}

	content, err := ioutil.ReadFile(localCluster)
	if err == nil {
		if err := yaml.Unmarshal(content, rkeConfig); err != nil {
			return nil, err
		}
	}

	roles := []string{
		services.ControlRole,
	}

	if len(rkeConfig.Services.Etcd.ExternalURLs) == 0 {
		roles = append(roles, services.ETCDRole)
	}

	rkeConfig.SystemImages = v3.K8sVersionToRKESystemImages[v3.K8sV1_8]
	rkeConfig.SystemImages.Kubernetes = settings.AgentImage.Get()
	rkeConfig.IgnoreDockerVersion = true
	rkeConfig.Nodes = []v3.RKEConfigNode{
		{
			HostnameOverride: "master",
			Address:          "127.0.0.1",
			User:             "root",
			Role:             roles,
			DockerSocket:     "/var/run/docker.sock",
		},
	}

	c, err := cluster.ParseCluster(context.Background(),
		rkeConfig,
		"",
		"",
		nil,
		nil,
		nil)
	if err != nil {
		return nil, err
	}

	return &c.RancherKubernetesEngineConfig, nil
}
