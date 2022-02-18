package secretmigrator

import (
	"encoding/json"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apiprjv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

// AssemblePrivateRegistryCredential looks up the registry Secret and inserts the keys into the PrivateRegistries list on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssemblePrivateRegistryCredential(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if cluster.Spec.RancherKubernetesEngineConfig == nil || len(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries) == 0 {
		return spec, nil
	}
	if cluster.Status.PrivateRegistrySecret == "" {
		if cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].Password != "" {
			logrus.Warnf("[secretmigrator] secrets for cluster %s are not finished migrating", cluster.Name)
		}
		return spec, nil

	}
	registrySecret, err := secretLister.Get(secretNamespace, cluster.Status.PrivateRegistrySecret)
	if err != nil {
		return spec, err
	}
	dockerCfg := credentialprovider.DockerConfigJSON{}
	err = json.Unmarshal(registrySecret.Data[".dockerconfigjson"], &dockerCfg)
	if err != nil {
		return spec, err
	}
	for i, privateRegistry := range cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries {
		if reg, ok := dockerCfg.Auths[privateRegistry.URL]; ok {
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].User = reg.Username
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].Password = reg.Password
		}
	}
	return spec, nil
}

// AssembleS3Credential looks up the S3 backup config Secret and inserts the keys into the S3BackupConfig on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleS3Credential(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if cluster.Spec.RancherKubernetesEngineConfig == nil || cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig == nil {
		return spec, nil
	}
	if cluster.Status.S3CredentialSecret == "" {
		if cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey != "" {
			logrus.Warnf("[secretmigrator] secrets for cluster %s are not finished migrating", cluster.Name)
		}
		return spec, nil
	}
	s3Cred, err := secretLister.Get(namespace.GlobalNamespace, cluster.Status.S3CredentialSecret)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = string(s3Cred.Data["secretKey"])
	return spec, nil
}

// AssembleWeaveCredential looks up the weave Secret and inserts the keys into the network provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleWeaveCredential(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if cluster.Spec.RancherKubernetesEngineConfig == nil || cluster.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider == nil {
		return spec, nil
	}
	if cluster.Status.WeavePasswordSecret == "" {
		if cluster.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for cluster %s are not finished migrating", cluster.Name)
		}
		return spec, nil

	}
	registrySecret, err := secretLister.Get(secretNamespace, cluster.Status.WeavePasswordSecret)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = string(registrySecret.Data["password"])
	return spec, nil
}

// AssembleSMTPCredential looks up the SMTP Secret and inserts the keys into the Notifier.
// It returns a new copy of the Notifier without modifying the original. The Notifier is never updated.
func AssembleSMTPCredential(notifier *apimgmtv3.Notifier, secretLister v1.SecretLister) (*apimgmtv3.NotifierSpec, error) {
	if notifier.Spec.SMTPConfig == nil {
		return &notifier.Spec, nil
	}
	if notifier.Status.SMTPCredentialSecret == "" {
		if notifier.Spec.SMTPConfig.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for notifier %s are not finished migrating", notifier.Name)
		}
		return &notifier.Spec, nil
	}
	smtpSecret, err := secretLister.Get(namespace.GlobalNamespace, notifier.Status.SMTPCredentialSecret)
	if err != nil {
		return &notifier.Spec, err
	}
	spec := notifier.Spec.DeepCopy()
	spec.SMTPConfig.Password = string(smtpSecret.Data["password"])
	return spec, nil
}

// AssembleWechatCredential looks up the Wechat Secret and inserts the keys into the Notifier.
// It returns a new copy of the Notifier without modifying the original. The Notifier is never updated.
func AssembleWechatCredential(notifier *apimgmtv3.Notifier, secretLister v1.SecretLister) (*apimgmtv3.NotifierSpec, error) {
	if notifier.Spec.WechatConfig == nil {
		return &notifier.Spec, nil
	}
	if notifier.Status.WechatCredentialSecret == "" {
		if notifier.Spec.WechatConfig.Secret != "" {
			logrus.Warnf("[secretmigrator] secrets for notifier %s are not finished migrating", notifier.Name)
		}
		return &notifier.Spec, nil
	}
	wechatSecret, err := secretLister.Get(namespace.GlobalNamespace, notifier.Status.WechatCredentialSecret)
	if err != nil {
		return &notifier.Spec, err
	}
	spec := notifier.Spec.DeepCopy()
	spec.WechatConfig.Secret = string(wechatSecret.Data["secret"])
	return spec, nil
}

