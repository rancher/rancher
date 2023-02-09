package catalog

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
)

const secretKey = "credential"

// AssembleCatalogCredential looks up the Catalog Secret and inserts the value into the catalog spec.
// It returns a new copy of the Spec without modifying the original. The Catalog is never updated.
func AssembleCatalogCredential(catalog *apimgmtv3.Catalog, secretLister v1.SecretLister) (apimgmtv3.CatalogSpec, error) {
	if catalog.GetSecret() == "" {
		if catalog.Spec.Password != "" {
			logrus.Warnf("[secretmigrator] secrets for catalog %s are not finished migrating", catalog.Name)
		}
		return catalog.Spec, nil
	}
	secret, err := secretLister.Get(namespace.GlobalNamespace, catalog.GetSecret())
	if err != nil {
		return catalog.Spec, err
	}
	spec := catalog.Spec.DeepCopy()
	spec.Password = string(secret.Data[secretKey])
	return *spec, nil
}
