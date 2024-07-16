package external

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rke/types/kdm"
	"github.com/stretchr/testify/assert"
)

const (
	k3s            = "k3s"
	rke2           = "rke2"
	rke1           = "rke1"
	k3sWebVersion  = "v1.30.2+k3s1"
	rke2WebVersion = "v1.30.1+rke2r1"
	devKDM         = "https://github.com/rancher/kontainer-driver-metadata/raw/dev-v2.9/data/data.json"
	// TODO: Change to release-v2.9 once that branch has been created
	releaseKDM = "https://releases.rancher.com/kontainer-driver-metadata/release-v2.8/data.json"
)

func createDummyKdmData(version, maxChannelVersion, minChannelVersion string) map[string]interface{} {
	data := make(map[string]interface{})
	data["version"] = version
	data["maxChannelServerVersion"] = maxChannelVersion
	data["minChannelServerVersion"] = minChannelVersion
	return data
}

func buildExternalData(entries ...interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	out["releases"] = entries
	return out
}

func Test_GetLatestPatchesForVersion(t *testing.T) {

	type test struct {
		name             string
		externalKdmData  map[string]interface{}
		expectedVersions []DistributionVersion
		source           Source
		wantErr          bool
	}

	tests := []test{
		{
			name:   "Get latest RKE2 patch for multiple versions",
			source: rke2,
			externalKdmData: buildExternalData(
				createDummyKdmData("v1.30.1+rke2r1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.30.2+rke2r1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.29.4+rke2r1", "2.9.0", "2.8.0"),
				createDummyKdmData("v1.29.5+rke2r1", "2.9.0", "2.8.0"),
			),
			expectedVersions: []DistributionVersion{
				{
					VersionString: "v1.30.2+rke2r1",
				},
				{
					VersionString: "v1.29.5+rke2r1",
				},
			},
			wantErr: false,
		},
		{
			name:   "Get latest RKE2 release version for same patch",
			source: rke2,
			externalKdmData: buildExternalData(createDummyKdmData("v1.30.1+rke2r1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.30.1+rke2r2", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.29.4+rke2r1", "2.9.0", "2.8.0"),
				createDummyKdmData("v1.29.4+rke2r2", "2.9.0", "2.8.0")),
			expectedVersions: []DistributionVersion{
				{
					VersionString: "v1.30.1+rke2r2",
				},
				{
					VersionString: "v1.29.4+rke2r2",
				},
			},
		},
		{
			name:   "Get latest K3s patch for multiple versions",
			source: k3s,
			externalKdmData: buildExternalData(createDummyKdmData("v1.30.1+k3s1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.30.2+k3s1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.29.4+k3s1", "2.9.0", "2.8.0"),
				createDummyKdmData("v1.29.5+k3s1", "2.9.0", "2.8.0")),
			expectedVersions: []DistributionVersion{
				{
					VersionString: "v1.30.2+k3s1",
				},
				{
					VersionString: "v1.29.5+k3s1",
				},
			},
		},
		{
			name:   "Get latest K3s patch for multiple versions",
			source: k3s,
			externalKdmData: buildExternalData(createDummyKdmData("v1.30.1+k3s1", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.30.1+k3s2", "2.10.0", "2.9.0"),
				createDummyKdmData("v1.29.4+k3s1", "2.9.0", "2.8.0"),
				createDummyKdmData("v1.29.4+k3s2", "2.9.0", "2.8.0")),
			expectedVersions: []DistributionVersion{
				{
					VersionString: "v1.30.1+k3s2",
				},
				{
					VersionString: "v1.29.4+k3s2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			patches, err := GetLatestPatchesForSupportedVersions("2.9.0", test.externalKdmData, test.source, &semver.Version{
				Major: 1,
				Minor: 21,
				Patch: 0})

			if err != nil && !test.wantErr {
				t.Error(err)
			}

			if len(patches) != len(test.expectedVersions) {
				t.Errorf("Did not receive the correct amount of versions, expected %d, got %d", len(test.expectedVersions), len(patches))
			}

			for _, expectedVer := range test.expectedVersions {
				if !slices.ContainsFunc(patches, func(version DistributionVersion) bool {
					return version.VersionString == expectedVer.VersionString
				}) {
					t.Errorf("did not find version %s in list of returned versions %v", expectedVer.VersionString, patches)
				}
			}
		})
	}
}

func TestGetExternalImages(t *testing.T) {
	kubeSemVer := &semver.Version{
		Major: 1,
		Minor: 21,
		Patch: 0,
	}

	type args struct {
		rancherVersion           string
		externalData             map[string]interface{}
		source                   Source
		versions                 []DistributionVersion
		minimumKubernetesVersion *semver.Version
		kdmUrl                   string
		image1                   string
		image2                   string
		image3                   string
	}

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "k3s-test",
			args: args{
				externalData:             map[string]interface{}{},
				source:                   k3s,
				minimumKubernetesVersion: kubeSemVer,
				kdmUrl:                   devKDM,
				image1:                   "rancher/klipper-lb:v0.4.7",
				image2:                   "rancher/mirrored-pause:3.6",
				image3:                   "rancher/mirrored-metrics-server:v0.7.0",
				versions: []DistributionVersion{
					{
						VersionString: k3sWebVersion,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "rke2-test",
			args: args{
				externalData: map[string]interface{}{},
				source:       rke2,
				versions: []DistributionVersion{
					{
						VersionString: rke2WebVersion,
					},
				},
				minimumKubernetesVersion: kubeSemVer,
				kdmUrl:                   releaseKDM,
				image1:                   "rancher/mirrored-pause:3.6",
				image2:                   "rancher/rke2-runtime:v1.30.1-rke2r1",
				image3:                   "rancher/rke2-cloud-provider:v1.29.3-build20240412",
			},
			wantErr: false,
		},
		{
			name: "rke1-test-fail",
			args: args{
				externalData:             map[string]interface{}{},
				source:                   rke1,
				minimumKubernetesVersion: kubeSemVer,
				kdmUrl:                   releaseKDM,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			get, err := http.Get(tt.args.kdmUrl)
			if err != nil {
				t.Errorf("failed to get KDM data.json from url %v", tt.args.kdmUrl)
			}
			resp, err := ioutil.ReadAll(get.Body)
			if err != nil {
				t.Errorf("failed to read response from url %v", tt.args.kdmUrl)
			}
			data, err := kdm.FromData(resp)
			if err != nil {
				t.Error(err)
			}
			switch tt.args.source {
			case rke2:
				tt.args.externalData = data.RKE2
			case k3s:
				tt.args.externalData = data.K3S
			}

			// RKE1 does not use a system installer image
			var systemAgentInstallerImage string
			if tt.args.source == rke2 || tt.args.source == k3s {
				systemAgentInstallerImage = fmt.Sprintf("%s%s:%s", "rancher/system-agent-installer-", tt.args.source, strings.ReplaceAll(tt.args.versions[0].VersionString, "+", "-"))
			}

			got, err := GetExternalImagesForVersions(tt.args.source, image.Linux, tt.args.versions)
			if err != nil {
				a.Equal(tt.wantErr, true, "GetExternalImages() errored as expected")
			}
			if !tt.wantErr {
				a.NotEmpty(got)
				a.Contains(got, systemAgentInstallerImage)
				a.Contains(got, tt.args.image1)
				a.Contains(got, tt.args.image2)
				a.Contains(got, tt.args.image3)
			}
		})
	}
}

func Test_downloadExternalImageListFromURL(t *testing.T) {
	type args struct {
		url    string
		image1 string
		image2 string
		image3 string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "k3s-url",
			args: args{
				url:    fmt.Sprintf("https://github.com/k3s-io/k3s/releases/download/%s/k3s-images.txt", k3sWebVersion),
				image1: "rancher/klipper-lb:v0.4.7",
				image2: "rancher/mirrored-pause:3.6",
				image3: "rancher/mirrored-metrics-server:v0.7.0",
			},
		},
		{
			name: "rke2-url-linux",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images-all.linux-amd64.txt", rke2WebVersion),
				image1: "rancher/mirrored-pause:3.6",
				image2: "rancher/rke2-runtime:v1.30.1-rke2r1",
				image3: "rancher/rke2-cloud-provider:v1.29.3-build20240412",
			},
		},
		{
			name: "rke2-url-windows",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images.windows-amd64.txt", rke2WebVersion),
				image1: "docker.io/rancher/rke2-runtime:v1.30.1-rke2r1-windows-amd64",
				image2: "rancher/mirrored-pause:3.6-windows-1809-amd64",
				image3: "rancher/mirrored-pause:3.6-windows-ltsc2022-amd64",
			},
		},
		{
			name: "rancher-url",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rancher/releases/download/v2.6.4/rancher-images.txt"),
				image1: "fleet-agent:v0.3.9",
				image2: "rancher/system-agent-installer-rke2:v1.23.4-rke2r2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := downloadExternalImageListFromURL(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("downloadExternalImageListFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			a.NotEmpty(got)
			a.Contains(got, tt.args.image1)
			a.Contains(got, tt.args.image2)
			a.Contains(got, tt.args.image3)
		})
	}
}

