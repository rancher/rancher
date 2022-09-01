package v3

import (
	projectv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
)

var (
	ToolsSystemImages = struct {
		PipelineSystemImages projectv3.PipelineSystemImages
		AuthSystemImages     AuthSystemImages
	}{
		PipelineSystemImages: projectv3.PipelineSystemImages{
			Jenkins:       "rancher/pipeline-jenkins-server:v0.1.4",
			JenkinsJnlp:   "rancher/mirrored-jenkins-jnlp-slave:4.7-1",
			AlpineGit:     "rancher/pipeline-tools:v0.1.16",
			PluginsDocker: "rancher/mirrored-plugins-docker:19.03.8",
			Minio:         "rancher/mirrored-minio-minio:RELEASE.2020-07-13T18-09-56Z",
			Registry:      "rancher/mirrored-library-registry:2.7.1",
			RegistryProxy: "rancher/pipeline-tools:v0.1.16",
			KubeApply:     "rancher/pipeline-tools:v0.1.16",
		},
		AuthSystemImages: AuthSystemImages{
			KubeAPIAuth: "rancher/kube-api-auth:v0.1.8",
		},
	}
)
