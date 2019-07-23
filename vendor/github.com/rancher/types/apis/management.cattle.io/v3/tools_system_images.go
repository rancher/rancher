package v3

import (
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/image"
)

var (
	m = image.Mirror

	ToolsSystemImages = struct {
		AlertSystemImages    AlertSystemImages
		PipelineSystemImages projectv3.PipelineSystemImages
		LoggingSystemImages  LoggingSystemImages
		AuthSystemImages     AuthSystemImages
	}{
		AlertSystemImages: AlertSystemImages{
			AlertManager:       m("prom/alertmanager:v0.15.2"),
			AlertManagerHelper: m("rancher/alertmanager-helper:v0.0.2"),
		},
		PipelineSystemImages: projectv3.PipelineSystemImages{
			Jenkins:       m("rancher/pipeline-jenkins-server:v0.1.0"),
			JenkinsJnlp:   m("jenkins/jnlp-slave:3.10-1-alpine"),
			AlpineGit:     m("rancher/pipeline-tools:v0.1.10"),
			PluginsDocker: m("plugins/docker:17.12"),
			Minio:         m("minio/minio:RELEASE.2018-05-25T19-49-13Z"),
			Registry:      m("registry:2"),
			RegistryProxy: m("rancher/pipeline-tools:v0.1.10"),
			KubeApply:     m("rancher/pipeline-tools:v0.1.10"),
		},
		LoggingSystemImages: LoggingSystemImages{
			Fluentd:                       m("rancher/fluentd:v0.1.11"),
			FluentdHelper:                 m("rancher/fluentd-helper:v0.1.2"),
			LogAggregatorFlexVolumeDriver: m("rancher/log-aggregator:v0.1.4"),
		},
		AuthSystemImages: AuthSystemImages{
			KubeAPIAuth: m("rancher/kube-api-auth:v0.1.3"),
		},
	}
)
