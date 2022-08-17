package secretmigrator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/rancher/rancher/pkg/fleet"

	"github.com/rancher/norman/types/convert"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
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
	SecretNamespace              = namespace.GlobalNamespace
	SecretKey                    = "credential"
	S3BackupAnswersPath          = "rancherKubernetesEngineConfig.services.etcd.backupConfig.s3BackupConfig.secretKey"
	WeavePasswordAnswersPath     = "rancherKubernetesEngineConfig.network.weaveNetworkProvider.password"
	RegistryPasswordAnswersPath  = "rancherKubernetesEngineConfig.privateRegistries[%d].password"
	VsphereGlobalAnswersPath     = "rancherKubernetesEngineConfig.cloudProvider.vsphereCloudProvider.global.password"
	VcenterAnswersPath           = "rancherKubernetesEngineConfig.cloudProvider.vsphereCloudProvider.virtualCenter[%s].password"
	OpenStackAnswersPath         = "rancherKubernetesEngineConfig.cloudProvider.openstackCloudProvider.global.password"
	AADClientAnswersPath         = "rancherKubernetesEngineConfig.cloudProvider.azureCloudProvider.aadClientSecret"
	AADCertAnswersPath           = "rancherKubernetesEngineConfig.cloudProvider.azureCloudProvider.aadClientCertPassword"
	ACIUserKeyAnswersPath        = "rancherKubernetesEngineConfig.network.aciNetworkProvider.apicUserKey"
	ACITokenAnswersPath          = "rancherKubernetesEngineConfig.network.aciNetworkProvider.token"
	ACIKafkaClientKeyAnswersPath = "rancherKubernetesEngineConfig.network.aciNetworkProvider.kafkaClientKey"
)

var PrivateRegistryQuestion = regexp.MustCompile(`rancherKubernetesEngineConfig.privateRegistries[[0-9]+].password`)
var VcenterQuestion = regexp.MustCompile(`rancherKubernetesEngineConfig.cloudProvider.vsphereCloudProvider.virtualCenter\[.+\].password`)

func (h *handler) sync(_ string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	var err error
	cluster, err = h.migrateClusterSecrets(cluster)
	if err != nil {
		// cluster is returned here since multiple updates take place in migrateClusterSecrets and the object
		// will be set according to most up to date
		return cluster, err
	}

	return h.migrateServiceAccountSecrets(cluster)
}

// CreateOrUpdatePrivateRegistrySecret accepts an optional secret name and a RancherKubernetesEngineConfig object and creates a dockerconfigjson Secret
// containing the login credentials for every registry in the array, if there are any.
// If an owner is passed, the owner is set as an owner reference on the Secret. If no owner is passed,
// the caller is responsible for calling UpdateSecretOwnerReference once the owner is known.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdatePrivateRegistrySecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil {
		return nil, nil
	}
	rkeConfig = rkeConfig.DeepCopy()
	privateRegistries := rkeConfig.PrivateRegistries
	if len(privateRegistries) == 0 {
		return nil, nil
	}
	var existing *corev1.Secret
	var err error
	if secretName != "" {
		var err error
		existing, err = m.secretLister.Get(SecretNamespace, secretName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	registry := credentialprovider.DockerConfigJSON{
		Auths: map[string]credentialprovider.DockerConfigEntry{},
	}
	active := make(map[string]struct{})
	registrySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:         secretName, // if empty, the secret will be created with a generated name
			GenerateName: "cluster-registry-",
			Namespace:    SecretNamespace,
		},
		Data: map[string][]byte{},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	if owner != nil {
		gvk := owner.GetObjectKind().GroupVersionKind()
		accessor, err := meta.Accessor(owner)
		if err != nil {
			return nil, err
		}
		registrySecret.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: gvk.Group + "/" + gvk.Version,
				Kind:       gvk.Kind,
				Name:       accessor.GetName(),
				UID:        accessor.GetUID(),
			},
		}
	}
	if existing != nil {
		err = json.Unmarshal(existing.Data[corev1.DockerConfigJsonKey], &registry)
		if err != nil {
			return nil, err
		}
	}
	for _, privateRegistry := range privateRegistries {
		active[privateRegistry.URL] = struct{}{}
		if privateRegistry.Password == "" {
			continue
		}
		// limitation: if a URL is repeated in the privateRegistries list, it will be overwritten in the registry secret
		registry.Auths[privateRegistry.URL] = credentialprovider.DockerConfigEntry{
			Username: privateRegistry.User,
			Password: privateRegistry.Password,
		}
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
	} else {
		for url := range registry.Auths {
			if _, ok := active[url]; !ok {
				delete(registry.Auths, url)
			}
		}
		registrySecret.Data[corev1.DockerConfigJsonKey], err = json.Marshal(registry)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(existing.Data, registrySecret.Data) {
			return m.secrets.Update(registrySecret)
		}
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
	if len(secret.OwnerReferences) == 0 || !reflect.DeepEqual(secret.OwnerReferences[0], owner) {
		secret.OwnerReferences = []metav1.OwnerReference{owner}
		_, err := m.secrets.Update(secret)
		return err
	}
	return nil
}

