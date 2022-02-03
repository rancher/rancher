package catalog

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
)

// AssembleDingtalkCredential looks up the Dingtalk Secret and inserts the keys into the Notifier.
// It returns a new copy of the Notifier without modifying the original. The Notifier is never updated.
func AssembleCatalogCredential(catalog *apimgmtv3.Catalog, secretLister v1.SecretLister) (apimgmtv3.CatalogSpec, error) {
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
