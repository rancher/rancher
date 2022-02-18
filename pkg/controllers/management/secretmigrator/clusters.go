package secretmigrator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/mitchellh/mapstructure"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apiprjv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	pipelineutils "github.com/rancher/rancher/pkg/pipeline/utils"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	secretNamespace             = namespace.GlobalNamespace
	S3BackupAnswersPath         = "rancherKubernetesEngineConfig.services.etcd.backupConfig.s3BackupConfig.secretKey"
	WeavePasswordAnswersPath    = "rancherKubernetesEngineConfig.network.weaveNetworkProvider.password"
	RegistryPasswordAnswersPath = "rancherKubernetesEngineConfig.privateRegistries[%d].password"
)

var PrivateRegistryQuestion = regexp.MustCompile("rancherKubernetesEngineConfig.privateRegistries[[0-9]+].password")

func (h *handler) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}
	if apimgmtv3.ClusterConditionSecretsMigrated.IsTrue(cluster) {
		logrus.Tracef("[secretmigrator] cluster %s already migrated", cluster.Name)
		return cluster, nil
	}
	obj, err := apimgmtv3.ClusterConditionSecretsMigrated.Do(cluster, func() (runtime.Object, error) {
		// privateRegistries
		if cluster.Status.PrivateRegistrySecret == "" {
			logrus.Tracef("[secretmigrator] migrating private registry secrets for cluster %s", cluster.Name)
			regSecret, err := h.migrator.CreateOrUpdatePrivateRegistrySecret(cluster.Status.PrivateRegistrySecret, cluster.Spec.RancherKubernetesEngineConfig, cluster)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", cluster.Name, err)
				return nil, err
			}
			if regSecret != nil {
				logrus.Tracef("[secretmigrator] private registry secret found for cluster %s", cluster.Name)
				cluster.Status.PrivateRegistrySecret = regSecret.Name
				cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries)
				if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}
				if cluster.Status.FailedSpec != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					cluster.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(cluster.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}
				clusterCopy, err := h.clusters.Update(cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, regSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				cluster = clusterCopy
			}
		}

		// s3 backup cred
		if cluster.Status.S3CredentialSecret == "" {
			logrus.Tracef("[secretmigrator] migrating S3 secrets for cluster %s", cluster.Name)
			s3Secret, err := h.migrator.CreateOrUpdateS3Secret("", cluster.Spec.RancherKubernetesEngineConfig, cluster)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for cluster %s, will retry: %v", cluster.Name, err)
				return nil, err
			}
			if s3Secret != nil {
				logrus.Tracef("[secretmigrator] S3 secret found for cluster %s", cluster.Name)
				cluster.Status.S3CredentialSecret = s3Secret.Name
				cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig != nil && cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
					cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				}
				if cluster.Status.FailedSpec != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
					cluster.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				}
				clusterCopy, err := h.clusters.Update(cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, s3Secret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				cluster = clusterCopy
			}
		}

		// weave CNI password
		if cluster.Status.WeavePasswordSecret == "" {
			logrus.Tracef("[secretmigrator] migrating weave CNI secrets for cluster %s", cluster.Name)
			weaveSecret, err := h.migrator.CreateOrUpdateWeaveSecret("", cluster.Spec.RancherKubernetesEngineConfig, cluster)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for cluster %s, will retry: %v", cluster.Name, err)
				return nil, err
			}
			if weaveSecret != nil {
				logrus.Tracef("[secretmigrator] weave secret found for cluster %s", cluster.Name)
				cluster.Status.WeavePasswordSecret = weaveSecret.Name
				cluster.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider != nil {
					cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				}
				if cluster.Status.FailedSpec != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig != nil && cluster.Status.FailedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider != nil {
					cluster.Status.FailedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				}
				clusterCopy, err := h.clusters.Update(cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, weaveSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				cluster = clusterCopy
			}
		}

		// cluster template questions and answers
		logrus.Tracef("[secretmigrator] cleaning questions and answers from cluster %s", cluster.Name)
		cleanQuestions(cluster)

		// notifiers
		notifiers, err := h.notifierLister.List(cluster.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get notifiers for cluster %s, will retry: %v", cluster.Name, err)
			return nil, err
		}
		for _, n := range notifiers {
			if n.Status.SMTPCredentialSecret == "" && n.Spec.SMTPConfig != nil {
				logrus.Tracef("[secretmigrator] migrating SMTP secrets for notifier %s in cluster %s", n.Name, cluster.Name)
				smtpSecret, err := h.migrator.CreateOrUpdateSMTPSecret("", n.Spec.SMTPConfig, cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate SMTP secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
					return nil, err
				}
				if smtpSecret != nil {
					logrus.Tracef("[secretmigrator] SMTP secret found for notifier %s in cluster %s", n.Name, cluster.Name)
					n.Status.SMTPCredentialSecret = smtpSecret.Name
					n.Spec.SMTPConfig.Password = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate SMTP secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, smtpSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
			if n.Status.WechatCredentialSecret == "" && n.Spec.WechatConfig != nil {
				logrus.Tracef("[secretmigrator] migrating Wechat secrets for notifier %s in cluster %s", n.Name, cluster.Name)
				wechatSecret, err := h.migrator.CreateOrUpdateWechatSecret("", n.Spec.WechatConfig, cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate Wechat secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
					return nil, err
				}
				if wechatSecret != nil {
					logrus.Tracef("[secretmigrator] Wechat secret found for notifier %s in cluster %s", n.Name, cluster.Name)
					n.Status.WechatCredentialSecret = wechatSecret.Name
					n.Spec.WechatConfig.Secret = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate Wechat secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, wechatSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
			if n.Status.DingtalkCredentialSecret == "" && n.Spec.DingtalkConfig != nil {
				logrus.Tracef("[secretmigrator] migrating Dingtalk secrets for notifier %s in cluster %s", n.Name, cluster.Name)
				dingtalkSecret, err := h.migrator.CreateOrUpdateDingtalkSecret(n.Status.DingtalkCredentialSecret, n.Spec.DingtalkConfig, cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate Dingtalk secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
					return nil, err
				}
				if dingtalkSecret != nil {
					logrus.Tracef("[secretmigrator] Dingtalk secret found for notifier %s in cluster %s", n.Name, cluster.Name)
					n.Status.DingtalkCredentialSecret = dingtalkSecret.Name
					n.Spec.DingtalkConfig.Secret = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate Dingtalk secrets for notifier %s in cluster %s, will retry: %v", n.Name, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, dingtalkSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
		}

		// cluster catalogs
		clusterCatalogs, err := h.clusterCatalogLister.List(cluster.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get cluster catalogs for cluster %s, will retry: %v", cluster.Name, err)
			return nil, err
		}
		for _, c := range clusterCatalogs {
			if c.Status.CredentialSecret == "" && c.Spec.Password != "" {
				logrus.Tracef("[secretmigrator] migrating secrets for cluster catalog %s in cluster %s", c.Name, cluster.Name)
				secret, err := h.migrator.CreateOrUpdateCatalogSecret(c.Status.CredentialSecret, c.Spec.Password, cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for cluster catalog %s in cluster %s, will retry: %v", c.Name, cluster.Name, err)
					return nil, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for cluster catalog %s in cluster %s", c.Name, cluster.Name)
					c.Status.CredentialSecret = secret.Name
					c.Spec.Password = ""
					_, err = h.clusterCatalogs.Update(c)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for cluster catalog %s in cluster %s, will retry: %v", c.Name, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
		}

		projects, err := h.projectLister.List(cluster.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get projects for cluster %s, will retry: %v", cluster.Name, err)
			return nil, err
		}

		// project catalogs
		for _, p := range projects {
			projectCatalogs, err := h.projectCatalogLister.List(p.Name, labels.NewSelector())
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to get project catalogs for cluster %s, will retry: %v", cluster.Name, err)
				return nil, err
			}
			for _, c := range projectCatalogs {
				if c.Status.CredentialSecret == "" && c.Spec.Password != "" {
					logrus.Tracef("[secretmigrator] migrating secrets for project catalog %s in cluster %s", c.Name, cluster.Name)
					secret, err := h.migrator.CreateOrUpdateCatalogSecret(c.Status.CredentialSecret, c.Spec.Password, cluster)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for project catalog %s in cluster %s, will retry: %v", c.Name, cluster.Name, err)
						return nil, err
					}
					if secret != nil {
						logrus.Tracef("[secretmigrator] secret found for project catalog %s in cluster %s", c.Name, cluster.Name)
						c.Status.CredentialSecret = secret.Name
						c.Spec.Password = ""
						_, err = h.projectCatalogs.Update(c)
						if err != nil {
							logrus.Errorf("[secretmigrator] failed to migrate secrets for project catalog %s in cluster %s, will retry: %v", c.Name, cluster.Name, err)
							deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
							if deleteErr != nil {
								logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
							}
							return nil, err
						}
					}
				}
			}
		}

		// sourcecodeproviderconfigs
		for _, p := range projects {
			m, err := h.getUnstructuredPipelineConfig(p.Name, model.GithubType)
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GithubType, cluster.Name, err)
				return nil, err
			}
			if !apierrors.IsNotFound(err) {
				if credentialSecret, ok := m["credentialSecret"]; ok && credentialSecret != nil {
					continue
				}
				logrus.Tracef("[secretmigrator] migrating secrets for %s pipeline config in cluster %s", model.GithubType, cluster.Name)
				github := &apiprjv3.GithubPipelineConfig{}
				if err = mapstructure.Decode(m, github); err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GithubType, cluster.Name, err)
					return nil, err
				}
				secret, err := h.migrator.CreateOrUpdateSourceCodeProviderConfigSecret("", github.ClientSecret, cluster, model.GithubType)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GithubType, cluster.Name, err)
					return nil, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for %s pipeline config in cluster %s", model.GithubType, cluster.Name)
					github.CredentialSecret = secret.Name
					github.ClientSecret = ""
					github.ObjectMeta, github.APIVersion, github.Kind, err = setSourceCodeProviderConfigMetadata(m)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GithubType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
					if _, err = h.sourceCodeProviderConfigs.ObjectClient().Update(github.Name, github); err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GithubType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
			m, err = h.getUnstructuredPipelineConfig(p.Name, model.GitlabType)
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GitlabType, cluster.Name, err)
				return nil, err
			}
			if !apierrors.IsNotFound(err) {
				if credentialSecret, ok := m["credentialSecret"]; ok && credentialSecret != nil {
					continue
				}
				logrus.Tracef("[secretmigrator] migrating secrets for %s pipeline config in cluster %s", model.GitlabType, cluster.Name)
				gitlab := &apiprjv3.GitlabPipelineConfig{}
				if err = mapstructure.Decode(m, gitlab); err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GitlabType, cluster.Name, err)
					return nil, err
				}
				secret, err := h.migrator.CreateOrUpdateSourceCodeProviderConfigSecret("", gitlab.ClientSecret, cluster, model.GitlabType)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GitlabType, cluster.Name, err)
					return nil, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for %s pipeline config in cluster %s", model.GitlabType, cluster.Name)
					gitlab.CredentialSecret = secret.Name
					gitlab.ClientSecret = ""
					gitlab.ObjectMeta, gitlab.APIVersion, gitlab.Kind, err = setSourceCodeProviderConfigMetadata(m)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GitlabType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
					if _, err = h.sourceCodeProviderConfigs.ObjectClient().Update(gitlab.Name, gitlab); err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.GitlabType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
			m, err = h.getUnstructuredPipelineConfig(p.Name, model.BitbucketCloudType)
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketCloudType, cluster.Name, err)
				return nil, err
			}
			if !apierrors.IsNotFound(err) {
				if credentialSecret, ok := m["credentialSecret"]; ok && credentialSecret != nil {
					continue
				}
				logrus.Tracef("[secretmigrator] migrating secrets for %s pipeline config in cluster %s", model.BitbucketCloudType, cluster.Name)
				bbcloud := &apiprjv3.BitbucketCloudPipelineConfig{}
				if err = mapstructure.Decode(m, bbcloud); err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketCloudType, cluster.Name, err)
					return nil, err
				}
				secret, err := h.migrator.CreateOrUpdateSourceCodeProviderConfigSecret("", bbcloud.ClientSecret, cluster, model.BitbucketCloudType)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketCloudType, cluster.Name, err)
					return nil, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for %s pipeline config in cluster %s", model.BitbucketCloudType, cluster.Name)
					bbcloud.CredentialSecret = secret.Name
					bbcloud.ClientSecret = ""
					bbcloud.ObjectMeta, bbcloud.APIVersion, bbcloud.Kind, err = setSourceCodeProviderConfigMetadata(m)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketCloudType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
					if _, err = h.sourceCodeProviderConfigs.ObjectClient().Update(bbcloud.Name, bbcloud); err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketCloudType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
			m, err = h.getUnstructuredPipelineConfig(p.Name, model.BitbucketServerType)
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketServerType, cluster.Name, err)
				return nil, err
			}
			if !apierrors.IsNotFound(err) {
				if credentialSecret, ok := m["credentialSecret"]; ok && credentialSecret != nil {
					continue
				}
				logrus.Tracef("[secretmigrator] migrating secrets for %s pipeline config in cluster %s", model.BitbucketServerType, cluster.Name)
				bbserver := &apiprjv3.BitbucketServerPipelineConfig{}
				if err = mapstructure.Decode(m, bbserver); err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketServerType, cluster.Name, err)
					return nil, err
				}
				secret, err := h.migrator.CreateOrUpdateSourceCodeProviderConfigSecret("", bbserver.PrivateKey, cluster, model.BitbucketServerType)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketServerType, cluster.Name, err)
					return nil, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for %s pipeline config in cluster %s", model.BitbucketServerType, cluster.Name)
					bbserver.CredentialSecret = secret.Name
					bbserver.PrivateKey = ""
					bbserver.ObjectMeta, bbserver.APIVersion, bbserver.Kind, err = setSourceCodeProviderConfigMetadata(m)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketServerType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
					_, err = h.sourceCodeProviderConfigs.ObjectClient().Update(bbserver.Name, bbserver)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for %s pipeline config in cluster %s, will retry: %v", model.BitbucketServerType, cluster.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return nil, err
					}
				}
			}
		}

		logrus.Tracef("[secretmigrator] setting cluster condition and updating cluster %s", cluster.Name)
		apimgmtv3.ClusterConditionSecretsMigrated.True(cluster)
		return h.clusters.Update(cluster)
	})
	return obj.(*v3.Cluster), err
}