// AssembleDingtalkCredential looks up the Dingtalk Secret and inserts the keys into the Notifier.
// It returns a new copy of the Notifier without modifying the original. The Notifier is never updated.
func AssembleDingtalkCredential(notifier *apimgmtv3.Notifier, secretLister v1.SecretLister) (*apimgmtv3.NotifierSpec, error) {
	if notifier.Spec.DingtalkConfig == nil {
		return &notifier.Spec, nil
	}
	if notifier.Status.DingtalkCredentialSecret == "" {
		if notifier.Spec.DingtalkConfig.Secret != "" {
			logrus.Warnf("[secretmigrator] secrets for notifier %s are not finished migrating", notifier.Name)
		}
		return &notifier.Spec, nil
	}
	secret, err := secretLister.Get(namespace.GlobalNamespace, notifier.Status.DingtalkCredentialSecret)
	if err != nil {
		return &notifier.Spec, err
	}
	spec := notifier.Spec.DeepCopy()
	spec.DingtalkConfig.Secret = string(secret.Data["credential"])
	return spec, nil
}

// AssembleGithubPipelineConfigCredential looks up the github pipeline client secret and inserts it into the config.
// It returns a new copy of the GithubPipelineConfig without modifying the original. The config is never updated.
func (m *Migrator) AssembleGithubPipelineConfigCredential(config apiprjv3.GithubPipelineConfig) (apiprjv3.GithubPipelineConfig, error) {
	if config.CredentialSecret == "" {
		if config.ClientSecret != "" {
			logrus.Warnf("[secretmigrator] secrets for %s pipeline config in project %s are not finished migrating", model.GithubType, config.ProjectName)
		}
		return config, nil
	}
	secret, err := m.secretLister.Get(namespace.GlobalNamespace, config.CredentialSecret)
	if err != nil {
		return config, err
	}
	config.ClientSecret = string(secret.Data["credential"])
	return config, nil
}

// AssembleGitlabPipelineConfigCredential looks up the gitlab pipeline client secret and inserts it into the config.
// It returns a new copy of the GitlabPipelineConfig without modifying the original. The config is never updated.
func (m *Migrator) AssembleGitlabPipelineConfigCredential(config apiprjv3.GitlabPipelineConfig) (apiprjv3.GitlabPipelineConfig, error) {
	if config.CredentialSecret == "" {
		if config.ClientSecret != "" {
			logrus.Warnf("[secretmigrator] secrets for %s pipeline config in project %s are not finished migrating", model.GitlabType, config.ProjectName)
		}
		return config, nil
	}
	secret, err := m.secretLister.Get(namespace.GlobalNamespace, config.CredentialSecret)
	if err != nil {
		return config, err
	}
	config.ClientSecret = string(secret.Data["credential"])
	return config, nil
}

// AssembleBitbucketCloudPipelineConfigCredential looks up the bitbucket cloud pipeline client secret and inserts it into the config.
// It returns a new copy of the BitbucketCloudPipelineConfig without modifying the original. The config is never updated.
func (m *Migrator) AssembleBitbucketCloudPipelineConfigCredential(config apiprjv3.BitbucketCloudPipelineConfig) (apiprjv3.BitbucketCloudPipelineConfig, error) {
	if config.CredentialSecret == "" {
		if config.ClientSecret != "" {
			logrus.Warnf("[secretmigrator] secrets for %s pipeline config in project %s are not finished migrating", model.BitbucketCloudType, config.ProjectName)
		}
		return config, nil
	}
	secret, err := m.secretLister.Get(namespace.GlobalNamespace, config.CredentialSecret)
	if err != nil {
		return config, err
	}
	config.ClientSecret = string(secret.Data["credential"])
	return config, nil
}

// AssembleBitbucketServerPipelineConfigCredential looks up the bitbucket server pipeline client secret and inserts it into the config.
// It returns a new copy of the BitbucketServerPipelineConfig without modifying the original. The config is never updated.
func (m *Migrator) AssembleBitbucketServerPipelineConfigCredential(config apiprjv3.BitbucketServerPipelineConfig) (apiprjv3.BitbucketServerPipelineConfig, error) {
	if config.CredentialSecret == "" {
		if config.PrivateKey != "" {
			logrus.Warnf("[secretmigrator] secrets for %s pipeline config in project %s are not finished migrating", model.BitbucketServerType, config.ProjectName)
		}
		return config, nil
	}
	secret, err := m.secretLister.Get(namespace.GlobalNamespace, config.CredentialSecret)
	if err != nil {
		return config, err
	}
	config.PrivateKey = string(secret.Data["credential"])
	return config, nil
}
