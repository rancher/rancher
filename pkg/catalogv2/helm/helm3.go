package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"strings"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func isHelm3(labels map[string]string) bool {
	return labels["owner"] == "helm"
}

func fromHelm3Data(data string, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	release, err := decodeHelm3(data)
	if err != nil {
		return nil, err
	}

	return fromHelm3ReleaseToRelease(release, isNamespaced)
}

func fromHelm3ReleaseToRelease(release *release.Release, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	var (
		info  = &v1.Info{}
		chart = &v1.Chart{}
		err   error
	)

	if release.Info != nil {
		info = &v1.Info{
			Description: release.Info.Description,
			Status:      v1.Status(release.Info.Status),
			Notes:       release.Info.Notes,
		}
		if !release.Info.FirstDeployed.IsZero() {
			info.FirstDeployed = &metav1.Time{Time: release.Info.FirstDeployed.Time}
		}
		if !release.Info.LastDeployed.IsZero() {
			info.LastDeployed = &metav1.Time{Time: release.Info.LastDeployed.Time}
		}
		if !release.Info.Deleted.IsZero() {
			info.Deleted = &metav1.Time{Time: release.Info.Deleted.Time}
		}
	}

	if release.Chart != nil {
		chart = &v1.Chart{
			Values: release.Chart.Values,
		}
		if release.Chart.Metadata != nil {
			chart.Metadata = &v1.Metadata{
				Name:        release.Chart.Metadata.Name,
				Home:        release.Chart.Metadata.Home,
				Sources:     release.Chart.Metadata.Sources,
				Version:     release.Chart.Metadata.Version,
				Description: release.Chart.Metadata.Description,
				Keywords:    release.Chart.Metadata.Keywords,
				Icon:        release.Chart.Metadata.Icon,
				APIVersion:  release.Chart.Metadata.APIVersion,
				Condition:   release.Chart.Metadata.Condition,
				Tags:        release.Chart.Metadata.Tags,
				AppVersion:  release.Chart.Metadata.AppVersion,
				Deprecated:  release.Chart.Metadata.Deprecated,
				Annotations: release.Chart.Metadata.Annotations,
				KubeVersion: release.Chart.Metadata.KubeVersion,
				Type:        release.Chart.Metadata.Type,
			}

			for _, m := range release.Chart.Metadata.Maintainers {
				if m == nil {
					continue
				}
				chart.Metadata.Maintainers = append(chart.Metadata.Maintainers, v1.Maintainer{
					Name:  m.Name,
					Email: m.Email,
					URL:   m.URL,
				})
			}
		}

		for _, f := range release.Chart.Files {
			if f == nil {
				continue
			}
			if readmes[strings.ToLower(f.Name)] {
				info.Readme = string(f.Data)
			}
		}
	}

	hr := &v1.ReleaseSpec{
		Name:             release.Name,
		Info:             info,
		Chart:            chart,
		Values:           release.Config,
		Resources:        nil,
		Version:          release.Version,
		Namespace:        release.Namespace,
		HelmMajorVersion: 3,
	}

	reg := settings.SystemDefaultRegistry.Get()
	if reg != "" {
		hr.Values["systemDefaultRegistry"] = reg
	}

	hr.Resources, err = resourcesFromManifest(release.Namespace, release.Manifest, isNamespaced)
	return hr, err
}

func decodeHelm3(data string) (*release.Release, error) {
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		b2, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls release.Release
	// unmarshal release object bytes
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}