// createOrUpdateSecret accepts an optional secret name and tries to update it with the provided data if it exists, or creates it.
// If an owner is provided, it sets it as an owner reference before creating it. If annotations are provided, they are added
// before the secret is created.
func (m *Migrator) createOrUpdateSecret(secretName, secretNamespace string, data, annotations map[string]string, owner runtime.Object, kind, field string) (*corev1.Secret, error) {
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
	if annotations != nil {
		secret.Annotations = annotations
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
func (m *Migrator) createOrUpdateSecretForCredential(secretName, secretNamespace, secretValue string, annotations map[string]string, owner runtime.Object, kind, field string) (*corev1.Secret, error) {
	if secretValue == "" {
		if secretName == "" {
			logrus.Debugf("Secret name is empty")
		}
		logrus.Debugf("Refusing to create empty secret [%s]/[%s]", secretNamespace, secretName)
		return nil, nil
	}
	data := map[string]string{
		SecretKey: secretValue,
	}
	secret, err := m.createOrUpdateSecret(secretName, secretNamespace, data, annotations, owner, kind, field)
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
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey, nil, owner, "cluster", "s3backup")
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
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.Network.WeaveNetworkProvider.Password, nil, owner, "cluster", "weave")
}

// CreateOrUpdateVsphereGlobalSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the Vsphere global password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateVsphereGlobalSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.CloudProvider.VsphereCloudProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.CloudProvider.VsphereCloudProvider.Global.Password, nil, owner, "cluster", "vsphereglobal")
}

// CreateOrUpdateVsphereVirtualCenterSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the Vsphere VirtualCenter password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateVsphereVirtualCenterSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.CloudProvider.VsphereCloudProvider == nil {
		return nil, nil
	}
	data := map[string]string{}
	for k, v := range rkeConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
		if v.Password != "" {
			data[k] = v.Password
		}
	}
	if len(data) == 0 {
		return nil, nil
	}
	return m.createOrUpdateSecret(secretName, SecretNamespace, data, nil, owner, "cluster", "vspherevcenter")
}

// CreateOrUpdateOpenStackSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the OpenStack password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateOpenStackSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.CloudProvider.OpenstackCloudProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.CloudProvider.OpenstackCloudProvider.Global.Password, nil, owner, "cluster", "openstack")
}

// CreateOrUpdateAADClientSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the AAD client secret if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateAADClientSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.CloudProvider.AzureCloudProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.CloudProvider.AzureCloudProvider.AADClientSecret, nil, owner, "cluster", "aadclientsecret")
}

// CreateOrUpdateAADCertSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the AAD client cert password if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateAADCertSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.CloudProvider.AzureCloudProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword, nil, owner, "cluster", "aadcert")
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
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, smtpConfig.Password, nil, owner, "notifier", "smtpconfig")
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
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, wechatConfig.Secret, nil, owner, "notifier", "wechatconfig")
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
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, dingtalkConfig.Secret, nil, owner, "notifier", "dingtalkconfig")
}

// CreateOrUpdateSourceCodeProviderConfigSecret accepts an optional secret name and a client secret or
// private key for a SourceCodeProviderConfig and creates a Secret for the credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateSourceCodeProviderConfigSecret(secretName string, credential string, owner runtime.Object, provider string) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, credential, nil, owner, "sourcecodeproviderconfig", provider)
}

// CreateOrUpdateHarvesterCloudConfigSecret accepts an optional secret name and a client secret or
// harvester cloud-provider-config and creates a Secret for the credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateHarvesterCloudConfigSecret(secretName string, credential string, annotations map[string]string, owner runtime.Object, provider string) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, fleet.ClustersDefaultNamespace, credential, annotations, owner, "harvester", provider)
}

// CreateOrUpdateACIAPICUserKeySecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the AciNetworkProvider user key if there are any.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateACIAPICUserKeySecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.Network.AciNetworkProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.Network.AciNetworkProvider.ApicUserKey, nil, owner, "cluster", "acikey")
}

// CreateOrUpdateACITokenSecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the AciNetworkProvider token if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateACITokenSecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.Network.AciNetworkProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.Network.AciNetworkProvider.Token, nil, owner, "cluster", "acitoken")
}

// CreateOrUpdateACIKafkaClientKeySecret accepts an optional secret name and a RancherKubernetesEngineConfig object
// and creates a Secret for the AciNetworkProvider kafka client key if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateACIKafkaClientKeySecret(secretName string, rkeConfig *rketypes.RancherKubernetesEngineConfig, owner runtime.Object) (*corev1.Secret, error) {
	if rkeConfig == nil || rkeConfig.Network.AciNetworkProvider == nil {
		return nil, nil
	}
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, rkeConfig.Network.AciNetworkProvider.KafkaClientKey, nil, owner, "cluster", "acikafkakey")
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

// CleanupKnownSecrets deletes a slice of secrets and logs any encountered errors at a WARNING level.
func (m *Migrator) CleanupKnownSecrets(secrets []*corev1.Secret) {
	for _, secret := range secrets {
		cleanUpErr := m.secrets.DeleteNamespaced(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		if cleanUpErr != nil {
			logrus.Warnf("[secretmigrator] error encountered while handling secrets cleanup for migration error; secret %s:%s may not have been cleaned up: %s", secret.Namespace, secret.Name, cleanUpErr)
		}
	}
}

// isHarvesterCluster determines if a v1.Cluster represents a harvester cluster
func (m *Migrator) isHarvesterCluster(cluster *v1.Cluster) bool {
	if cluster == nil || cluster.Spec.RKEConfig == nil {
		return false
	}

	for _, selectorConfig := range cluster.Spec.RKEConfig.MachineSelectorConfig {
		if strings.ToLower(convert.ToString(selectorConfig.Config.Data["cloud-provider-name"])) == "harvester" {
			return true
		}
	}

	return false
}

// CreateOrUpdateServiceAccountTokenSecret accepts an optional secret name and a token string
// and creates a Secret for the cluster service account token if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateServiceAccountTokenSecret(secretName string, credential string, owner runtime.Object) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, credential, nil, owner, "cluster", "serviceaccounttoken")
}

// MatchesQuestionPath checks whether the given string matches the question-formatted path of the
// s3 secret, weave password, or registry password.
func MatchesQuestionPath(variable string) bool {
	return variable == S3BackupAnswersPath ||
		variable == WeavePasswordAnswersPath ||
		PrivateRegistryQuestion.MatchString(variable) ||
		variable == VsphereGlobalAnswersPath ||
		VcenterQuestion.MatchString(variable) ||
		variable == OpenStackAnswersPath ||
		variable == AADClientAnswersPath ||
		variable == AADCertAnswersPath ||
		variable == ACIUserKeyAnswersPath ||
		variable == ACITokenAnswersPath ||
		variable == ACIKafkaClientKeyAnswersPath
}

