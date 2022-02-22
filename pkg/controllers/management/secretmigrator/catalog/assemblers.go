package catalog

import (
	"github.com/rancher/rancher/pkg/namespace"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

// AssembleDingtalkCredential looks up the Dingtalk Secret and inserts the keys into the Notifier.
// It returns a new copy of the Notifier without modifying the original. The Notifier is never updated.
func AssembleCatalogCredential(catalog *v3.Catalog, secretLister v1.SecretLister) (v3.CatalogSpec, error) {
	if catalog.Status.CredentialSecret == "" {
		if catalog.Spec.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for catalog %s are not finished migrating", catalog.Name)
		}
		return catalog.Spec, nil
	}
	secret, err := secretLister.Get(namespace.GlobalNamespace, catalog.Status.CredentialSecret)
	if err != nil {
		return catalog.Spec, err
	}
	spec := catalog.Spec.DeepCopy()
	spec.Password = string(secret.Data["credential"])
	return *spec, nil
}
