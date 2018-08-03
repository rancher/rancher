package embedded

import (
	"io/ioutil"

	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"gopkg.in/yaml.v2"
)

const (
	localCluster = "/etc/rancher/cluster.yml"
)

func localConfig() (*v3.RancherKubernetesEngineConfig, error) {
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

	if rkeConfig.Services.KubeAPI.ExtraArgs == nil {
		rkeConfig.Services.KubeAPI.ExtraArgs = map[string]string{}
	}

	rkeConfig.Services.KubeAPI.ExtraArgs["advertise-address"] = "10.43.0.1"
	rkeConfig.Services.KubeAPI.ExtraArgs["bind-address"] = "127.0.0.1"
	rkeConfig.Services.KubeAPI.ExtraArgs["requestheader-client-ca-file"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["requestheader-allowed-names"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["proxy-client-key-file"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["proxy-client-cert-file"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["requestheader-extra-headers-prefix"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["requestheader-group-headers"] = ""
	rkeConfig.Services.KubeAPI.ExtraArgs["requestheader-username-headers"] = ""
	rkeConfig.SystemImages = v3.AllK8sVersions["v1.10.5-rancher1-1"]
	rkeConfig.Version = settings.KubernetesVersion.Get()
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

	c, err := librke.New().ParseCluster("local", rkeConfig,
		nil,
		nil,
		nil)
	if err != nil {
		return nil, err
	}

	return &c.RancherKubernetesEngineConfig, nil
}
