package helm

import (
	"bytes"
	"encoding/base64"
	"errors"

	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ErrNotHelmRelease = errors.New("not helm release")
	magicGzip         = []byte{0x1f, 0x8b, 0x08}
)

type IsNamespaced func(gvk schema.GroupVersionKind) bool

func ToRelease(obj runtime.Object, isNamespaced IsNamespaced) (*Release, error) {
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

func getReleaseDataAndKind(obj runtime.Object) (string, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
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
		return t.Data["release"], nil
	case *corev1.Secret:
		return string(t.Data["release"]), nil
	}

	return "", ErrNotHelmRelease
}

func resourcesFromManifest(namespace string, manifest string, isNamespaced IsNamespaced) (result []Resource, err error) {
	objs, err := yaml.ToObjects(bytes.NewReader([]byte(manifest)))
	if err != nil {
		return nil, err
	}

	for _, obj := range objs {
		meta, err := meta2.Accessor(obj)
		if err != nil {
			return nil, err
		}
		r := Resource{
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
