package external

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rke/types/kdm"
	"github.com/stretchr/testify/assert"
)

const (
	k3s            = "k3s"
	rancherVersion = "v2.6.4"
	k3sWebVersion  = "v1.23.6+k3s1"
	rke2WebVersion = "v1.23.6+rke2r1"
	rke2           = "rke2"
	rke1           = "rke1"
	devKDM         = "https://github.com/rancher/kontainer-driver-metadata/raw/dev-v2.6/data/data.json"
	releaseKDM     = "https://releases.rancher.com/kontainer-driver-metadata/release-v2.6/data.json"
)

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
		minimumKubernetesVersion *semver.Version
		kdmUrl                   string
		image1                   string
		image2                   string
		image3                   string
		version                  string
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
				rancherVersion:           rancherVersion,
				externalData:             map[string]interface{}{},
				source:                   k3s,
				version:                  k3sWebVersion,
				minimumKubernetesVersion: kubeSemVer,
				kdmUrl:                   devKDM,
				image1:                   "rancher/klipper-lb:v0.3.5",
				image2:                   "rancher/mirrored-pause:3.6",
				image3:                   "rancher/mirrored-metrics-server:v0.5.2",
			},
			wantErr: false,
		},
		{
			name: "rke2-test",
			args: args{
				rancherVersion:           rancherVersion,
				externalData:             map[string]interface{}{},
				source:                   rke2,
				version:                  rke2WebVersion,
				minimumKubernetesVersion: kubeSemVer,
				kdmUrl:                   releaseKDM,
				image1:                   "rancher/pause:3.6",
				image2:                   "rancher/rke2-runtime:v1.23.6-rke2r1",
				image3:                   "rancher/rke2-cloud-provider:v0.0.3-build20211118",
			},
			wantErr: false,
		},
		{
			name: "rke1-test-fail",
			args: args{
				rancherVersion:           rancherVersion,
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
			systemAgentInstallerImage := fmt.Sprintf("%s%s:%s", "rancher/system-agent-installer-", tt.args.source, strings.ReplaceAll(tt.args.version, "+", "-"))

			got, err := GetExternalImages(tt.args.rancherVersion, tt.args.externalData, tt.args.source, tt.args.minimumKubernetesVersion, image.Linux)
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
