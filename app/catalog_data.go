package app

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	libraryURL    = "https://git.rancher.io/charts"
	libraryBranch = "master"
	libraryName   = "library"

	systemLibraryURL    = "https://git.rancher.io/system-charts"
	systemLibraryBranch = "master"
	systemLibraryName   = "system-library"
)

func addCatalogs(management *config.ManagementContext) error {
	return utilerrors.AggregateGoroutines(
		// add charts
		func() error {
			return doAddCatalogs(management, libraryName, libraryURL, libraryBranch)
		},
		// add rancher-charts
		func() error {
			return doAddCatalogs(management, systemLibraryName, systemLibraryURL, systemLibraryBranch)
		},
	)
}

func doAddCatalogs(management *config.ManagementContext, name, url, branch string) error {
	catalogClient := management.Management.Catalogs("")
	_, err := catalogClient.Get(name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		obj := &v3.Catalog{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v3.CatalogSpec{
				URL:         url,
				CatalogKind: "helm",
				Branch:      branch,
			},
		}
		if _, err := catalogClient.Create(obj); err != nil {
			return err
		}
	}
	return nil
}
