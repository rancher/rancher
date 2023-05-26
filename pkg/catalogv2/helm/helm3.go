/*
Package helm implements ways of extracting information from helm2 and helm3 data and making a k8s releaseSpec.
It also implements a partition.Partition to handle the resources needed by the release.
*/
package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"

	"helm.sh/helm/v3/pkg/release"
)

// isHelm3 checks if the value of the owner key of the received map is equal to helm.
// Every helm3 release object has this particular label on it.
func isHelm3(labels map[string]string) bool {
	return labels["owner"] == "helm"
}

// fromHelm3Data receives a helm3 release data string of an installed helm chart.
// It then converts the string into Helm3 release struct and again to rancher
// v1.ReleaseSpec struct to return it.
func fromHelm3Data(data string, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	release, err := decodeHelm3(data)
	if err != nil {
		return nil, err
	}

	return fromHelm3ReleaseToRelease(release, isNamespaced)
}

// fromHelm3ReleaseToRelease receives a helm3 release struct.
// Returns a pointer to a rancher v1.ReleaseSpec struct constructed from the helm3 release struct.
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

	hr.Resources, err = resourcesFromManifest(release.Namespace, release.Manifest, isNamespaced)
	return hr, err
}

// decodeHelm3 receives a helm3 release data string, decodes the string data using the standard base64 library
// and unmarshals the data into release.Release struct to return it.
func decodeHelm3(data string) (*release.Release, error) {
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// Data is too small to be helm 3 release object
	if len(b) <= 3 {
		return nil, ErrNotHelmRelease
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
