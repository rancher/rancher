package secretmigrator

import (
	"encoding/json"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	ClusterType                 = "cluster"
	ClusterTemplateRevisionType = "cluster template revision"
)

// AssemblePrivateRegistryCredential looks up the registry Secret and inserts the keys into the PrivateRegistries list on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssemblePrivateRegistryCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || len(spec.RancherKubernetesEngineConfig.PrivateRegistries) == 0 {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.PrivateRegistries[0].Password != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	registrySecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	dockerCfg := credentialprovider.DockerConfigJSON{}
	err = json.Unmarshal(registrySecret.Data[".dockerconfigjson"], &dockerCfg)
	if err != nil {
		return spec, err
	}
	for i, privateRegistry := range spec.RancherKubernetesEngineConfig.PrivateRegistries {
		if reg, ok := dockerCfg.Auths[privateRegistry.URL]; ok {
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].User = reg.Username
			spec.RancherKubernetesEngineConfig.PrivateRegistries[i].Password = reg.Password
		}
	}
	return spec, nil
}

// AssembleS3Credential looks up the S3 backup config Secret and inserts the keys into the S3BackupConfig on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleS3Credential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig == nil || spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil
	}
	s3Cred, err := secretLister.Get(namespace.GlobalNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = string(s3Cred.Data[SecretKey])
	return spec, nil
}

// AssembleWeaveCredential looks up the weave Secret and inserts the keys into the network provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleWeaveCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	weaveSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = string(weaveSecret.Data[SecretKey])
	return spec, nil
}

// AssembleVsphereGlobalCredential looks up the vsphere global Secret and inserts the keys into the cloud provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleVsphereGlobalCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	vsphereSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = string(vsphereSecret.Data[SecretKey])
	return spec, nil
}

// AssembleVsphereVirtualCenterCredential looks up the vsphere virtualcenter Secret and inserts the keys into the cloud provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleVsphereVirtualCenterCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		for _, v := range spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
			if v.Password != "" {
				logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
				break
			}
		}
		return spec, nil

	}
	vcenterSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	for k, v := range vcenterSecret.Data {
		vCenter := spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k]
		vCenter.Password = string(v)
		spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = vCenter
	}
	return spec, nil
}

// AssembleOpenStackCredential looks up the OpenStack Secret and inserts the keys into the cloud provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleOpenStackCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	openStackSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = string(openStackSecret.Data[SecretKey])
	return spec, nil
}

// AssembleAADClientSecretCredential looks up the AAD client secret Secret and inserts the keys into the cloud provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleAADClientSecretCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	aadClientSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = string(aadClientSecret.Data[SecretKey])
	return spec, nil
}

// AssembleAADCertCredential looks up the AAD client cert password Secret and inserts the keys into the cloud provider config on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleAADCertCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	aadCertSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = string(aadCertSecret.Data[SecretKey])
	return spec, nil
}

// AssembleACIAPICUserKeyCredential looks up the aci apic user key Secret and inserts the keys into the AciNetworkProvider on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleACIAPICUserKeyCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	aciUserKeySecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey = string(aciUserKeySecret.Data[SecretKey])
	return spec, nil
}

// AssembleACITokenCredential looks up the aci token Secret and inserts the keys into the AciNetworkProvider on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleACITokenCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	aciTokenSecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token = string(aciTokenSecret.Data[SecretKey])
	return spec, nil
}

// AssembleACIKafkaClientKeyCredential looks up the aci kafka client key Secret and inserts the keys into the AciNetworkProvider on the Cluster spec.
// It returns a new copy of the spec without modifying the original. The Cluster is never updated.
func AssembleACIKafkaClientKeyCredential(secretRef, objType, objName string, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	if spec.RancherKubernetesEngineConfig == nil || spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider == nil {
		return spec, nil
	}
	if secretRef == "" {
		if spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey != "" {
			logrus.Warnf("[secretmigrator] secrets for %s %s are not finished migrating", objType, objName)
		}
		return spec, nil

	}
	aciKafkaClientKeySecret, err := secretLister.Get(SecretNamespace, secretRef)
	if err != nil {
		return spec, err
	}
	spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey = string(aciKafkaClientKeySecret.Data[SecretKey])
	return spec, nil
}

// AssembleRKEConfigSpec is a wrapper assembler for assembling configs on Clusters.
func AssembleRKEConfigSpec(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	spec, err := AssembleS3Credential(cluster.GetSecret("S3CredentialSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssemblePrivateRegistryCredential(cluster.GetSecret("PrivateRegistrySecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleWeaveCredential(cluster.GetSecret("WeavePasswordSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleVsphereGlobalCredential(cluster.GetSecret("VsphereSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleVsphereVirtualCenterCredential(cluster.GetSecret("VirtualCenterSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleOpenStackCredential(cluster.GetSecret("OpenStackSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleAADClientSecretCredential(cluster.GetSecret("AADClientSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleAADCertCredential(cluster.GetSecret("AADClientCertSecret"), ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleACIAPICUserKeyCredential(cluster.Spec.ClusterSecrets.ACIAPICUserKeySecret, ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleACITokenCredential(cluster.Spec.ClusterSecrets.ACITokenSecret, ClusterType, cluster.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	return AssembleACIKafkaClientKeyCredential(cluster.Spec.ClusterSecrets.ACIKafkaClientKeySecret, ClusterType, cluster.Name, spec, secretLister)
}

// AssembleRKEConfigTemplateSpec is a wrapper assembler for assembling configs on ClusterTemplateRevisions. It returns a ClusterSpec.
func AssembleRKEConfigTemplateSpec(template *apimgmtv3.ClusterTemplateRevision, spec apimgmtv3.ClusterSpec, secretLister v1.SecretLister) (apimgmtv3.ClusterSpec, error) {
	spec, err := AssembleS3Credential(template.Status.S3CredentialSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssemblePrivateRegistryCredential(template.Status.PrivateRegistrySecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleWeaveCredential(template.Status.WeavePasswordSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleVsphereGlobalCredential(template.Status.VsphereSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleVsphereVirtualCenterCredential(template.Status.VirtualCenterSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleOpenStackCredential(template.Status.OpenStackSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleAADClientSecretCredential(template.Status.AADClientSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleAADCertCredential(template.Status.AADClientCertSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleACIAPICUserKeyCredential(template.Status.ACIAPICUserKeySecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	spec, err = AssembleACITokenCredential(template.Status.ACITokenSecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
	if err != nil {
		return spec, err
	}
	return AssembleACIKafkaClientKeyCredential(template.Status.ACIKafkaClientKeySecret, ClusterTemplateRevisionType, template.Name, spec, secretLister)
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
	spec.SMTPConfig.Password = string(smtpSecret.Data[SecretKey])
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
	spec.WechatConfig.Secret = string(wechatSecret.Data[SecretKey])
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
	spec.DingtalkConfig.Secret = string(secret.Data[SecretKey])
	return spec, nil
}