// cleanQuestions removes credentials from the questions and answers sections of the cluster object.
// Answers are already substituted into the spec in norman, so they can be deleted without migration.
func cleanQuestions(cluster *v3.Cluster) {
	cleanQuestions := func(questions []apimgmtv3.Question) {
		for i, q := range questions {
			if MatchesQuestionPath(q.Variable) {
				questions[i].Default = ""
			}
		}
	}
	if len(cluster.Spec.ClusterTemplateQuestions) > 0 {
		cleanQuestions(cluster.Spec.ClusterTemplateQuestions)
	}
	if len(cluster.Status.AppliedSpec.ClusterTemplateQuestions) > 0 {
		cleanQuestions(cluster.Status.AppliedSpec.ClusterTemplateQuestions)
	}
	if cluster.Status.FailedSpec != nil && len(cluster.Status.FailedSpec.ClusterTemplateQuestions) > 0 {
		cleanQuestions(cluster.Status.FailedSpec.ClusterTemplateQuestions)
	}
	cleanAnswers := func(answers *apimgmtv3.Answer) {
		for i := 0; ; i++ {
			key := fmt.Sprintf(RegistryPasswordAnswersPath, i)
			if _, ok := answers.Values[key]; !ok {
				break
			}
			delete(answers.Values, key)
		}
		if cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider != nil {
			vcenters := cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter
			for k := range vcenters {
				key := fmt.Sprintf(VcenterAnswersPath, k)
				delete(answers.Values, key)
			}
		}
		delete(answers.Values, S3BackupAnswersPath)
		delete(answers.Values, WeavePasswordAnswersPath)
		delete(answers.Values, VsphereGlobalAnswersPath)
		delete(answers.Values, OpenStackAnswersPath)
		delete(answers.Values, AADClientAnswersPath)
		delete(answers.Values, AADCertAnswersPath)
		delete(answers.Values, ACIUserKeyAnswersPath)
		delete(answers.Values, ACITokenAnswersPath)
		delete(answers.Values, ACIKafkaClientKeyAnswersPath)
	}
	if cluster.Spec.ClusterTemplateAnswers.Values != nil {
		cleanAnswers(&cluster.Spec.ClusterTemplateAnswers)
	}
	if cluster.Status.AppliedSpec.ClusterTemplateAnswers.Values != nil {
		cleanAnswers(&cluster.Status.AppliedSpec.ClusterTemplateAnswers)
	}
	if cluster.Status.FailedSpec != nil && cluster.Status.FailedSpec.ClusterTemplateAnswers.Values != nil {
		cleanAnswers(&cluster.Status.FailedSpec.ClusterTemplateAnswers)
	}
}

