package manager

import (
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Set_Catalog_Error_State(t *testing.T) {
	type testcase struct {
		caseName       string
		catalogInfo    CatalogInfo
		catalog        v3.Catalog
		projectCatalog v3.ProjectCatalog
		clusterCatalog v3.ClusterCatalog
	}
	testcases := []testcase{
		{
			caseName: "default",
			catalogInfo: CatalogInfo{
				catalog:        nil,
				projectCatalog: nil,
				clusterCatalog: nil,
			},
			catalog: v3.Catalog{
				ObjectMeta: v1.ObjectMeta{
					Name: "testCatalog"},
				Status: v3.CatalogStatus{
					LastRefreshTimestamp: "",
					Commit:               "",
					HelmVersionCommits:   nil,
					Conditions: []v3.CatalogCondition{
						{
							Type:    "Refreshed",
							Status:  "True",
							Message: "Test Catalog Stuff",
						},
					},
				},
			},
			projectCatalog: v3.ProjectCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ProjectName: "",
			},

			clusterCatalog: v3.ClusterCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ClusterName: "",
			},
		},
		{
			caseName: "catalogcondition nil status & nil message",
			catalogInfo: CatalogInfo{
				catalog:        nil,
				projectCatalog: nil,
				clusterCatalog: nil,
			},
			catalog: v3.Catalog{
				ObjectMeta: v1.ObjectMeta{
					Name: "testCatalog"},
				Status: v3.CatalogStatus{
					LastRefreshTimestamp: "",
					Commit:               "",
					HelmVersionCommits:   nil,
				},
			},
			projectCatalog: v3.ProjectCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ProjectName: "",
			},

			clusterCatalog: v3.ClusterCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ClusterName: "",
			},
		},
		{
			caseName: "default",
			catalogInfo: CatalogInfo{
				catalog:        nil,
				projectCatalog: nil,
				clusterCatalog: nil,
			},
			catalog: v3.Catalog{
				ObjectMeta: v1.ObjectMeta{
					Name: "testCatalog"},
				Status: v3.CatalogStatus{
					LastRefreshTimestamp: "",
					Commit:               "",
					HelmVersionCommits:   nil,
					Conditions: []v3.CatalogCondition{
						{
							Type: "Refreshed",
						},
					},
				},
			},
			projectCatalog: v3.ProjectCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ProjectName: "",
			},

			clusterCatalog: v3.ClusterCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ClusterName: "",
			},
		},
		{
			caseName: "false status",
			catalogInfo: CatalogInfo{
				catalog:        nil,
				projectCatalog: nil,
				clusterCatalog: nil,
			},
			catalog: v3.Catalog{
				ObjectMeta: v1.ObjectMeta{
					Name: "testCatalog"},
				Status: v3.CatalogStatus{
					LastRefreshTimestamp: "",
					Commit:               "",
					HelmVersionCommits:   nil,
					Conditions: []v3.CatalogCondition{
						{
							Type:    "Refreshed",
							Status:  "False",
							Message: "Test Catalog Stuff",
						},
					},
				},
			},
			projectCatalog: v3.ProjectCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ProjectName: "",
			},

			clusterCatalog: v3.ClusterCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ClusterName: "",
			},
		},
		{
			caseName: "invalid status",
			catalogInfo: CatalogInfo{
				catalog:        nil,
				projectCatalog: nil,
				clusterCatalog: nil,
			},
			catalog: v3.Catalog{
				ObjectMeta: v1.ObjectMeta{
					Name: "testCatalog"},
				Status: v3.CatalogStatus{
					LastRefreshTimestamp: "",
					Commit:               "",
					HelmVersionCommits:   nil,
					Conditions: []v3.CatalogCondition{
						{
							Type:    "Refreshed",
							Status:  "thisisnotanormalstatus",
							Message: "Test Catalog Stuff",
						},
					},
				},
			},
			projectCatalog: v3.ProjectCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ProjectName: "",
			},

			clusterCatalog: v3.ClusterCatalog{
				Namespaced:  types.Namespaced{},
				Catalog:     v3.Catalog{},
				ClusterName: "",
			},
		},
	}

	for _, c := range testcases {
		setCatalogErrorState(&c.catalogInfo, &c.catalog, &c.projectCatalog, &c.clusterCatalog)
		assert.True(t, v3.CatalogConditionRefreshed.IsFalse(&c.catalog))
		assert.Equal(t, "Error syncing catalog testCatalog", v3.CatalogConditionRefreshed.GetMessage(&c.catalog))
	}
}