func Test_downloadExternalSupportingImages(t *testing.T) {
	type args struct {
		release string
		source  Source
		os      image.OSType
		image1  string
		image2  string
		image3  string
		image4  string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "k3s-images",
			args: args{
				release: k3sWebVersion,
				source:  k3s,
				os:      image.Linux,
				image1:  "rancher/klipper-lb:v0.4.7",
				image2:  "rancher/mirrored-pause:3.6",
				image3:  "rancher/mirrored-coredns-coredns:1.10.1",
				image4:  "rancher/mirrored-metrics-server:v0.7.0",
			},
		},
		{
			name: "rke2-images-linux",
			args: args{
				release: rke2WebVersion,
				source:  rke2,
				os:      image.Linux,
				image1:  "rancher/harvester-csi-driver:v0.1.6",
				image2:  "rancher/rke2-runtime:v1.30.1-rke2r1",
				image3:  "rancher/rke2-cloud-provider:v1.29.3-build20240412",
			},
		},
		{
			name: "rke2-images-windows",
			args: args{
				release: rke2WebVersion,
				source:  rke2,
				os:      image.Windows,
				image1:  "rancher/rke2-runtime:v1.30.1-rke2r1-windows-amd64",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			got, err := downloadExternalSupportingImages(url.QueryEscape(tt.args.release), tt.args.source, tt.args.os)
			if err != nil {
				t.Errorf("downloadExternalSupportingImages() error = %v, wantErr %v", err, tt.wantErr)
			}
			a.NotEmpty(got)
			a.Contains(got, tt.args.image1)
			a.Contains(got, tt.args.image2)
			a.Contains(got, tt.args.image3)
			a.Contains(got, tt.args.image4)
		})
	}
}