func (h *handler) migrateClusterSecrets(cluster *v3.Cluster) (*v3.Cluster, error) {
	clusterCopy := cluster.DeepCopy()
	obj, doErr := apimgmtv3.ClusterConditionSecretsMigrated.DoUntilTrue(clusterCopy, func() (runtime.Object, error) {
		// privateRegistries
		if clusterCopy.GetSecret("PrivateRegistrySecret") == "" {
			logrus.Tracef("[secretmigrator] migrating private registry secrets for cluster %s", clusterCopy.Name)
			regSecret, err := h.migrator.CreateOrUpdatePrivateRegistrySecret(clusterCopy.GetSecret("PrivateRegistrySecret"), clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if regSecret != nil {
				logrus.Tracef("[secretmigrator] private registry secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.PrivateRegistrySecret = regSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(clusterCopy.Spec.RancherKubernetesEngineConfig.PrivateRegistries)
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries = CleanRegistries(clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.PrivateRegistries)
				}

				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate private registry secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, regSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}

		// s3 backup cred
		if clusterCopy.GetSecret("S3CredentialSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating S3 secrets for cluster %s", clusterCopy.Name)
			s3Secret, err := h.migrator.CreateOrUpdateS3Secret("", clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if s3Secret != nil {
				logrus.Tracef("[secretmigrator] S3 secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.S3CredentialSecret = s3Secret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate S3 secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, s3Secret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}

		// weave CNI password
		if clusterCopy.GetSecret("WeavePasswordSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating weave CNI secrets for cluster %s", clusterCopy.Name)
			weaveSecret, err := h.migrator.CreateOrUpdateWeaveSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if weaveSecret != nil {
				logrus.Tracef("[secretmigrator] weave secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.WeavePasswordSecret = weaveSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate weave CNI secrets for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, weaveSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}

		// cloud provider secrets

		// vsphere global
		if clusterCopy.GetSecret("VsphereSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere global secret for cluster %s", clusterCopy.Name)
			vsphereSecret, err := h.migrator.CreateOrUpdateVsphereGlobalSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if vsphereSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere global secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.VsphereSecret = vsphereSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere global secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, vsphereSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}
		// vsphere virtual center
		if clusterCopy.GetSecret("VirtualCenterSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating vsphere virtualcenter secret for cluster %s", clusterCopy.Name)
			vcenterSecret, err := h.migrator.CreateOrUpdateVsphereVirtualCenterSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if vcenterSecret != nil {
				logrus.Tracef("[secretmigrator] vsphere virtualcenter secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.VirtualCenterSecret = vcenterSecret.Name
				for k, v := range clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
					v.Password = ""
					clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
				}
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider != nil {
					for k, v := range clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
						v.Password = ""
						clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
					}
				}

				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider != nil {
					for k, v := range clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
						v.Password = ""
						clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
					}
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate vsphere virtualcenter secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, vcenterSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}
		// openstack
		if clusterCopy.GetSecret("OpenStackSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating openstack secret for cluster %s", clusterCopy.Name)
			openStackSecret, err := h.migrator.CreateOrUpdateOpenStackSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate openstack secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if openStackSecret != nil {
				logrus.Tracef("[secretmigrator] openstack secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.OpenStackSecret = openStackSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate openstack secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, openStackSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}
		// aad client secret
		if clusterCopy.GetSecret("AADClientSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating aad client secret for cluster %s", clusterCopy.Name)
			aadClientSecret, err := h.migrator.CreateOrUpdateAADClientSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad client secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if aadClientSecret != nil {
				logrus.Tracef("[secretmigrator] aad client secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.AADClientSecret = aadClientSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad client secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aadClientSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}
		// aad cert password
		if clusterCopy.GetSecret("AADClientCertSecret") == "" {
			logrus.Tracef("[secretmigrator] migrating aad cert secret for cluster %s", clusterCopy.Name)
			aadCertSecret, err := h.migrator.CreateOrUpdateAADCertSecret("", clusterCopy.Spec.RancherKubernetesEngineConfig, nil)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if aadCertSecret != nil {
				logrus.Tracef("[secretmigrator] aad cert secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.AADClientCertSecret = aadCertSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aad cert secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aadCertSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy.DeepCopy()
			}
		}

		// cluster template questions and answers
		logrus.Tracef("[secretmigrator] cleaning questions and answers from cluster %s", clusterCopy.Name)
		cleanQuestions(clusterCopy)

		// notifiers
		notifiers, err := h.notifierLister.List(clusterCopy.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get notifiers for cluster %s, will retry: %v", clusterCopy.Name, err)
			return cluster, err
		}
		for _, n := range notifiers {
			if n.Status.SMTPCredentialSecret == "" && n.Spec.SMTPConfig != nil {
				logrus.Tracef("[secretmigrator] migrating SMTP secrets for notifier %s in cluster %s", n.Name, clusterCopy.Name)
				smtpSecret, err := h.migrator.CreateOrUpdateSMTPSecret("", n.Spec.SMTPConfig, clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate SMTP secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
					return cluster, err
				}
				if smtpSecret != nil {
					logrus.Tracef("[secretmigrator] SMTP secret found for notifier %s in cluster %s", n.Name, clusterCopy.Name)
					n.Status.SMTPCredentialSecret = smtpSecret.Name
					n.Spec.SMTPConfig.Password = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate SMTP secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, smtpSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return cluster, err
					}
				}
			}
			if n.Status.WechatCredentialSecret == "" && n.Spec.WechatConfig != nil {
				logrus.Tracef("[secretmigrator] migrating Wechat secrets for notifier %s in cluster %s", n.Name, clusterCopy.Name)
				wechatSecret, err := h.migrator.CreateOrUpdateWechatSecret("", n.Spec.WechatConfig, clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate Wechat secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
					return cluster, err
				}
				if wechatSecret != nil {
					logrus.Tracef("[secretmigrator] Wechat secret found for notifier %s in cluster %s", n.Name, clusterCopy.Name)
					n.Status.WechatCredentialSecret = wechatSecret.Name
					n.Spec.WechatConfig.Secret = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate Wechat secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, wechatSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return cluster, err
					}
				}
			}
			if n.Status.DingtalkCredentialSecret == "" && n.Spec.DingtalkConfig != nil {
				logrus.Tracef("[secretmigrator] migrating Dingtalk secrets for notifier %s in cluster %s", n.Name, clusterCopy.Name)
				dingtalkSecret, err := h.migrator.CreateOrUpdateDingtalkSecret(n.Status.DingtalkCredentialSecret, n.Spec.DingtalkConfig, clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate Dingtalk secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
					return cluster, err
				}
				if dingtalkSecret != nil {
					logrus.Tracef("[secretmigrator] Dingtalk secret found for notifier %s in cluster %s", n.Name, clusterCopy.Name)
					n.Status.DingtalkCredentialSecret = dingtalkSecret.Name
					n.Spec.DingtalkConfig.Secret = ""
					_, err = h.notifiers.Update(n)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate Dingtalk secrets for notifier %s in cluster %s, will retry: %v", n.Name, clusterCopy.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, dingtalkSecret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return cluster, err
					}
				}
			}
		}

		// cluster catalogs
		clusterCatalogs, err := h.clusterCatalogLister.List(clusterCopy.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get cluster catalogs for cluster %s, will retry: %v", clusterCopy.Name, err)
			return cluster, err
		}
		for _, c := range clusterCatalogs {
			if c.GetSecret() == "" && c.Spec.Password != "" {
				logrus.Tracef("[secretmigrator] migrating secrets for cluster catalog %s in cluster %s", c.Name, clusterCopy.Name)
				secret, err := h.migrator.CreateOrUpdateCatalogSecret(c.GetSecret(), c.Spec.Password, cluster)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for cluster catalog %s in cluster %s, will retry: %v", c.Name, clusterCopy.Name, err)
					return cluster, err
				}
				if secret != nil {
					logrus.Tracef("[secretmigrator] secret found for cluster catalog %s in cluster %s", c.Name, clusterCopy.Name)
					c.Spec.CatalogSecrets.CredentialSecret = secret.Name
					c.Spec.Password = ""
					_, err = h.clusterCatalogs.Update(c)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for cluster catalog %s in cluster %s, will retry: %v", c.Name, clusterCopy.Name, err)
						deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, secret.Name, &metav1.DeleteOptions{})
						if deleteErr != nil {
							logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
						}
						return cluster, err
					}
				}
			}
		}

		projects, err := h.projectLister.List(clusterCopy.Name, labels.NewSelector())
		if err != nil {
			logrus.Errorf("[secretmigrator] failed to get projects for cluster %s, will retry: %v", clusterCopy.Name, err)
			return cluster, err
		}

		// project catalogs
		for _, p := range projects {
			projectCatalogs, err := h.projectCatalogLister.List(p.Name, labels.NewSelector())
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to get project catalogs for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			for _, c := range projectCatalogs {
				if c.GetSecret() == "" && c.Spec.Password != "" {
					logrus.Tracef("[secretmigrator] migrating secrets for project catalog %s in cluster %s", c.Name, clusterCopy.Name)
					secret, err := h.migrator.CreateOrUpdateCatalogSecret(c.GetSecret(), c.Spec.Password, clusterCopy)
					if err != nil {
						logrus.Errorf("[secretmigrator] failed to migrate secrets for project catalog %s in cluster %s, will retry: %v", c.Name, clusterCopy.Name, err)
						return cluster, err
					}
					if secret != nil {
						logrus.Tracef("[secretmigrator] secret found for project catalog %s in cluster %s", c.Name, clusterCopy.Name)
						c.Spec.CatalogSecrets.CredentialSecret = secret.Name
						c.Spec.Password = ""
						_, err = h.projectCatalogs.Update(c)
						if err != nil {
							logrus.Errorf("[secretmigrator] failed to migrate secrets for project catalog %s in cluster %s, will retry: %v", c.Name, clusterCopy.Name, err)
							deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, secret.Name, &metav1.DeleteOptions{})
							if deleteErr != nil {
								logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
							}
							return cluster, err
						}
					}
				}
			}
		}

		logrus.Tracef("[secretmigrator] setting cluster condition [%s] and updating cluster [%s]", apimgmtv3.ClusterConditionSecretsMigrated, clusterCopy.Name)
		clusterCopy, err = h.clusters.Update(clusterCopy)
		if err != nil {
			return cluster, err
		}
		cluster = clusterCopy.DeepCopy()
		// clusterCopy is returned here since it's value will be passed and fields modified without an update
		return clusterCopy, err
	})
	// this is done for safety, but obj should never be nil as long as the object passed into DoUntilTrue() is not nil
	clusterCopy, _ = obj.(*v3.Cluster)
	var err error
	clusterCopy, err = h.clusters.Update(clusterCopy)
	if err != nil {
		return cluster, err
	}
	cluster = clusterCopy.DeepCopy()
	return cluster, doErr
}

func (h *handler) migrateServiceAccountSecrets(cluster *v3.Cluster) (*v3.Cluster, error) {
	clusterCopy := cluster.DeepCopy()
	obj, doErr := apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.DoUntilTrue(clusterCopy, func() (runtime.Object, error) {
		// serviceAccountToken
		if clusterCopy.Status.ServiceAccountTokenSecret == "" {
			logrus.Tracef("[secretmigrator] migrating service account token secret for cluster %s", clusterCopy.Name)
			saSecret, err := h.migrator.CreateOrUpdateServiceAccountTokenSecret(clusterCopy.Status.ServiceAccountTokenSecret, clusterCopy.Status.ServiceAccountToken, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate service account token secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if saSecret != nil {
				logrus.Tracef("[secretmigrator] service account token secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Status.ServiceAccountTokenSecret = saSecret.Name
				clusterCopy.Status.ServiceAccountToken = ""
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate service account token secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, saSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy
			}
		}
		logrus.Tracef("[secretmigrator] setting cluster condition [%s] and updating cluster [%s]", apimgmtv3.ClusterConditionServiceAccountSecretsMigrated, clusterCopy.Name)
		var err error
		clusterCopy, err = h.clusters.Update(clusterCopy)
		if err != nil {
			return cluster, err
		}
		cluster = clusterCopy.DeepCopy()
		return clusterCopy, nil
	})
	// this is done for safety, but obj should never be nil as long as the object passed into DoUntilTrue() is not nil
	clusterCopy, _ = obj.(*v3.Cluster)
	var err error
	clusterCopy, err = h.clusters.Update(clusterCopy)
	if err != nil {
		return cluster, err
	}
	cluster = clusterCopy.DeepCopy()
	return cluster, doErr
}

func (h *handler) migrateACISecrets(cluster *v3.Cluster) (*v3.Cluster, error) {
	clusterCopy := cluster.DeepCopy()
	obj, doErr := apimgmtv3.ClusterConditionACISecretsMigrated.DoUntilTrue(clusterCopy, func() (runtime.Object, error) {
		// aci apic user key
		if clusterCopy.Spec.ClusterSecrets.ACIAPICUserKeySecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci apic user key secret for cluster %s", clusterCopy.Name)
			aciUserKeySecret, err := h.migrator.CreateOrUpdateACIAPICUserKeySecret(clusterCopy.Spec.ClusterSecrets.ACIAPICUserKeySecret, clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci apic user key secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if aciUserKeySecret != nil {
				logrus.Tracef("[secretmigrator] aci apic user key secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.ACIAPICUserKeySecret = aciUserKeySecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci apic user key secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciUserKeySecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy
			}
		}

		// aci token
		if clusterCopy.Spec.ClusterSecrets.ACITokenSecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci token secret for cluster %s", clusterCopy.Name)
			aciTokenSecret, err := h.migrator.CreateOrUpdateACITokenSecret(clusterCopy.Spec.ClusterSecrets.ACITokenSecret, clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci token secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if aciTokenSecret != nil {
				logrus.Tracef("[secretmigrator] aci token secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.ACITokenSecret = aciTokenSecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci token secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciTokenSecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy
			}
		}

		// aci kafka client key
		if clusterCopy.Spec.ClusterSecrets.ACIKafkaClientKeySecret == "" {
			logrus.Tracef("[secretmigrator] migrating aci kafkaClientKey secret for cluster %s", clusterCopy.Name)
			aciKafkaClientKeySecret, err := h.migrator.CreateOrUpdateACIKafkaClientKeySecret(clusterCopy.Spec.ClusterSecrets.ACIKafkaClientKeySecret, clusterCopy.Spec.RancherKubernetesEngineConfig, clusterCopy)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate aci kafka client key secret for cluster %s, will retry: %v", clusterCopy.Name, err)
				return cluster, err
			}
			if aciKafkaClientKeySecret != nil {
				logrus.Tracef("[secretmigrator] aci kafka client key secret found for cluster %s", clusterCopy.Name)
				clusterCopy.Spec.ClusterSecrets.ACIKafkaClientKeySecret = aciKafkaClientKeySecret.Name
				clusterCopy.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey = ""
				if clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.AppliedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey = ""
				}
				if clusterCopy.Status.FailedSpec != nil && clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig != nil {
					clusterCopy.Status.FailedSpec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey = ""
				}
				clusterCopy, err = h.clusters.Update(clusterCopy)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate aci kafka client key secret for cluster %s, will retry: %v", cluster.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, aciKafkaClientKeySecret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return cluster, err
				}
				cluster = clusterCopy
			}
		}
		logrus.Tracef("[secretmigrator] setting cluster condition [%s] and updating cluster [%s]", apimgmtv3.ClusterConditionACISecretsMigrated, clusterCopy.Name)
		var err error
		clusterCopy, err = h.clusters.Update(clusterCopy)
		if err != nil {
			return cluster, err
		}
		cluster = clusterCopy.DeepCopy()
		return clusterCopy, nil
	})
	// this is done for safety, but obj should never be nil as long as the object passed into Do() is not nil
	clusterCopy, _ = obj.(*v3.Cluster)
	var err error
	clusterCopy, err = h.clusters.Update(clusterCopy)
	if err != nil {
		return cluster, err
	}
	cluster = clusterCopy.DeepCopy()
	return cluster, doErr
}