func (h *handler) getUnstructuredPipelineConfig(namespace, pType string) (map[string]interface{}, error) {
	obj, err := h.sourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(namespace, pType, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, ok := obj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("could not get github source code provider")
	}
	return u.UnstructuredContent(), nil
}

// CreateOrUpdatePrivateRegistrySecret accepts an optional secret name and a RancherKubernetesEngineConfig object and creates a dockerconfigjson Secret
// containing the login credentials for every registry in the array, if there are any.
// If an owner is passed, the owner is set as an owner reference on the Secret. If no owner is passed,
// the caller is responsible for calling UpdateSecretOwnerReference once the owner is known.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdatePrivateRegistrySecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner *v3.Cluster) (*corev1.Secret, error) {
	if rkeConfig == nil {
		return nil, nil
	}
	rkeConfig = rkeConfig.DeepCopy()
	privateRegistries := rkeConfig.PrivateRegistries
	if len(privateRegistries) == 0 {
		return nil, nil
	}
	var existing *corev1.Secret
	if secretName != "" {
		var err error
		existing, err = m.secretLister.Get(secretNamespace, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	existingRegistry := credentialprovider.DockerConfigJSON{}
	active := make(map[string]struct{})
	needsUpdate := false
	registrySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:         secretName, // if empty, the secret will be created with a generated name
			GenerateName: "cluster-registry-",
			Namespace:    secretNamespace,
		},
		Data: map[string][]byte{},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	if owner != nil {
		registrySecret.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: owner.APIVersion,
				Kind:       owner.Kind,
				Name:       owner.Name,
				UID:        owner.UID,
			},
		}
	}
	for _, privateRegistry := range privateRegistries {
		active[privateRegistry.URL] = struct{}{}
		if privateRegistry.Password == "" {
			continue
		}
		registry := credentialprovider.DockerConfigJSON{
			Auths: credentialprovider.DockerConfig{
				privateRegistry.URL: credentialprovider.DockerConfigEntry{
					Username: privateRegistry.User,
					Password: privateRegistry.Password,
				},
			},
		}
		registryJSON, err := json.Marshal(registry)
		if err != nil {
			return nil, err
		}
		registrySecret.Data = map[string][]byte{
			corev1.DockerConfigJsonKey: registryJSON,
		}
		if existing == nil {
			registrySecret, err = m.secrets.Create(registrySecret)
			if err != nil {
				return nil, err
			}
		} else if !reflect.DeepEqual(existing.Data, registrySecret.Data) {
			err = json.Unmarshal(existing.Data[corev1.DockerConfigJsonKey], &existingRegistry)
			if err != nil {
				return nil, err
			}
			// limitation: if a URL is repeated in the privateRegistries list, it will be overwritten in the registry secret
			existingRegistry.Auths[privateRegistry.URL] = registry.Auths[privateRegistry.URL]
			registrySecret.Data[corev1.DockerConfigJsonKey], err = json.Marshal(existingRegistry)
			if err != nil {
				return nil, err
			}
			needsUpdate = true
		}
	}
	if existing != nil {
		for url := range existingRegistry.Auths {
			if _, ok := active[url]; !ok {
				delete(existingRegistry.Auths, url)
				var err error
				registrySecret.Data[corev1.DockerConfigJsonKey], err = json.Marshal(existingRegistry)
				if err != nil {
					return nil, err
				}
				needsUpdate = true
			}
		}
	}
	if needsUpdate {
		return m.secrets.Update(registrySecret)
	}
	return registrySecret, nil
}

