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
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rspb "k8s.io/helm/pkg/proto/hapi/release"
	"sigs.k8s.io/yaml"
)

var (
	readmes = map[string]bool{
		"readme":     true,
		"readme.txt": true,
		"readme.md":  true,
	}
	statusMapping = map[string]v1.Status{
		"UNKNOWN":          v1.StatusUnknown,
		"DEPLOYED":         v1.StatusDeployed,
		"DELETED":          v1.StatusUninstalled,
		"SUPERSEDED":       v1.StatusSuperseded,
		"FAILED":           v1.StatusFailed,
		"DELETING":         v1.StatusUninstalling,
		"PENDING_INSTALL":  v1.StatusPendingInstall,
		"PENDING_UPGRADE":  v1.StatusPendingUpgrade,
		"PENDING_ROLLBACK": v1.StatusPendingRollback,
	}
)

func isHelm2(labels map[string]string) bool {
	return labels["OWNER"] == "TILLER"
}

func fromHelm2Data(data string, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	release, err := decodeHelm2(data)
	if err != nil {
		return nil, err
	}

	return fromHelm2ReleaseToRelease(release, isNamespaced)
}

func toTime(t *timestamp.Timestamp) *metav1.Time {
	if t == nil || (t.Seconds == 0 && t.Nanos == 0) {
		return nil
	}
	return &metav1.Time{
		Time: time.Unix(t.GetSeconds(), int64(t.GetNanos())).UTC(),
	}
}

func fromHelm2ReleaseToRelease(release *rspb.Release, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	var (
		err error
	)

	hr := &v1.ReleaseSpec{
		Name: release.Name,
		Info: &v1.Info{
			FirstDeployed: toTime(release.GetInfo().GetFirstDeployed()),
			LastDeployed:  toTime(release.GetInfo().GetLastDeployed()),
			Deleted:       toTime(release.GetInfo().GetDeleted()),
			Description:   release.GetInfo().GetDescription(),
			Status:        statusMapping[release.GetInfo().GetStatus().GetCode().String()],
			Notes:         release.GetInfo().GetStatus().GetNotes(),
		},
		Chart: &v1.Chart{
			Values: toMap(release.Namespace, release.Name, release.GetChart().GetValues().GetRaw()),
			Metadata: &v1.Metadata{
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
		hr.Chart.Metadata.Maintainers = append(hr.Chart.Metadata.Maintainers, v1.Maintainer{
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
