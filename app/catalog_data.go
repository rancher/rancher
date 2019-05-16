package app

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	libraryURL    = "https://git.rancher.io/charts"
	libraryBranch = "master"
	libraryName   = "library"

	systemLibraryURL    = "https://git.rancher.io/system-charts"
	systemLibraryBranch = "master"
	systemLibraryName   = "system-library"
	defSystemChartVer   = "management.cattle.io/default-system-chart-version"
)

// updateCatalogURL updates annotations if they are outdated and system catalog url/branch if it matches outdated defaults
func updateCatalogURL(catalogClient v3.CatalogInterface, desiredDefaultURL string, desiredDefaultBranch string) error {
	oldCatalog, err := catalogClient.Get(systemLibraryName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	desiredCatalog := oldCatalog.DeepCopy()

	if oldAnno := oldCatalog.Annotations[defSystemChartVer]; oldAnno == "" {
		// If url/branch are old defaults, update - otherwise they are user set and should not be changed
		if oldCatalog.Spec.URL == systemLibraryURL && oldCatalog.Spec.Branch == systemLibraryBranch {
			setDesiredChartLib(desiredCatalog, desiredDefaultURL, desiredDefaultBranch)
		}
	} else {
		oldAnnotations := make(map[string]interface{})
		json.Unmarshal([]byte(oldAnno), &oldAnnotations)

		// If url/branch catalog spec and annotation do not match, user likely has not changed it, so to new defaults
		if oldCatalog.Spec.URL == oldAnnotations["url"] && oldCatalog.Spec.Branch == oldAnnotations["branch"] {
			setDesiredChartLib(desiredCatalog, desiredDefaultURL, desiredDefaultBranch)
		}
	}

	// Annotation should be up to date with current desired default
	setCatalogAnnotation(desiredCatalog, desiredDefaultURL, desiredDefaultBranch)

	// If old catalog does not match desired catalog state, update
	if !reflect.DeepEqual(oldCatalog, desiredCatalog) {
		return exponentialCatalogUpdate(catalogClient, desiredCatalog)
	}

	return nil
}

// setCatalogAnnotation sets default system chart version to match the desired URL and desired branch env variables
func setCatalogAnnotation(catalog *v3.Catalog, desiredURL string, desiredBranch string) {
	if catalog.Annotations == nil {
		catalog.Annotations = make(map[string]string)
	}
	systemChartMap := make(map[string]string)
	systemChartMap["url"] = desiredURL
	systemChartMap["branch"] = desiredBranch

	defChartAnno, _ := json.Marshal(systemChartMap)
	catalog.Annotations[defSystemChartVer] = string(defChartAnno)
}

// setDesiredChartLib sets the catalog url and branch to match the desired URL and desired branch env variables
func setDesiredChartLib(catalog *v3.Catalog, desiredURL string, desiredBranch string) {
	catalog.Spec.URL = desiredURL
	catalog.Spec.Branch = desiredBranch
}

func exponentialCatalogUpdate(catalogClient v3.CatalogInterface, desiredCatalog *v3.Catalog) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Steps:    3,
	}
	catalog := desiredCatalog
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		if _, err := catalogClient.Update(catalog); err != nil {
			if !errors.IsConflict(err) {
				return false, err
			}

			if catalog, err = catalogClient.Get(systemLibraryName, metav1.GetOptions{}); err != nil {
				return false, err
			}
			catalog.Annotations[defSystemChartVer] = desiredCatalog.Annotations[defSystemChartVer]
			setDesiredChartLib(catalog, desiredCatalog.Spec.URL, desiredCatalog.Spec.Branch)

			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed upgrading system-chart catalog: %v", err)
	}
	return nil
}

func syncCatalogs(management *config.ManagementContext) error {
	return utilerrors.AggregateGoroutines(
		// add charts
		func() error {
			return doAddCatalogs(management, libraryName, libraryURL, libraryBranch)
		},
		// add rancher-charts
		func() error {
			if err := doAddCatalogs(management, systemLibraryName, systemLibraryURL, systemLibraryBranch); err != nil {
				return err
			}
			desiredDefaultURL := systemLibraryURL
			desiredDefaultBranch := systemLibraryBranch

			if fromEnvURL := os.Getenv("CATTLE_SYSTEM_CHART_DEFAULT_URL"); fromEnvURL != "" {
				desiredDefaultURL = fromEnvURL
			}

			if fromEnvBranch := os.Getenv("CATTLE_SYSTEM_CHART_DEFAULT_BRANCH"); fromEnvBranch != "" {
				desiredDefaultBranch = fromEnvBranch
			}

			return updateCatalogURL(management.Management.Catalogs(""), desiredDefaultURL, desiredDefaultBranch)
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
				CatalogKind: "helm:git",
				Branch:      branch,
			},
		}
		if _, err := catalogClient.Create(obj); err != nil {
			return err
		}
	}

	return nil
}
