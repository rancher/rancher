package external

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/rancher/rancher/pkg/image"
	"github.com/stretchr/testify/assert"
)

const (
	k3s            = "k3s"
	rancherVersion = "v2.6.4"
	k3sWebVersion  = "v1.23.6+k3s1"
	rke2WebVersion = "v1.23.6+rke2r1"
	rke2           = "rke2"
	rke1           = "rke1"
)

// makeRelease creates a KDM release map entry with the given k8s version and
// rancher-compatibility bounds. The min/max values are wide enough that any
// testRancherVersion caller is considered compatible.
func makeRelease(version string) map[string]interface{} {
	return map[string]interface{}{
		"version":                 version,
		"minChannelServerVersion": "v2.14.0",
		"maxChannelServerVersion": "v2.16.0",
	}
}

// makeExternalData wraps a list of release versions into the map[string]interface{}
// shape expected by GetExternalImages (i.e. {"releases": [...]}).
func makeExternalData(versions ...string) map[string]interface{} {
	releases := make([]interface{}, len(versions))
	for i, v := range versions {
		releases[i] = makeRelease(v)
	}
	return map[string]interface{}{"releases": releases}
}

func TestGetExternalImages(t *testing.T) {
	// testRancherVersion is within the [v2.14.0, v2.16.0] band used by makeRelease.
	const testRancherVersion = "v2.15.0"

	kubeSemVer := &semver.Version{
		Major: 1,
		Minor: 32,
		Patch: 0,
	}

	tests := []struct {
		name           string
		rancherVersion string
		externalData   map[string]interface{}
		source         Source
		wantImages     []string
		dontWantImages []string
		wantErr        bool
	}{
		// Data set 1: two different patch versions for the same minor.
		// Expectation: only the latest patch (v1.35.2) survives filtering.
		{
			name:           "k3s: keeps latest patch per minor",
			rancherVersion: testRancherVersion,
			externalData:   makeExternalData("v1.35.1+k3s1", "v1.35.2+k3s1"),
			source:         K3S,
			wantImages:     []string{"rancher/system-agent-installer-k3s:v1.35.2-k3s1", "rancher/k3s-upgrade:v1.35.2-k3s1"},
			dontWantImages: []string{"rancher/system-agent-installer-k3s:v1.35.1-k3s1", "rancher/k3s-upgrade:v1.35.1-k3s1"},
		},
		{
			name:           "rke2: keeps latest patch per minor",
			rancherVersion: testRancherVersion,
			externalData:   makeExternalData("v1.35.1+rke2r1", "v1.35.2+rke2r1"),
			source:         RKE2,
			wantImages:     []string{"rancher/system-agent-installer-rke2:v1.35.2-rke2r1", "rancher/rke2-upgrade:v1.35.2-rke2r1"},
			dontWantImages: []string{"rancher/system-agent-installer-rke2:v1.35.1-rke2r1", "rancher/rke2-upgrade:v1.35.1-rke2r1"},
		},
		// Data set 2: same patch version, two different build numbers.
		// Expectation: only the highest build number (k3s2 / rke2r2) survives filtering.
		{
			name:           "k3s: keeps highest build number for same patch",
			rancherVersion: testRancherVersion,
			externalData:   makeExternalData("v1.35.2+k3s1", "v1.35.2+k3s2"),
			source:         K3S,
			wantImages:     []string{"rancher/system-agent-installer-k3s:v1.35.2-k3s2", "rancher/k3s-upgrade:v1.35.2-k3s2"},
			dontWantImages: []string{"rancher/system-agent-installer-k3s:v1.35.2-k3s1", "rancher/k3s-upgrade:v1.35.2-k3s1"},
		},
		{
			name:           "rke2: keeps highest build number for same patch",
			rancherVersion: testRancherVersion,
			externalData:   makeExternalData("v1.35.2+rke2r1", "v1.35.2+rke2r2"),
			source:         RKE2,
			wantImages:     []string{"rancher/system-agent-installer-rke2:v1.35.2-rke2r2", "rancher/rke2-upgrade:v1.35.2-rke2r2"},
			dontWantImages: []string{"rancher/system-agent-installer-rke2:v1.35.2-rke2r1", "rancher/rke2-upgrade:v1.35.2-rke2r1"},
		},
		{
			name:           "invalid source returns error",
			rancherVersion: testRancherVersion,
			externalData:   map[string]interface{}{},
			source:         rke1,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := GetExternalImages(tt.rancherVersion, tt.externalData, tt.source, kubeSemVer, image.Linux)
			if tt.wantErr {
				a.Error(err)
				return
			}
			a.NoError(err)
			a.NotEmpty(got)
			for _, img := range tt.wantImages {
				a.Contains(got, img)
			}
			for _, img := range tt.dontWantImages {
				a.NotContains(got, img)
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
				image1: "rancher/klipper-lb:v0.3.5",
				image2: "rancher/mirrored-pause:3.6",
				image3: "rancher/mirrored-metrics-server:v0.5.2",
			},
		},
		{
			name: "rke2-url-linux",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images-all.linux-amd64.txt", rke2WebVersion),
				image1: "rancher/pause:3.6",
				image2: "rancher/rke2-runtime:v1.23.6-rke2r1",
				image3: "rancher/rke2-cloud-provider:v0.0.3-build20211118",
			},
		},
		{
			name: "rke2-url-windows",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images.windows-amd64.txt", rke2WebVersion),
				image1: "docker.io/rancher/rke2-runtime:v1.23.6-rke2r1-windows-amd64",
				image2: "rancher/pause:3.6-windows-1809-amd64",
				image3: "rancher/pause:3.6-windows-ltsc2022-amd64",
			},
		},
		{
			name: "rancher-url",
			args: args{
				url:    fmt.Sprintf("https://github.com/rancher/rancher/releases/download/%s/rancher-images.txt", rancherVersion),
				image1: "fleet-agent:v0.3.9",
				image2: "rancher/system-agent-installer-rke2:v1.23.4-rke2r2",
				image3: "rancher/rancher-agent:" + rancherVersion,
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
				image1:  "rancher/klipper-lb:v0.3.5",
				image2:  "rancher/mirrored-pause:3.6",
				image3:  "rancher/mirrored-coredns-coredns:1.9.1",
				image4:  "rancher/mirrored-metrics-server:v0.5.2",
			},
		},
		{
			name: "rke2-images-linux",
			args: args{
				release: rke2WebVersion,
				source:  rke2,
				os:      image.Linux,
				image1:  "rancher/harvester-csi-driver:v0.1.3",
				image2:  "rancher/rke2-runtime:v1.23.6-rke2r1",
				image3:  "rancher/rke2-cloud-provider:v0.0.3-build20211118",
			},
		},
		{
			name: "rke2-images-windows",
			args: args{
				release: rke2WebVersion,
				source:  rke2,
				os:      image.Windows,
				image1:  "rancher/rke2-runtime:v1.23.6-rke2r1-windows-amd64",
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

func Test_filterLatestPatchReleases(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "keeps latest patch per minor",
			input: []string{"v1.28.1+k3s1", "v1.28.2+k3s1", "v1.28.3+k3s1"},
			want:  []string{"v1.28.3+k3s1"},
		},
		{
			name:  "keeps one entry per minor",
			input: []string{"v1.28.3+k3s1", "v1.29.2+k3s1", "v1.30.0+k3s1"},
			want:  []string{"v1.28.3+k3s1", "v1.29.2+k3s1", "v1.30.0+k3s1"},
		},
		{
			name:  "same patch prefers higher build number",
			input: []string{"v1.28.3+k3s1", "v1.28.3+k3s2"},
			want:  []string{"v1.28.3+k3s2"},
		},
		{
			name:  "rke2 versions",
			input: []string{"v1.29.1+rke2r1", "v1.29.2+rke2r1", "v1.30.1+rke2r1", "v1.30.1+rke2r2"},
			want:  []string{"v1.29.2+rke2r1", "v1.30.1+rke2r2"},
		},
		{
			name:  "mixed minor versions across k8s releases",
			input: []string{"v1.27.10+k3s1", "v1.28.5+k3s1", "v1.28.4+k3s1", "v1.29.0+k3s1"},
			want:  []string{"v1.27.10+k3s1", "v1.28.5+k3s1", "v1.29.0+k3s1"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "unparseable versions are skipped",
			input: []string{"not-a-version", "v1.28.3+k3s1"},
			want:  []string{"v1.28.3+k3s1"},
		},
		{
			name:  "RC versions are skipped entirely",
			input: []string{"v1.33.3-rc.1+rke2r1", "v1.33.3+rke2r1"},
			want:  []string{"v1.33.3+rke2r1"},
		},
		{
			name:  "all-RC input yields empty result",
			input: []string{"v1.33.3-rc.1+rke2r1", "v1.33.3-rc.2+rke2r2"},
			want:  []string{},
		},
		{
			name:  "RC patch is skipped, lower non-RC patch is kept",
			input: []string{"v1.33.3+rke2r1", "v1.33.4-rc.1+rke2r1"},
			want:  []string{"v1.33.3+rke2r1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			got := filterLatestPatchReleases(tt.input)
			a.ElementsMatch(tt.want, got)
		})
	}
}

func Test_extractBuildNumber(t *testing.T) {
	tests := []struct {
		metadata string
		want     int
	}{
		{"k3s1", 1},
		{"k3s2", 2},
		{"k3s10", 10},
		{"rke2r1", 1},
		{"rke2r2", 2},
		{"", 0},
		{"nodigits", 0},
	}

	for _, tt := range tests {
		t.Run(tt.metadata, func(t *testing.T) {
			assert.Equal(t, tt.want, extractBuildNumber(tt.metadata))
		})
	}
}
