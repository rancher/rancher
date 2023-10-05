package helm

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strconv"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"

	"github.com/rancher/wrangler/v2/pkg/data"
	"github.com/rancher/wrangler/v2/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ErrNotHelmRelease = errors.New("not helm release") // error for when it's not a helm release
	magicGzip         = []byte{0x1f, 0x8b, 0x08}       // gzip magic header
)

// IsNamespaced Alias for func(gvk schema.GroupVersionKind) bool
type IsNamespaced func(gvk schema.GroupVersionKind) bool

// IsLatest checked if the received v1.ReleaseSpec is on the latest version of a helm chart
// by comparing its version to all release helm objects.
func IsLatest(release *v1.ReleaseSpec, others []runtime.Object) bool {
	for _, other := range others {
		m, err := meta.Accessor(other)
		if err != nil {
			continue
		}
		labels := m.GetLabels()
		name := labels["name"]
		if name == "" {
			name = labels["NAME"]
		}
		if name != release.Name {
			continue
		}

		version := labels["version"]
		if version == "" {
			version = labels["VERSION"]
		}

		v, err := strconv.Atoi(version)
		if err != nil {
			continue
		}

		if v > release.Version {
			return false
		}
	}

	return true
}

// ToRelease return a v1.ReleaseSpec for the given runtime.Object received.
// The function populates the releaseSpec with release information related
// to an installed app based on the helm version used to install the app.
func ToRelease(obj runtime.Object, isNamespaced IsNamespaced) (*v1.ReleaseSpec, error) {
	releaseData, err := getReleaseDataAndKind(obj)
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	switch {
	case isHelm3(meta.GetLabels()):
		return fromHelm3Data(releaseData, isNamespaced)
	case isHelm2(meta.GetLabels()):
		return fromHelm2Data(releaseData, isNamespaced)
	}

	return nil, ErrNotHelmRelease
}

// getReleaseDataAndKind receives a runtime.Object which can be an
// unstructured.Unstructured, corev1.ConfigMap or a corev1.Secret.
// It extracts the data["release"] based on the object type and returns it.
func getReleaseDataAndKind(obj runtime.Object) (string, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		if t == nil {
			return "", ErrNotHelmRelease
		}
		releaseData := data.Object(t.Object).String("data", "release")
		switch t.GetKind() {
		case "ConfigMap":
			return releaseData, nil
		case "Secret":
			data, err := base64.StdEncoding.DecodeString(releaseData)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
	case *corev1.ConfigMap:
		if t == nil {
			return "", ErrNotHelmRelease
		}
		return t.Data["release"], nil
	case *corev1.Secret:
		if t == nil {
			return "", ErrNotHelmRelease
		}
		return string(t.Data["release"]), nil
	}

	return "", ErrNotHelmRelease
}

// resourcesFromManifest receives the rendered manifest template as a string and
// uses the wrangler yaml functions to convert it to a list of runtime.Objects.
// It then converts each object to a v1.ReleaseResource and returns that list.
func resourcesFromManifest(namespace string, manifest string, isNamespaced IsNamespaced) (result []v1.ReleaseResource, err error) {
	objs, err := yaml.ToObjects(bytes.NewReader([]byte(manifest)))
	if err != nil {
		return nil, err
	}

	for _, obj := range objs {
		meta, err := meta2.Accessor(obj)
		if err != nil {
			return nil, err
		}
		r := v1.ReleaseResource{
			Name:      meta.GetName(),
			Namespace: meta.GetNamespace(),
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		if isNamespaced != nil && isNamespaced(gvk) && r.Namespace == "" {
			r.Namespace = namespace
		}
		r.APIVersion, r.Kind = gvk.ToAPIVersionAndKind()
		result = append(result, r)
	}

	return result, nil
}
