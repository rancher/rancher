package management

import (
	"encoding/json"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewCatalogClientMock(oldURL string, oldBranch string, annoURL string, annoBranch string) v3.CatalogInterface {
	catalog := &v3.Catalog{
		Spec: v32.CatalogSpec{
			URL:    oldURL,
			Branch: oldBranch,
		},
	}

	if annoURL != "" || annoBranch != "" {
		setCatalogAnnotation(catalog, annoURL, annoBranch)
	}

	return &fakes.CatalogInterfaceMock{
		GetFunc: func(name string, opts v1.GetOptions) (*v3.Catalog, error) {
			if name == systemLibraryName {
				return catalog, nil
			}
			return nil, nil
		},
		UpdateFunc: func(c *v3.Catalog) (*v3.Catalog, error) {
			catalog = c
			return catalog, nil
		},
	}
}

// TestUpdateCatalogHasAnnoMatchEnv tests for scenarios where system-library catalog does not contain a chart version annotation
func TestUpdateCatalogNoAnno(t *testing.T) {
	assert := assert.New(t)

	// If Annotations field is nil and current url/branch match v2.2.2 values,
	// the url/branch should be set and annotation updated
	c := NewCatalogClientMock(systemLibraryURL, systemLibraryBranch, "", "")

	updateCatalogURL(c, systemLibraryURL, "v3")

	catalog, err := c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v3", catalog.Spec.Branch)

	annoMap := make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v3", annoMap["branch"])

	// If Annotations field is nil and current url/branch do not match v2.2.2 values,
	// the url/branch should not be set and annotations should be updated
	c = NewCatalogClientMock("https://test-system-chart-url.io/", "v4", "", "")

	updateCatalogURL(c, systemLibraryURL, "v3")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal("https://test-system-chart-url.io/", catalog.Spec.URL)
	assert.Equal("v4", catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v3", annoMap["branch"])
}

// TestUpdateCatalogHasAnnoMatchEnv tests for scenarios where system-library catalog contains a chart version annotation,
// and the system-library catalog spec does match the "SYSTEM_CHART_DEFAULT" environment variable
func TestUpdateCatalogHasAnnoMatchEnv(t *testing.T) {
	assert := assert.New(t)

	// If the url and branch DO match the old defaults and DO match environment and DO match annotation
	// the url/branch and annotation should remain the same
	c := NewCatalogClientMock(systemLibraryURL, systemLibraryBranch, systemLibraryURL, systemLibraryBranch)

	updateCatalogURL(c, systemLibraryURL, systemLibraryBranch)
	catalog, err := c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal(systemLibraryBranch, catalog.Spec.Branch)

	annoMap := make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal(systemLibraryBranch, annoMap["branch"])

	// If the url and branch DO match the old defaults and DO match environment and DO NOT match annotation
	// the url/branch should NOT be updated and annotations should be updated
	c = NewCatalogClientMock(systemLibraryURL, systemLibraryBranch, systemLibraryURL, "v3")

	updateCatalogURL(c, systemLibraryURL, systemLibraryBranch)
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal(systemLibraryBranch, catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal(systemLibraryBranch, annoMap["branch"])

	// If the url and branch DO NOT match the old defaults and DO match environment and DO match annotation
	// the url/branch should NOT be updated and annotations NOT should be updated
	c = NewCatalogClientMock(systemLibraryURL, "v4", systemLibraryURL, "v4")

	updateCatalogURL(c, systemLibraryURL, "v4")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v4", catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v4", annoMap["branch"])

	// If the url and branch DO NOT match the old defaults and DO match environment and DO NOT match annotation
	// the url/branch should NOT be updated and annotations should be updated
	c = NewCatalogClientMock(systemLibraryURL, "v5", systemLibraryURL, "v4")

	updateCatalogURL(c, systemLibraryURL, "v5")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v5", catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v5", annoMap["branch"])
}

// TestUpdateCatalogHasAnnoMatchEnv tests for scenarios where system-library catalog contains a chart version annotation,
// and the system-library catalog spec does not match the "SYSTEM_CHART_DEFAULT" environment variable
func TestUpdateCatalogHasAnnoNoMatchEnv(t *testing.T) {
	assert := assert.New(t)

	// If the url and branch DO match the old defaults, DO NOT match Environment, DO match annotation
	// the url/branch should be set and annotations should be updated
	c := NewCatalogClientMock(systemLibraryURL, systemLibraryBranch, systemLibraryURL, systemLibraryBranch)

	updateCatalogURL(c, systemLibraryURL, "v2.2")
	catalog, err := c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v2.2", catalog.Spec.Branch)

	annoMap := make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v2.2", annoMap["branch"])

	// If the url and branch DO match the old defaults, DO NOT match Environment, DO NOT match annotation
	// the url/branch should not be set and annotations should be updated (if not equal to environment)
	c = NewCatalogClientMock(systemLibraryURL, systemLibraryBranch, systemLibraryURL, "v2")

	updateCatalogURL(c, systemLibraryURL, "v2.2")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal(systemLibraryBranch, catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v2.2", annoMap["branch"])

	// If the url and branch DO NOT match the old defaults, DO NOT match Environment, DO match annotation
	// the url/branch should be set and annotations should be updated
	c = NewCatalogClientMock(systemLibraryURL, "v0.5", systemLibraryURL, "v0.5")

	updateCatalogURL(c, systemLibraryURL, "v2.2")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v2.2", catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(systemLibraryURL, annoMap["url"])
	assert.Equal("v2.2", annoMap["branch"])

	// If the url and branch DO NOT match the old defaults, DO NOT match Environment, DO NOT match annotation
	// the url/branch should not be set and annotations should be updated (if not equal to environment)
	c = NewCatalogClientMock(systemLibraryURL, "v0.5", systemLibraryURL, "v1")

	updateCatalogURL(c, systemLibraryURL, "v2.2")
	catalog, err = c.Get(systemLibraryName, v1.GetOptions{})
	assert.Nil(err)
	assert.Equal(systemLibraryURL, catalog.Spec.URL)
	assert.Equal("v0.5", catalog.Spec.Branch)

	annoMap = make(map[string]string)
	err = json.Unmarshal([]byte(catalog.Annotations[defSystemChartVer]), &annoMap)
	assert.Nil(err)
	assert.Equal(annoMap["url"], systemLibraryURL)
	assert.Equal(annoMap["branch"], "v2.2")
}
