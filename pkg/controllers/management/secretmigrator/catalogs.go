package secretmigrator

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func (h *handler) syncCatalog(key string, catalog *v3.Catalog) (runtime.Object, error) {
	if catalog == nil || catalog.DeletionTimestamp != nil {
		return catalog, nil
	}

	obj, err := apimgmtv3.CatalogConditionSecretsMigrated.DoUntilTrue(catalog, func() (runtime.Object, error) {
		if catalog.Status.CredentialSecret == "" && catalog.Spec.Password != "" {
			logrus.Tracef("[secretmigrator] migrating secrets for global catalog %s", catalog.Name)
			secret, err := h.migrator.CreateOrUpdateCatalogSecret("", catalog.Spec.Password, catalog)
			if err != nil {
				logrus.Errorf("[secretmigrator] failed to migrate secrets for global catalog %s, will retry: %v", catalog.Name, err)
				return nil, err
			}
			if secret != nil {
				logrus.Tracef("[secretmigrator] secret found for global catalog %s", catalog.Name)
				catalog.Status.CredentialSecret = secret.Name
				catalog.Spec.Password = ""
				catalogCopy, err := h.catalogs.Update(catalog)
				if err != nil {
					logrus.Errorf("[secretmigrator] failed to migrate secrets for global catalog %s, will retry: %v", catalog.Name, err)
					deleteErr := h.migrator.secrets.DeleteNamespaced(SecretNamespace, secret.Name, &metav1.DeleteOptions{})
					if deleteErr != nil {
						logrus.Errorf("[secretmigrator] encountered error while handling migration error: %v", deleteErr)
					}
					return nil, err
				}
				catalog = catalogCopy
			}
		}

		logrus.Tracef("[secretmigrator] setting catalog condition and updating catalog %s", catalog.Name)
		apimgmtv3.CatalogConditionSecretsMigrated.True(catalog)
		return h.catalogs.Update(catalog)
	})
	return obj.(*v3.Catalog), err
}

// CreateOrUpdateCatalogSecret accepts an optional secret name and a catalog password
// and creates a Secret for the credential if there is one.
// If an owner is passed, the owner is set as an owner reference on the Secret.
// It returns a reference to the Secret if one was created. If the returned Secret is not nil and there is no error,
// the caller is responsible for un-setting the secret data, setting a reference to the Secret, and
// updating the Cluster object, if applicable.
func (m *Migrator) CreateOrUpdateCatalogSecret(secretName, password string, owner runtime.Object) (*corev1.Secret, error) {
	return m.createOrUpdateSecretForCredential(secretName, SecretNamespace, password, nil, owner, "catalog", "password")
}