// CleanRegistries unsets the password of every private registry in the list.
// Must be called after passwords have been migrated.
func CleanRegistries(privateRegistries []rketypes.PrivateRegistry) []rketypes.PrivateRegistry {
	for i := range privateRegistries {
		privateRegistries[i].Password = ""
	}
	return privateRegistries
}

// UpdateSecretOwnerReference sets an object as owner of a given Secret and updates the Secret.
// The object must be a non-namespaced resource.
func (m *Migrator) UpdateSecretOwnerReference(secret *corev1.Secret, owner metav1.OwnerReference) error {
	secret.OwnerReferences = []metav1.OwnerReference{owner}
	_, err := m.secrets.Update(secret)
	return err
}

// createOrUpdateSecret accepts an optional secret name and tries to update it with the provided data if it exists, or creates it.
// If an owner is provided, it sets it as an owner reference before creating or updating it.
func (m *Migrator) createOrUpdateSecret(secretName string, data map[string]string, owner runtime.Object, kind, field string) (*corev1.Secret, error) {
	var existing *corev1.Secret
	var err error
	if secretName != "" {
		existing, err = m.secretLister.Get(secretNamespace, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:         secretName,
			GenerateName: fmt.Sprintf("%s-%s-", kind, field),
			Namespace:    secretNamespace,
		},
		StringData: data,
		Type:       corev1.SecretTypeOpaque,
	}
	if owner != nil {
		gvk := owner.GetObjectKind().GroupVersionKind()
		accessor, err := meta.Accessor(owner)
		if err != nil {
			return nil, err
		}
		secret.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: gvk.Group + "/" + gvk.Version,
				Kind:       gvk.Kind,
				Name:       accessor.GetName(),
				UID:        accessor.GetUID(),
			},
		}
	}
	if existing == nil {
		return m.secrets.Create(secret)
	} else if !reflect.DeepEqual(existing.StringData, secret.StringData) {
		existing.StringData = data
		return m.secrets.Update(existing)
	}
	return secret, nil
}

