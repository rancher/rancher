package manager

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func hasAllUpdates(catalog *v3.Catalog) bool {
	upgraded := v3.CatalogConditionUpgraded.IsTrue(catalog)
	diskCached := v3.CatalogConditionDiskCached.IsTrue(catalog)
	return upgraded && diskCached
}

func IsUpToDate(commit string, catalog *v3.Catalog) bool {
	commitsEqual := commit == catalog.Status.Commit
	updated := hasAllUpdates(catalog)
	return commitsEqual && updated
}

func setRefreshed(catalog *v3.Catalog) bool {
	logrus.Debugf("Catalog %s is already up to date", catalog.Name)
	if !v3.CatalogConditionRefreshed.IsTrue(catalog) {
		v3.CatalogConditionRefreshed.True(catalog)
		v3.CatalogConditionRefreshed.Reason(catalog, "")
		v3.CatalogConditionRefreshed.Message(catalog, "")
		return true
	}
	return false
}

func SetRefreshedError(catalog *v3.Catalog, err error) {
	v3.CatalogConditionRefreshed.False(catalog)
	v3.CatalogConditionRefreshed.ReasonAndMessageFromError(catalog, err)
}

func (m *Manager) deleteTemplates(key string, namespace string) error {
	templates, err := m.getTemplateMap(key, namespace)
	if err != nil {
		return err
	}
	tvToDelete := map[string]struct{}{}
	for _, t := range templates {
		tvs, err := m.getTemplateVersion(t.Name, namespace)
		if err != nil {
			return err
		}
		for k := range tvs {
			tvToDelete[k] = struct{}{}
		}
	}
	go func() {
		for {
			for k := range templates {
				if err := m.templateClient.DeleteNamespaced(namespace, k, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
					logrus.Warnf("Deleting template %v doesn't succeed. Continue loop", k)
					continue
				}
			}
			for k := range tvToDelete {
				if err := m.templateVersionClient.DeleteNamespaced(namespace, k, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
					logrus.Warnf("Deleting templateVersion %v doesn't succeed. Continue loop", k)
					continue
				}
			}
			break
		}
	}()
	return nil
}

func GetCatalogType(cmt *CatalogInfo) string {
	if cmt.ProjectCatalog == nil && cmt.ClusterCatalog == nil {
		return client.CatalogType
	} else if cmt.ProjectCatalog != nil {
		return client.ProjectCatalogType
	} else {
		return client.ClusterCatalogType
	}
}

func (m *Manager) updateCatalogInfo(cmt *CatalogInfo, catalogType string, templateName string, condition bool, updateOnly bool) (*CatalogInfo, error) {
	var obj runtime.Object
	if condition {
		switch catalogType {
		case client.CatalogType:
			obj = runtime.Object(cmt.Catalog)
		case client.ProjectCatalogType:
			obj = runtime.Object(cmt.ProjectCatalog)
		case client.ClusterCatalogType:
			obj = runtime.Object(cmt.ClusterCatalog)
		default:
			return cmt, fmt.Errorf("incorrect catalog type")
		}
		v3.CatalogConditionRefreshed.Unknown(obj)
		if templateName != "" {
			v3.CatalogConditionRefreshed.Message(obj, fmt.Sprintf("syncing template %v", templateName))
		} else {
			v3.CatalogConditionRefreshed.Message(obj, fmt.Sprintf(""))
		}
	}

	if updateOnly {
		switch catalogType {
		case client.CatalogType:
			if _, err := m.catalogClient.Update(cmt.Catalog); err != nil {
				return nil, err
			}
		case client.ProjectCatalogType:
			if _, err := m.projectCatalogClient.Update(cmt.ProjectCatalog); err != nil {
				return nil, err
			}
		case client.ClusterCatalogType:
			if _, err := m.clusterCatalogClient.Update(cmt.ClusterCatalog); err != nil {
				return nil, err
			}
		default:
			return cmt, fmt.Errorf("incorrect catalog type")
		}
		return cmt, nil
	}

	switch catalogType {
	case client.CatalogType:
		catalog := cmt.Catalog
		if newCatalog, err := m.catalogClient.Update(cmt.Catalog); err == nil {
			catalog = newCatalog
		} else {
			catalog, _ = m.catalogClient.Get(catalog.Name, metav1.GetOptions{})
		}
		cmt.Catalog = catalog
	case client.ProjectCatalogType:
		projectCatalog := cmt.ProjectCatalog
		if newCatalog, err := m.projectCatalogClient.Update(projectCatalog); err == nil {
			projectCatalog = newCatalog
		} else {
			projectCatalog, _ = m.projectCatalogClient.Get(projectCatalog.Name, metav1.GetOptions{})
		}
		cmt.Catalog = &projectCatalog.Catalog
		cmt.ProjectCatalog = projectCatalog
	case client.ClusterCatalogType:
		clusterCatalog := cmt.ClusterCatalog
		if newCatalog, err := m.clusterCatalogClient.Update(clusterCatalog); err == nil {
			clusterCatalog = newCatalog
		} else {
			clusterCatalog, _ = m.clusterCatalogClient.Get(clusterCatalog.Name, metav1.GetOptions{})
		}
		cmt.Catalog = &clusterCatalog.Catalog
		cmt.ClusterCatalog = clusterCatalog
	default:
		return cmt, fmt.Errorf("incorrect catalog type")
	}

	return cmt, nil
}
