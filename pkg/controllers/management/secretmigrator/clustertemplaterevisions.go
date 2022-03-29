package secretmigrator

import (
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) syncTemplate(key string, clusterTemplateRevision *v3.ClusterTemplateRevision) (runtime.Object, error) {
	if clusterTemplateRevision == nil || clusterTemplateRevision.DeletionTimestamp != nil {
		return clusterTemplateRevision, nil
	}
	if v3.ClusterTemplateRevisionConditionSecretsMigrated.IsTrue(clusterTemplateRevision) {
		logrus.Tracef("[secretmigrator] clusterTemplateRevision %s already migrated", clusterTemplateRevision.Name)
		return clusterTemplateRevision, nil
	}
	obj, err := v3.ClusterTemplateRevisionConditionSecretsMigrated.Do(clusterTemplateRevision, func() (runtime.Object, error) {
		// privateRegistries
		if clusterTemplateRevision.Status.PrivateRegistrySecret == "" {
			logrus.Tracef("[secretmigrator] migrating private registry secrets for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			regSecret, err := h.migrator.CreateOrUpdatePrivateRegistrySecret(clusterTemplateRevision.Status.PrivateRegistrySecret, clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevision)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if regSecret != nil {
				logrus.Tracef("[secretmigrator] private registry secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.PrivateRegistrySecret = regSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.PrivateRegistries)
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, regSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// s3 backup cred
		if clusterTemplateRevision.Status.S3CredentialSecret == "" {
			logrus.Tracef("[secretmigrator] migrating S3 secrets for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			s3Secret, err := h.migrator.CreateOrUpdateS3Secret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevision)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if s3Secret != nil {
				logrus.Tracef("[secretmigrator] S3 secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.S3CredentialSecret = s3Secret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, s3Secret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// weave CNI password
		if clusterTemplateRevision.Status.WeavePasswordSecret == "" {
			logrus.Tracef("[secretmigrator] migrating weave CNI secrets for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			weaveSecret, err := h.migrator.CreateOrUpdateWeaveSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevision)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if weaveSecret != nil {
				logrus.Tracef("[secretmigrator] weave secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.WeavePasswordSecret = weaveSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, weaveSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cloud provider secrets

		// vsphere global
		if clusterTemplateRevision.Status.VsphereSecret == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere global secret for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			vsphereSecret, err := h.migrator.CreateOrUpdateVsphereGlobalSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevision)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if vsphereSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere global secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.VsphereSecret = vsphereSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, vsphereSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// vsphere virtual center
		if clusterTemplateRevision.Status.VirtualCenterSecret == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere virtualcenter secret for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			vcenterSecret, err := h.migrator.CreateOrUpdateVsphereVirtualCenterSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevision)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if vcenterSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere virtualcenter secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.VirtualCenterSecret = vcenterSecret.Name
				for k, v := range clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
					v.Password = ""
					clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
				}
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, vcenterSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// openstack
		if clusterTemplateRevision.Status.OpenStackSecret == "" {
			logrus.Tracef("[secretmigrator] migrating openstack secret for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			openStackSecret, err := h.migrator.CreateOrUpdateOpenStackSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate openstack secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if openStackSecret != nil {
				logrus.Tracef("[secretmigrator] openstack secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.OpenStackSecret = openStackSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate openstack secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, openStackSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// aad client secret
		if clusterTemplateRevision.Status.AADClientSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aad client secret for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			aadClientSecret, err := h.migrator.CreateOrUpdateAADClientSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad client secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if aadClientSecret != nil {
				logrus.Tracef("[secretmigrator] aad client secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.AADClientSecret = aadClientSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad client secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, aadClientSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// aad cert password
		if clusterTemplateRevision.Status.AADClientCertSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aad cert secret for clusterTemplateRevision %s", clusterTemplateRevision.Name)
			aadCertSecret, err := h.migrator.CreateOrUpdateAADCertSecret("", clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
				return nil, err
			}
			if aadCertSecret != nil {
				logrus.Tracef("[secretmigrator] aad cert secret found for clusterTemplateRevision %s", clusterTemplateRevision.Name)
				clusterTemplateRevision.Status.AADClientCertSecret = aadCertSecret.Name
				clusterTemplateRevision.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevision)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(secretNamespace, aadCertSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cluster template questions and answers
		// The cluster store will look up defaults in the ClusterConfig after assembling it.
		logrus.Tracef("[secretmigrator] cleaning questions and answers from clusterTemplateRevision %s", clusterTemplateRevision.Name)
		for i, q := range clusterTemplateRevision.Spec.Questions {
			if MatchesQuestionPath(q.Variable) {
				clusterTemplateRevision.Spec.Questions[i].Default = ""
			}
		}

		logrus.Tracef("[secretmigrator] setting clusterTemplateRevision condition and updating clusterTemplateRevision %s", clusterTemplateRevision.Name)
		v3.ClusterTemplateRevisionConditionSecretsMigrated.True(clusterTemplateRevision)
		return h.clusterTemplateRevisions.Update(clusterTemplateRevision)
	})
	return obj.(*v3.ClusterTemplateRevision), err
}
