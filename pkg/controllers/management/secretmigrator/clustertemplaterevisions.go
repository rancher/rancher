package secretmigrator

import (
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) syncTemplate(_ string, clusterTemplateRevision *apimgmtv3.ClusterTemplateRevision) (runtime.Object, error) {
	if clusterTemplateRevision == nil || clusterTemplateRevision.DeletionTimestamp != nil {
		return clusterTemplateRevision, nil
	}
	clusterTemplateRevisionCopy := clusterTemplateRevision.DeepCopy()
	obj, doErr := apimgmtv3.ClusterTemplateRevisionConditionSecretsMigrated.DoUntilTrue(clusterTemplateRevisionCopy, func() (runtime.Object, error) {
		// privateRegistries
		if clusterTemplateRevisionCopy.Status.PrivateRegistrySecret == "" {
			logrus.Tracef("[secretmigrator] migrating private registry secrets for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			regSecret, err := h.migrator.CreateOrUpdatePrivateRegistrySecret(clusterTemplateRevisionCopy.Status.PrivateRegistrySecret, clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if regSecret != nil {
				logrus.Tracef("[secretmigrator] private registry secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.PrivateRegistrySecret = regSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.PrivateRegistries)
				clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, regSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// s3 backup cred
		if clusterTemplateRevisionCopy.Status.S3CredentialSecret == "" {
			logrus.Tracef("[secretmigrator] migrating S3 secrets for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			s3Secret, err := h.migrator.CreateOrUpdateS3Secret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if s3Secret != nil {
				logrus.Tracef("[secretmigrator] S3 secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.S3CredentialSecret = s3Secret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, s3Secret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// weave CNI password
		if clusterTemplateRevisionCopy.Status.WeavePasswordSecret == "" {
			logrus.Tracef("[secretmigrator] migrating weave CNI secrets for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			weaveSecret, err := h.migrator.CreateOrUpdateWeaveSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if weaveSecret != nil {
				logrus.Tracef("[secretmigrator] weave secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.WeavePasswordSecret = weaveSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, weaveSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cloud provider secrets

		// vsphere global
		if clusterTemplateRevisionCopy.Status.VsphereSecret == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere global secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			vsphereSecret, err := h.migrator.CreateOrUpdateVsphereGlobalSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if vsphereSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere global secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.VsphereSecret = vsphereSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, vsphereSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// vsphere virtual center
		if clusterTemplateRevisionCopy.Status.VirtualCenterSecret == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere virtualcenter secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			vcenterSecret, err := h.migrator.CreateOrUpdateVsphereVirtualCenterSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if vcenterSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere virtualcenter secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.VirtualCenterSecret = vcenterSecret.Name
				for k, v := range clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
					v.Password = ""
					clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
				}
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, vcenterSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// openstack
		if clusterTemplateRevisionCopy.Status.OpenStackSecret == "" {
			logrus.Tracef("[secretmigrator] migrating openstack secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			openStackSecret, err := h.migrator.CreateOrUpdateOpenStackSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate openstack secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if openStackSecret != nil {
				logrus.Tracef("[secretmigrator] openstack secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.OpenStackSecret = openStackSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate openstack secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, openStackSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// aad client secret
		if clusterTemplateRevisionCopy.Status.AADClientSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aad client secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			aadClientSecret, err := h.migrator.CreateOrUpdateAADClientSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad client secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if aadClientSecret != nil {
				logrus.Tracef("[secretmigrator] aad client secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.AADClientSecret = aadClientSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad client secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aadClientSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}
		// aad cert password
		if clusterTemplateRevisionCopy.Status.AADClientCertSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aad cert secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			aadCertSecret, err := h.migrator.CreateOrUpdateAADCertSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if aadCertSecret != nil {
				logrus.Tracef("[secretmigrator] aad cert secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.AADClientCertSecret = aadCertSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aadCertSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cluster template questions and answers
		// The cluster store will look up defaults in the ClusterConfig after assembling it.
		logrus.Tracef("[secretmigrator] cleaning questions and answers from clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
		for i, q := range clusterTemplateRevisionCopy.Spec.Questions {
			if MatchesQuestionPath(q.Variable) {
				clusterTemplateRevisionCopy.Spec.Questions[i].Default = ""
			}
		}

		var err error
		clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
		if err != nil {
			return clusterTemplateRevision, err
		}
		clusterTemplateRevision = clusterTemplateRevisionCopy
		return clusterTemplateRevisionCopy, nil
	})
	clusterTemplateRevisionCopy, _ = obj.(*apimgmtv3.ClusterTemplateRevision)
	var err error
	logrus.Tracef("[secretmigrator] setting clusterTemplateRevision [%s] condition and updating clusterTemplateRevision [%s]", apimgmtv3.ClusterTemplateRevisionConditionSecretsMigrated, clusterTemplateRevisionCopy.Name)
	clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
	if err != nil {
		return clusterTemplateRevision, err
	}
	clusterTemplateRevision = clusterTemplateRevisionCopy.DeepCopy()
	if doErr != nil {
		return clusterTemplateRevision, doErr
	}

	obj, doErr = apimgmtv3.ClusterTemplateRevisionConditionACISecretsMigrated.DoUntilTrue(clusterTemplateRevisionCopy, func() (runtime.Object, error) {
		// aci apic user key
		if clusterTemplateRevisionCopy.Status.ACIAPICUserKeySecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci apic user key secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			aciAPICUserKeySecret, err := h.migrator.CreateOrUpdateACIAPICUserKeySecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci apic user key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if aciAPICUserKeySecret != nil {
				logrus.Tracef("[secretmigrator] aci apic user key secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.ACIAPICUserKeySecret = aciAPICUserKeySecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci apic user key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciAPICUserKeySecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// aci token
		if clusterTemplateRevisionCopy.Status.ACITokenSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci token secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			aciTokenSecret, err := h.migrator.CreateOrUpdateACITokenSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci token secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if aciTokenSecret != nil {
				logrus.Tracef("[secretmigrator] aci token secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.ACITokenSecret = aciTokenSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token = ""
				clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci token secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciTokenSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// aci kafka client key
		if clusterTemplateRevisionCopy.Status.ACIKafkaClientKeySecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci kafka client key secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			aciKafkaClientKeySecret, err := h.migrator.CreateOrUpdateACIKafkaClientKeySecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci kafka client key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if aciKafkaClientKeySecret != nil {
				logrus.Tracef("[secretmigrator] aci kafka client key secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.ACIKafkaClientKeySecret = aciKafkaClientKeySecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci kafka client key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciKafkaClientKeySecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cluster template questions and answers
		// The cluster store will look up defaults in the ClusterConfig after assembling it.
		logrus.Tracef("[secretmigrator] cleaning questions and answers from clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
		for i, q := range clusterTemplateRevisionCopy.Spec.Questions {
			if MatchesQuestionPath(q.Variable) {
				clusterTemplateRevisionCopy.Spec.Questions[i].Default = ""
			}
		}

		var err error
		clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
		if err != nil {
			return clusterTemplateRevision, err
		}
		clusterTemplateRevision = clusterTemplateRevisionCopy
		return clusterTemplateRevisionCopy, nil
	})

	clusterTemplateRevisionCopy, _ = obj.(*apimgmtv3.ClusterTemplateRevision)
	logrus.Tracef("[secretmigrator] setting clusterTemplateRevision [%s] condition and updating clusterTemplateRevision [%s]", apimgmtv3.ClusterTemplateRevisionConditionACISecretsMigrated, clusterTemplateRevisionCopy.Name)
	clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
	if err != nil {
		return clusterTemplateRevision, err
	}
	clusterTemplateRevision = clusterTemplateRevisionCopy.DeepCopy()
	if doErr != nil {
		return clusterTemplateRevision, doErr
	}

	obj, doErr = apimgmtv3.ClusterTemplateRevisionConditionRKESecretsMigrated.DoUntilTrue(clusterTemplateRevisionCopy, func() (runtime.Object, error) {
		// rke secrets encryption providers
		if clusterTemplateRevisionCopy.Status.SecretsEncryptionProvidersSecret == "" {
			logrus.Tracef("[secretmigrator] migrating secrets encryption provider secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			secretsEncryptionProvidersSecret, err := h.migrator.CreateOrUpdateSecretsEncryptionProvidersSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate secrets encryption provider secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if secretsEncryptionProvidersSecret != nil {
				logrus.Tracef("[secretmigrator] secrets encryption provider secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.SecretsEncryptionProvidersSecret = secretsEncryptionProvidersSecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig.Resources = nil
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets encryption provider secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, secretsEncryptionProvidersSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// rke bastion host
		if clusterTemplateRevisionCopy.Status.BastionHostSSHKeySecret == "" {
			logrus.Tracef("[secretmigrator] migrating rke bastion host ssh key secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			bastionHostSSHKeySecret, err := h.migrator.CreateOrUpdateBastionHostSSHKeySecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate rke bastion host ssh key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if bastionHostSSHKeySecret != nil {
				logrus.Tracef("[secretmigrator] rke bastion host ssh key secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.BastionHostSSHKeySecret = bastionHostSSHKeySecret.Name
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.BastionHost.SSHKey = ""
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate rke bastion host ssh key secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, bastionHostSSHKeySecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// rke kubelet extra env
		if clusterTemplateRevisionCopy.Status.KubeletExtraEnvSecret == "" {
			logrus.Tracef("[secretmigrator] migrating rke kubelet extra env secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			kubeletExtraEnvSecret, err := h.migrator.CreateOrUpdateKubeletExtraEnvSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate rke kubelet extra env secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if kubeletExtraEnvSecret != nil {
				logrus.Tracef("[secretmigrator] rke kubelet extra env secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.KubeletExtraEnvSecret = kubeletExtraEnvSecret.Name
				env := make([]string, 0, len(clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv))
				for _, e := range clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv {
					if !strings.Contains(e, "AWS_SECRET_ACCESS_KEY") {
						env = append(env, e)
					}
				}
				clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv = env
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate rke kubelet extra env secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, kubeletExtraEnvSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// rke private registry ecr
		if clusterTemplateRevisionCopy.Status.PrivateRegistryECRSecret == "" {
			logrus.Tracef("[secretmigrator] migrating rke private registry ecr secret for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
			privateRegistryEcrSecret, err := h.migrator.CreateOrUpdatePrivateRegistryECRSecret("", clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig, clusterTemplateRevisionCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate rke private registry ecr secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevisionCopy.Name, err)
				return clusterTemplateRevision, err
			}
			if privateRegistryEcrSecret != nil {
				logrus.Tracef("[secretmigrator] rke private registry ecr secret found for clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
				clusterTemplateRevisionCopy.Status.PrivateRegistryECRSecret = privateRegistryEcrSecret.Name
				for _, reg := range clusterTemplateRevisionCopy.Spec.ClusterConfig.RancherKubernetesEngineConfig.PrivateRegistries {
					if ecr := reg.ECRCredentialPlugin; ecr != nil {
						ecr.AwsSecretAccessKey = ""
						ecr.AwsSessionToken = ""
					}
				}
				clusterTemplateRevisionCopy, err := h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate rke private registry ecr secret for clusterTemplateRevision %s, will retry: %v", clusterTemplateRevision.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, privateRegistryEcrSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return clusterTemplateRevision, err
				}
				clusterTemplateRevision = clusterTemplateRevisionCopy
			}
		}

		// cluster template questions and answers
		// The cluster store will look up defaults in the ClusterConfig after assembling it.
		logrus.Tracef("[secretmigrator] cleaning questions and answers from clusterTemplateRevision %s", clusterTemplateRevisionCopy.Name)
		for i, q := range clusterTemplateRevisionCopy.Spec.Questions {
			if MatchesQuestionPath(q.Variable) {
				clusterTemplateRevisionCopy.Spec.Questions[i].Default = ""
			}
		}

		var err error
		clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
		if err != nil {
			return clusterTemplateRevision, err
		}
		clusterTemplateRevision = clusterTemplateRevisionCopy
		return clusterTemplateRevisionCopy, nil
	})

	clusterTemplateRevisionCopy, _ = obj.(*apimgmtv3.ClusterTemplateRevision)
	logrus.Tracef("[secretmigrator] setting clusterTemplateRevision [%s] condition and updating clusterTemplateRevision [%s]", apimgmtv3.ClusterTemplateRevisionConditionRKESecretsMigrated, clusterTemplateRevisionCopy.Name)
	clusterTemplateRevisionCopy, err = h.clusterTemplateRevisions.Update(clusterTemplateRevisionCopy)
	if err != nil {
		return clusterTemplateRevision, err
	}
	clusterTemplateRevision = clusterTemplateRevisionCopy.DeepCopy()
	return clusterTemplateRevision, doErr
}
