package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/sirupsen/logrus"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"sigs.k8s.io/yaml"
)

var (
	readmes = map[string]bool{
		"readme":     true,
		"readme.txt": true,
		"readme.md":  true,
	}
	statusMapping = map[string]Status{
		"UNKNOWN":          StatusUnknown,
		"DEPLOYED":         StatusDeployed,
		"DELETED":          StatusUninstalled,
		"SUPERSEDED":       StatusSuperseded,
		"FAILED":           StatusFailed,
		"DELETING":         StatusUninstalling,
		"PENDING_INSTALL":  StatusPendingInstall,
		"PENDING_UPGRADE":  StatusPendingUpgrade,
		"PENDING_ROLLBACK": StatusPendingRollback,
	}
)

func isHelm2(labels map[string]string) bool {
	return labels["OWNER"] == "TILLER"
}

func fromHelm2Data(data string, isNamespaced IsNamespaced) (*Release, error) {
	release, err := decodeHelm2(data)
	if err != nil {
		return nil, err
	}

	return fromHelm2ReleaseToRelease(release, isNamespaced)
}

func toTime(t *timestamp.Timestamp) time.Time {
	return time.Unix(t.GetSeconds(), int64(t.GetNanos())).UTC()
}

func fromHelm2ReleaseToRelease(release *rspb.Release, isNamespaced IsNamespaced) (*Release, error) {
	var (
		err error
	)

	hr := &Release{
		Name: release.Name,
		Info: &Info{
			FirstDeployed: toTime(release.GetInfo().GetFirstDeployed()),
			LastDeployed:  toTime(release.GetInfo().GetLastDeployed()),
			Deleted:       toTime(release.GetInfo().GetDeleted()),
			Description:   release.GetInfo().GetDescription(),
			Status:        statusMapping[release.GetInfo().GetStatus().GetCode().String()],
			Notes:         release.GetInfo().GetStatus().GetNotes(),
		},
		Chart: &Chart{
			Values: toMap(release.Namespace, release.Name, release.GetChart().GetValues().GetRaw()),
			Metadata: &Metadata{
				Name:        release.GetChart().GetMetadata().GetName(),
				Home:        release.GetChart().GetMetadata().GetHome(),
				Sources:     release.GetChart().GetMetadata().GetSources(),
				Version:     release.GetChart().GetMetadata().GetVersion(),
				Description: release.GetChart().GetMetadata().GetDescription(),
				Keywords:    release.GetChart().GetMetadata().GetKeywords(),
				Icon:        release.GetChart().GetMetadata().GetIcon(),
				Condition:   release.GetChart().GetMetadata().GetCondition(),
				Tags:        release.GetChart().GetMetadata().GetTags(),
				AppVersion:  release.GetChart().GetMetadata().GetAppVersion(),
				Deprecated:  release.GetChart().GetMetadata().GetDeprecated(),
				Annotations: release.GetChart().GetMetadata().GetAnnotations(),
				KubeVersion: release.GetChart().GetMetadata().GetKubeVersion(),
			},
		},
		Values:           toMap(release.Namespace, release.Name, release.GetConfig().GetRaw()),
		Version:          int(release.Version),
		Namespace:        release.Namespace,
		HelmMajorVersion: 3,
	}

	for _, m := range release.GetChart().GetMetadata().GetMaintainers() {
		if m == nil {
			continue
		}
		hr.Chart.Metadata.Maintainers = append(hr.Chart.Metadata.Maintainers, Maintainer{
			Name:  m.GetName(),
			Email: m.GetEmail(),
			URL:   m.GetUrl(),
		})
	}

	for _, f := range release.GetChart().GetFiles() {
		if f == nil {
			continue
		}
		if readmes[strings.ToLower(f.TypeUrl)] {
			hr.Info.Readme = string(f.Value)
		}
	}

	hr.Resources, err = resourcesFromManifest(release.Namespace, release.Manifest, isNamespaced)
	return hr, err
}

func toMap(namespace, name string, manifest string) map[string]interface{} {
	values := map[string]interface{}{}

	if manifest == "" {
		return values
	}

	if err := yaml.Unmarshal([]byte(manifest), &values); err != nil {
		logrus.Errorf("failed to unmarshal yaml for %s/%s", namespace, name)
	}

	return values
}

func decodeHelm2(data string) (*rspb.Release, error) {
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

	var rls rspb.Release
	// unmarshal protobuf bytes
	if err := proto.Unmarshal(b, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}