// createOrUpdateSecretForCredential accepts an optional secret name and a value containing the data that needs to be sanitized,
// and creates a secret to hold the sanitized data. If an owner is passed, the owner is set as an owner reference on the secret.
func (m *Migrator) createOrUpdateSecretForCredential(secretName, secretValue string, owner runtime.Object, kind, field string) (*corev1.Secret, error) {
	if secretValue == "" {
		return nil, nil
	}
	data := map[string]string{
		"credential": secretValue,
	}
	secret, err := m.createOrUpdateSecret(secretName, data, owner, kind, field)
	if err != nil {
		return nil, fmt.Errorf("error creating secret for credential: %w", err)
	}
	return secret, nil
}

// CreateOrUpdateS3Secret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the S3BackupConfig credentials if there are any.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateS3Secret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.Services.Etcd.BackupConfig == nil || rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey, owner, "cluster", "s3backup")
}

// CreateOrUpdateWeaveSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the Weave CNI password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateWeaveSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.Network.WeaveNetworkProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, rkeConfig.Network.WeaveNetworkProvider.Password, owner, "cluster", "weave")
}

// CreateOrUpdateSMTPSecret accepts an optional secret name and an SMTPConfig object
// and creates a Secret for the SMTP server password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateSMTPSecret(secretName string, smtpConfig *apimgmtv3.SMTPConfig, owner runtime.Object) (*corev1.Secret, error) {
	if smtpConfig == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, smtpConfig.Password, owner, "notifier", "smtpconfig")
}

