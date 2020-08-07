package v3

import (
	projectv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/rke/types/image"
)

var (
	m = image.Mirror

	ToolsSystemImages = struct {
		PipelineSystemImages projectv3.PipelineSystemImages
		AuthSystemImages     AuthSystemImages
	}{
		PipelineSystemImages: projectv3.PipelineSystemImages{
			Jenkins:       m("rancher/pipeline-jenkins-server:v0.1.4"),
			JenkinsJnlp:   m("jenkins/jnlp-slave:3.35-4"),
			AlpineGit:     m("rancher/pipeline-tools:v0.1.15"),
			PluginsDocker: m("plugins/docker:18.09"),
			Minio:         m("minio/minio:RELEASE.2020-07-13T18-09-56Z"),
			Registry:      m("registry:2"),
			RegistryProxy: m("rancher/pipeline-tools:v0.1.15"),
			KubeApply:     m("rancher/pipeline-tools:v0.1.15"),
		},
		AuthSystemImages: AuthSystemImages{
			KubeAPIAuth: m("rancher/kube-api-auth:v0.1.4"),
		},
	}
)
