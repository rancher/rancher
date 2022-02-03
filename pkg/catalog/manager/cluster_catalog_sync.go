package manager

import (
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	helmlib "github.com/rancher/rancher/pkg/helm"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func (m *Manager) updateClusterCatalogError(clusterCatalog *v3.ClusterCatalog, err error) (runtime.Object, error) {
	setRefreshedError(&clusterCatalog.Catalog, err)
	m.clusterCatalogClient.Update(clusterCatalog)
	return nil, err
}

func (m *Manager) ClusterCatalogSync(key string, obj *v3.ClusterCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, m.deleteTemplates(name, ns)
	}

	// always get a refresh catalog from etcd
	clusterCatalog, err := m.clusterCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	commit, helm, err := helmlib.NewForceUpdate(&clusterCatalog.Catalog, m.SecretLister)
	if err != nil {
		return m.updateClusterCatalogError(clusterCatalog, err)
	}
	logrus.Debugf("Chart hash comparison for cluster catalog %v: new -- %v --- current -- %v", clusterCatalog.Name, commit, &clusterCatalog.Catalog.Status.Commit)

	if isUpToDate(commit, &clusterCatalog.Catalog) {
		if setRefreshed(&clusterCatalog.Catalog) {
			m.clusterCatalogClient.Update(clusterCatalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog:        &clusterCatalog.Catalog,
		clusterCatalog: clusterCatalog,
	}

	logrus.Infof("Updating cluster catalog %s", clusterCatalog.Name)
	return nil, m.traverseAndUpdate(helm, commit, cmt)
}
