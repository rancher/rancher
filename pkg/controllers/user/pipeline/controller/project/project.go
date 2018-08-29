package project

import (
	"context"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	pclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This controller is responsible for initializing source code
// provider configs & pipeline settings for projects.

var settings = map[string]string{
	utils.SettingExecutorQuota: utils.SettingExecutorQuotaDefault,
}

func Register(ctx context.Context, cluster *config.UserContext) {
	projects := cluster.Management.Management.Projects("")
	projectSyncer := &Syncer{
		sourceCodeProviderConfigs: cluster.Management.Project.SourceCodeProviderConfigs(""),
		pipelineSettings:          cluster.Management.Project.PipelineSettings(""),
	}

	projects.AddClusterScopedHandler("pipeline-controller", cluster.ClusterName, projectSyncer.Sync)
}

type Syncer struct {
	sourceCodeProviderConfigs pv3.SourceCodeProviderConfigInterface
	pipelineSettings          pv3.PipelineSettingInterface
	clusterName               string
}

func (l *Syncer) Sync(key string, obj *v3.Project) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}

	if err := l.addSourceCodeProviderConfigs(obj); err != nil {
		return err
	}

	return l.addPipelineSettings(obj)
}

func (l *Syncer) addSourceCodeProviderConfigs(obj *v3.Project) error {
	if err := l.addSourceCodeProviderConfig(model.GithubType, pclient.GithubPipelineConfigType, false, obj); err != nil {
		return err
	}

	return l.addSourceCodeProviderConfig(model.GitlabType, pclient.GitlabPipelineConfigType, false, obj)
}

func (l *Syncer) addSourceCodeProviderConfig(name, pType string, enabled bool, obj *v3.Project) error {
	_, err := l.sourceCodeProviderConfigs.ObjectClient().Create(&pv3.SourceCodeProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: obj.Name,
		},
		ProjectName: ref.Ref(obj),
		Type:        pType,
		Enabled:     enabled,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (l *Syncer) addPipelineSettings(obj *v3.Project) error {
	for k, v := range settings {
		if err := l.addPipelineSetting(k, v, obj); err != nil {
			return err
		}
	}

	return nil
}

func (l *Syncer) addPipelineSetting(settingName string, value string, obj *v3.Project) error {
	setting := &pv3.PipelineSetting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      settingName,
			Namespace: obj.Name,
		},
		ProjectName: ref.Ref(obj),
		Default:     value,
	}

	if _, err := l.pipelineSettings.Create(setting); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
