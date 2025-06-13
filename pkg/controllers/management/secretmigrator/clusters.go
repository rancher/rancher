package secretmigrator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/rancher/pkg/fleet"
	"github.com/rancher/rancher/pkg/namespace"

	"github.com/rancher/norman/types/convert"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	SecretNamespace = namespace.GlobalNamespace
	SecretKey       = "credential"
)

func (h *handler) sync(_ string, cluster *apimgmtv3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	return h.migrateServiceAccountSecrets(cluster)
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
	}
	if !reflect.DeepEqual(existing.StringData, secret.StringData) {
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

// CreateOrUpdateHarvesterCloudConfigSecret accepts an optional secret name and a client secret or
// harvester cloud-provider-config and creates a Secret for the credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateHarvesterCloudConfigSecret(secretName string, credential string, annotations map[string]string, owner runtime.Object, provider string) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, fleet.ClustersDefaultNamespace, credential, annotations, owner, "harvester", provider)
}

// Cleanup deletes a secret if provided a secret name, otherwise does nothing.
func (m *Migrator) Cleanup(secretName string) error {
	if secretName == "" {
		return nil
	}
	_, err := m.secretLister.Get(SecretNamespace, secretName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = m.secrets.DeleteNamespaced(SecretNamespace, secretName, &metav1.DeleteOptions{})
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

type cleanupFunc func(spec *apimgmtv3.ClusterSpec)

func (h *handler) migrateServiceAccountSecrets(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(cluster) {
		return cluster, nil
	}
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
		return clusterCopy, nil
	})
	logrus.Tracef("[secretmigrator] setting cluster condition [%s] and updating cluster [%s]", apimgmtv3.ClusterConditionServiceAccountSecretsMigrated, clusterCopy.Name)
	// this is done for safety, but obj should never be nil as long as the object passed into DoUntilTrue() is not nil
	clusterCopy, _ = obj.(*apimgmtv3.Cluster)
	var err error
	clusterCopy, err = h.clusters.Update(clusterCopy)
	if err != nil {
		return cluster, err
	}
	cluster = clusterCopy.DeepCopy()
	return cluster, doErr
}
