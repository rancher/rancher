package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
)

func TestVersionBetween(t *testing.T) {
	assert.True(t, VersionBetween("1", "2", "3"))
	assert.True(t, VersionBetween("1", "2", ""))
	assert.True(t, VersionBetween("", "2", "3"))
	assert.True(t, VersionBetween("", "2", ""))
	assert.True(t, VersionBetween("1", "", ""))
	assert.True(t, VersionBetween("", "", "3"))
	assert.True(t, VersionBetween("1", "", "3"))

	assert.True(t, VersionBetween("2", "2", "2"))
	assert.True(t, VersionBetween("2", "2", ""))
	assert.True(t, VersionBetween("", "2", "2"))
}

func testVersionSatifiesRange(t *testing.T, v, rng string) {
	satisfiesRange, err := VersionSatisfiesRange(v, rng)
	assert.Nil(t, err)
	assert.True(t, satisfiesRange)
}

func testNotVersionSatifiesRange(t *testing.T, v, rng string) {
	satisfiesRange, err := VersionSatisfiesRange(v, rng)
	assert.Nil(t, err)
	assert.False(t, satisfiesRange)
}

func testInvalidVersion(t *testing.T, v, rng string) {
	satisfiesRange, _ := VersionSatisfiesRange(v, rng)
	assert.False(t, satisfiesRange)
}

