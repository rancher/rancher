package project

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/types/apis/core/v1"
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
	utils.SettingExecutorQuota:         utils.SettingExecutorQuotaDefault,
	utils.SettingSigningDuration:       utils.SettingSigningDurationDefault,
	utils.SettingGitCaCerts:            "",
	utils.SettingExecutorMemoryRequest: utils.SettingExecutorMemoryRequestDefault,
	utils.SettingExecutorMemoryLimit:   utils.SettingExecutorMemoryLimitDefault,
	utils.SettingExecutorCPURequest:    utils.SettingExecutorCPURequestDefault,
	utils.SettingExecutorCPULimit:      utils.SettingExecutorCPULimitDefault,
}

func Register(ctx context.Context, cluster *config.UserContext) {
	projects := cluster.Management.Management.Projects("")
	projectSyncer := &Syncer{
		systemAccountManager:      systemaccount.NewManager(cluster.Management),
		configMaps:                cluster.Core.ConfigMaps(""),
		configMapLister:           cluster.Core.ConfigMaps("").Controller().Lister(),
		sourceCodeProviderConfigs: cluster.Management.Project.SourceCodeProviderConfigs(""),
		pipelineSettings:          cluster.Management.Project.PipelineSettings(""),
	}

	projects.AddClusterScopedHandler(ctx, "pipeline-controller", cluster.ClusterName, projectSyncer.Sync)
}

type Syncer struct {
	systemAccountManager      *systemaccount.Manager
	configMaps                v1.ConfigMapInterface
	configMapLister           v1.ConfigMapLister
	sourceCodeProviderConfigs pv3.SourceCodeProviderConfigInterface
	pipelineSettings          pv3.PipelineSettingInterface
	clusterName               string
}

func (l *Syncer) Sync(key string, obj *v3.Project) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		projectID := ""
		splits := strings.Split(key, "/")
		if len(splits) == 2 {
			projectID = splits[1]
		}
		return nil, l.cleanInternalRegistryEntry(projectID)
	}

	if err := l.addSourceCodeProviderConfigs(obj); err != nil {
		return nil, err
	}
	if err := l.addPipelineSettings(obj); err != nil {
		return nil, err
	}
	return nil, l.addSystemToken(obj)
}

func (l *Syncer) addSourceCodeProviderConfigs(obj *v3.Project) error {
	supportedProviders := map[string]string{
		model.GithubType:          pclient.GithubPipelineConfigType,
		model.GitlabType:          pclient.GitlabPipelineConfigType,
		model.BitbucketCloudType:  pclient.BitbucketCloudPipelineConfigType,
		model.BitbucketServerType: pclient.BitbucketServerPipelineConfigType,
	}
	for name, pType := range supportedProviders {
		if err := l.addSourceCodeProviderConfig(name, pType, false, obj); err != nil {
			return err
		}
	}
	return nil
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

func (l *Syncer) addSystemToken(obj *v3.Project) error {
	if err := l.systemAccountManager.GetOrCreateProjectSystemAccount(ref.Ref(obj)); err != nil {
		return err
	}
	if _, err := l.systemAccountManager.GetOrCreateProjectSystemToken(obj.Name); err != nil {
		return err
	}
	return nil
}

func (l *Syncer) cleanInternalRegistryEntry(projectID string) error {
	_, projectID = ref.Parse(projectID)
	cm, err := l.configMapLister.Get(utils.PipelineNamespace, utils.ProxyConfigMapName)
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	portMap, err := utils.GetRegistryPortMapping(cm)
	if err != nil {
		return err
	}
	if _, ok := portMap[projectID]; !ok {
		return nil
	}
	delete(portMap, projectID)
	toUpdate := cm.DeepCopy()
	utils.SetRegistryPortMapping(toUpdate, portMap)
	if _, err := l.configMaps.Update(toUpdate); err != nil {
		return err
	}
	return nil
}
