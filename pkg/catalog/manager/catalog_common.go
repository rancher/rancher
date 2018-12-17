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

func getCatalogType(cmt *CatalogInfo) string {
	if cmt.projectCatalog == nil && cmt.clusterCatalog == nil {
		return client.CatalogType
	} else if cmt.projectCatalog != nil {
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
			obj = runtime.Object(cmt.catalog)
		case client.ProjectCatalogType:
			obj = runtime.Object(cmt.projectCatalog)
		case client.ClusterCatalogType:
			obj = runtime.Object(cmt.clusterCatalog)
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
			if _, err := m.catalogClient.Update(cmt.catalog); err != nil {
				return nil, err
			}
		case client.ProjectCatalogType:
			if _, err := m.projectCatalogClient.Update(cmt.projectCatalog); err != nil {
				return nil, err
			}
		case client.ClusterCatalogType:
			if _, err := m.clusterCatalogClient.Update(cmt.clusterCatalog); err != nil {
				return nil, err
			}
		default:
			return cmt, fmt.Errorf("incorrect catalog type")
		}
		return cmt, nil
	}

	switch catalogType {
	case client.CatalogType:
		catalog := cmt.catalog
		if newCatalog, err := m.catalogClient.Update(cmt.catalog); err == nil {
			catalog = newCatalog
		} else {
			catalog, _ = m.catalogClient.Get(catalog.Name, metav1.GetOptions{})
		}
		cmt.catalog = catalog
	case client.ProjectCatalogType:
		projectCatalog := cmt.projectCatalog
		if newCatalog, err := m.projectCatalogClient.Update(projectCatalog); err == nil {
			projectCatalog = newCatalog
		} else {
			projectCatalog, _ = m.projectCatalogClient.Get(projectCatalog.Name, metav1.GetOptions{})
		}
		cmt.catalog = &projectCatalog.Catalog
		cmt.projectCatalog = projectCatalog
	case client.ClusterCatalogType:
		clusterCatalog := cmt.clusterCatalog
		if newCatalog, err := m.clusterCatalogClient.Update(clusterCatalog); err == nil {
			clusterCatalog = newCatalog
		} else {
			clusterCatalog, _ = m.clusterCatalogClient.Get(clusterCatalog.Name, metav1.GetOptions{})
		}
		cmt.catalog = &clusterCatalog.Catalog
		cmt.clusterCatalog = clusterCatalog
	default:
		return cmt, fmt.Errorf("incorrect catalog type")
	}

	return cmt, nil
}
