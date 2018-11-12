package manager

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func (m *Manager) ClusterCatalogSync(key string, obj *v3.ClusterCatalog) (runtime.Object, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, m.deleteTemplates(name)
	}

	// always get a refresh catalog from etcd
	clusterCatalog, err := m.clusterCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	repoPath, commit, err := m.prepareRepoPath(obj.Catalog)
	if err != nil {
		v3.CatalogConditionRefreshed.False(clusterCatalog)
		v3.CatalogConditionRefreshed.ReasonAndMessageFromError(clusterCatalog, err)
		m.clusterCatalogClient.Update(clusterCatalog)
		return nil, err
	}

	if commit == clusterCatalog.Status.Commit {
		logrus.Debugf("Catalog %s is already up to date", clusterCatalog.Name)
		if !v3.CatalogConditionRefreshed.IsTrue(clusterCatalog) {
			v3.CatalogConditionRefreshed.True(clusterCatalog)
			v3.CatalogConditionRefreshed.Reason(clusterCatalog, "")
			v3.CatalogConditionRefreshed.Message(clusterCatalog, "")
			m.clusterCatalogClient.Update(clusterCatalog)
		}
		return nil, nil
	}

	cmt := &CatalogInfo{
		catalog:        &clusterCatalog.Catalog,
		clusterCatalog: clusterCatalog,
	}

	logrus.Infof("Updating catalog %s", clusterCatalog.Name)
	return nil, m.traverseAndUpdate(repoPath, commit, cmt)
}