// CreateOrUpdateWechatSecret accepts an optional secret name and a WechatConfig object
// and creates a Secret for the Wechat credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateWechatSecret(secretName string, wechatConfig *apimgmtv3.WechatConfig, owner runtime.Object) (*corev1.Secret, error) {
	if wechatConfig == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, wechatConfig.Secret, owner, "notifier", "wechatconfig")
}

// CreateOrUpdateDingtalkSecret accepts an optional secret name and a DingtalkConfig object
// and creates a Secret for the Dingtalk credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateDingtalkSecret(secretName string, dingtalkConfig *apimgmtv3.DingtalkConfig, owner runtime.Object) (*corev1.Secret, error) {
	if dingtalkConfig == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, dingtalkConfig.Secret, owner, "notifier", "dingtalkconfig")
}

// CreateOrUpdateSourceCodeProviderConfigSecret accepts an optional secret name and a client secret or
// private key for a SourceCodeProviderConfig and creates a Secret for the credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateSourceCodeProviderConfigSecret(secretName string, credential string, owner runtime.Object, provider string) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, credential, owner, "sourcecodeproviderconfig", provider)
}

// Cleanup deletes a secret if provided a secret name, otherwise does nothing.
func (m *Migrator) Cleanup(secretName string) error {
	if secretName == "" {
		return nil
	}
	_, err := m.secretLister.Get(namespace.GlobalNamespace, secretName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = m.secrets.DeleteNamespaced(namespace.GlobalNamespace, secretName, &metav1.DeleteOptions{})
	return err
}

// MatchesQuestionPath checks whether the given string matches the question-formatted path of the
// s3 secret, weave password, or registry password.
func MatchesQuestionPath(variable string) bool {
	return variable == "rancherKubernetesEngineConfig.services.etcd.backupConfig.s3BackupConfig.secretKey" ||
		variable == "rancherKubernetesEngineConfig.network.weaveNetworkProvider.password" ||
		PrivateRegistryQuestion.MatchString(variable)
}

// cleanQuestions removes credentials from the questions and answers sections of the cluster object.
// Answers are already substituted into the spec in norman, so they can be deleted without migration.
func cleanQuestions(cluster *v3.Cluster) {
	if cluster.Spec.ClusterTemplateQuestions != nil {
		for i, q := range cluster.Spec.ClusterTemplateQuestions {
			if MatchesQuestionPath(q.Variable) {
				cluster.Spec.ClusterTemplateQuestions[i].Default = ""
			}
		}
	}
	if cluster.Spec.ClusterTemplateAnswers.Values != nil {
		for i := 0; ; i++ {
			key := fmt.Sprintf(RegistryPasswordAnswersPath, i)
			if _, ok := cluster.Spec.ClusterTemplateAnswers.Values[key]; !ok {
				break
			}
			delete(cluster.Spec.ClusterTemplateAnswers.Values, key)
		}
		delete(cluster.Spec.ClusterTemplateAnswers.Values, S3BackupAnswersPath)
		delete(cluster.Spec.ClusterTemplateAnswers.Values, WeavePasswordAnswersPath)
	}

}

func setSourceCodeProviderConfigMetadata(m map[string]interface{}) (metav1.ObjectMeta, string, string, error) {
	objectMeta, err := pipelineutils.ObjectMetaFromUnstructureContent(m)
	if err != nil {
		return metav1.ObjectMeta{}, "", "", err
	}
	if objectMeta == nil {
		return metav1.ObjectMeta{}, "", "", fmt.Errorf("could not get ObjectMeta from sourcecodeproviderconfig")
	}
	return *objectMeta, "project.cattle.io/v3", pv3.SourceCodeProviderConfigGroupVersionKind.Kind, nil
}