func TestVersionSatifiesRange(t *testing.T) {
	testVersionSatifiesRange(t, "v1.0.0", "=1.0.0")
	testVersionSatifiesRange(t, "1.0.0", "!2.0.0")
	testVersionSatifiesRange(t, "v1.0.2", ">1.0.1 <1.0.3")
	testVersionSatifiesRange(t, "1.0.0", "<1.0.1 || >1.0.3")
	testVersionSatifiesRange(t, "v1.0.4", "<1.0.1 || >1.0.3")
	testVersionSatifiesRange(t, "v1.0.0", "=v1.0.0")
	testVersionSatifiesRange(t, "1.0.0", "!v2.0.0")
	testVersionSatifiesRange(t, "v1.0.2", ">v1.0.1 <v1.0.3")
	testVersionSatifiesRange(t, "1.0.0", "<v1.0.1 || >v1.0.3")
	testVersionSatifiesRange(t, "v1.0.4", "<v1.0.1 || >v1.0.3")

	testVersionSatifiesRange(t, "v1.0.0-rancher11", "=1.0.0-rancher11")
	testVersionSatifiesRange(t, "1.0.0-rancher11", "!1.0.0-rancher12")
	testVersionSatifiesRange(t, "v1.0.0-rancher2", ">1.0.0-rancher1 <1.0.0-rancher3")
	testVersionSatifiesRange(t, "1.0.0-rancher1", "<1.0.0-rancher2 || >1.0.0-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-rancher5", "<1.0.0-rancher2 || >1.0.0-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-rancher11", "=v1.0.0-rancher11")
	testVersionSatifiesRange(t, "1.0.0-rancher11", "!v1.0.0-rancher12")
	testVersionSatifiesRange(t, "v1.0.0-rancher2", ">v1.0.0-rancher1 <v1.0.0-rancher3")
	testVersionSatifiesRange(t, "1.0.0-rancher1", "<v1.0.0-rancher2 || >v1.0.0-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-rancher5", "<v1.0.0-rancher2 || >v1.0.0-rancher4")

	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher11", "=1.0.0-pre1-rancher11")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher11", "!1.0.0-pre1-rancher12")
	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher2", ">1.0.0-pre1-rancher1 <1.0.0-pre1-rancher3")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<1.0.0-pre1-rancher2 || >1.0.0-pre1-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher5", "<1.0.0-pre1-rancher2 || >1.0.0-pre1-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher11", "=v1.0.0-pre1-rancher11")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher11", "!v1.0.0-pre1-rancher12")
	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher2", ">v1.0.0-pre1-rancher1 <v1.0.0-pre1-rancher3")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<v1.0.0-pre1-rancher2 || >v1.0.0-pre1-rancher4")
	testVersionSatifiesRange(t, "v1.0.0-pre1-rancher5", "<v1.0.0-pre1-rancher2 || >v1.0.0-pre1-rancher4")

	testVersionSatifiesRange(t, "v1.0.0-pre11-rancher1", "=1.0.0-pre11-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre11-rancher1", "!1.0.0-pre12-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">1.0.0-pre1-rancher1 <1.0.0-pre3-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<1.0.0-pre2-rancher1 || >1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre5-rancher1", "<1.0.0-pre2-rancher1 || >1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre11-rancher1", "=v1.0.0-pre11-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre11-rancher1", "!v1.0.0-pre12-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">v1.0.0-pre1-rancher1 <v1.0.0-pre3-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<v1.0.0-pre2-rancher1 || >v1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre5-rancher1", "<v1.0.0-pre2-rancher1 || >v1.0.0-pre4-rancher1")

	testVersionSatifiesRange(t, "v1.0.0-pre11-rancher1", "=1.0.0-pre11-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre11-rancher1", "!1.0.0-pre12-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">1.0.0-pre1-rancher1 <1.0.0-pre3-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<1.0.0-pre2-rancher1 || >1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre5-rancher1", "<1.0.0-pre2-rancher1 || >1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre11-rancher1", "=v1.0.0-pre11-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre11-rancher1", "!v1.0.0-pre12-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">v1.0.0-pre1-rancher1 <v1.0.0-pre3-rancher1")
	testVersionSatifiesRange(t, "1.0.0-pre1-rancher1", "<v1.0.0-pre2-rancher1 || >v1.0.0-pre4-rancher1")
	testVersionSatifiesRange(t, "v1.0.0-pre5-rancher1", "<v1.0.0-pre2-rancher1 || >v1.0.0-pre4-rancher1")

	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">1.0.0-pre1-rancher2")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", "<1.0.0")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", ">v1.0.0-pre1-rancher2")
	testVersionSatifiesRange(t, "v1.0.0-pre2-rancher1", "<v1.0.0")

	testNotVersionSatifiesRange(t, "v1.0.0-rancher12", "=1.0.0-rancher11")
	testNotVersionSatifiesRange(t, "1.0.0-rancher12", "!1.0.0-rancher12")
	testNotVersionSatifiesRange(t, "v1.0.0-rancher5", ">1.0.0-rancher1 <1.0.0-rancher3")
	testNotVersionSatifiesRange(t, "1.0.0-rancher3", "<1.0.0-rancher2 || >1.0.0-rancher4")
	testNotVersionSatifiesRange(t, "v1.0.0-rancher12", "=v1.0.0-rancher11")
	testNotVersionSatifiesRange(t, "1.0.0-rancher12", "!v1.0.0-rancher12")
	testNotVersionSatifiesRange(t, "v1.0.0-rancher5", ">v1.0.0-rancher1 <v1.0.0-rancher3")
	testNotVersionSatifiesRange(t, "1.0.0-rancher3", "<v1.0.0-rancher2 || >v1.0.0-rancher4")

	testInvalidVersion(t, "versionInvalid-1.0", "versionInvalid-1.0")
	testInvalidVersion(t, "versionInvalid-1.0", "=versionInvalid-1.0")
	testInvalidVersion(t, "versionInvalid-1.0", "<versionInvalid-1.0")
	testInvalidVersion(t, "versionInvalid-1.0", "<=versionInvalid-1.0")
	testInvalidVersion(t, "versionInvalid-1.0", ">versionInvalid-1.0")
	testInvalidVersion(t, "versionInvalid-1.0", ">=versionInvalid-1.0")

	testInvalidVersion(t, "v1.0.0-validVersion", "versionInvalid-1.0")
	testInvalidVersion(t, "v1.0.0-validVersion", "=versionInvalid-1.0")
	testInvalidVersion(t, "v1.0.0-validVersion", ">versionInvalid-1.0")
	testInvalidVersion(t, "v1.0.0-validVersion", ">=versionInvalid-1.0")
	testInvalidVersion(t, "v1.0.0-validVersion", "<versionInvalid-1.0")
	testInvalidVersion(t, "v1.0.0-validVersion", "<=versionInvalid-1.0")

	testInvalidVersion(t, "versionInvalid-1.0", "v1.0.0-validVersion")
	testInvalidVersion(t, "versionInvalid-1.0", "=v1.0.0-validVersion")
	testInvalidVersion(t, "versionInvalid-1.0", ">v1.0.0-validVersion")
	testInvalidVersion(t, "versionInvalid-1.0", ">=v1.0.0-validVersion")
	testInvalidVersion(t, "versionInvalid-1.0", "<v1.0.0-validVersion")
	testInvalidVersion(t, "versionInvalid-1.0", "<=v1.0.0-validVersion")

}

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
	templateVertion, err := LatestAvailableTemplateVersion(template)
	assert.Nil(t, err)
	assert.Equal(t, expectedCatalogVersion, templateVertion.Version)
}
