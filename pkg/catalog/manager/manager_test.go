package manager

import (
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func TestLatestAvailableTemplateVersion(t *testing.T) {
	template := &v3.CatalogTemplate{
		Template: v3.Template{
			Spec: v3.TemplateSpec{
				Versions: []v3.TemplateVersionSpec{
					{
						ExternalID:        "catalog://?catalog=library&template=artifactory-ha&version=0.12.16",
						Version:           "0.12.16",
						RancherMinVersion: "v2.2.0",
						RancherMaxVersion: "v2.3.0",
					},
					{
						ExternalID:        "catalog://?catalog=library&template=artifactory-ha&version=0.12.15",
						Version:           "0.12.15",
						RancherMinVersion: "v2.1.0",
						RancherMaxVersion: "v2.2.0",
					},
					{
						ExternalID:        "catalog://?catalog=library&template=artifactory-ha&version=0.12.14",
						Version:           "0.12.14",
						RancherMinVersion: "v2.0.0",
						RancherMaxVersion: "v2.1.0",
					},
				},
			},
		},
	}

	templateWithoutRancherVersion := &v3.CatalogTemplate{
		Template: v3.Template{
			Spec: v3.TemplateSpec{
				Versions: []v3.TemplateVersionSpec{
					{
						ExternalID: "catalog://?catalog=library&template=artifactory-ha&version=0.12.16",
						Version:    "0.12.16",
					},
					{
						ExternalID: "catalog://?catalog=library&template=artifactory-ha&version=0.12.15",
						Version:    "0.12.15",
					},
					{
						ExternalID: "catalog://?catalog=library&template=artifactory-ha&version=0.12.14",
						Version:    "0.12.14",
					},
				},
			},
		},
	}

	testLatestAvailableTemplateVersion(t, "v2.1.0", "0.12.15", template)
	testLatestAvailableTemplateVersion(t, "dev", "0.12.16", template)
	testLatestAvailableTemplateVersion(t, "master", "0.12.16", template)
	testLatestAvailableTemplateVersion(t, "master-head", "0.12.16", template)
	testLatestAvailableTemplateVersion(t, "", "0.12.16", template)

	testLatestAvailableTemplateVersion(t, "v2.1.0", "0.12.16", templateWithoutRancherVersion)
	testLatestAvailableTemplateVersion(t, "dev", "0.12.16", templateWithoutRancherVersion)
	testLatestAvailableTemplateVersion(t, "master", "0.12.16", templateWithoutRancherVersion)
	testLatestAvailableTemplateVersion(t, "master-head", "0.12.16", templateWithoutRancherVersion)
	testLatestAvailableTemplateVersion(t, "", "0.12.16", templateWithoutRancherVersion)
}

func testLatestAvailableTemplateVersion(t *testing.T, serverVersion, expectedCatalogVersion string, template *v3.CatalogTemplate) {
	err := settings.ServerVersion.Set(serverVersion)
	assert.Nil(t, err)

	catalogManager := Manager{}
	templateVersion, err := catalogManager.LatestAvailableTemplateVersion(template, "")
	assert.Nil(t, err)
	assert.Equal(t, expectedCatalogVersion, templateVersion.Version)
}
