package image

import (
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

func TestConvertMirroredImages(t *testing.T) {
	testCases := []struct {
		caseName                string
		inputRawImages          map[string]map[string]struct{}
		outputImagesShouldEqual map[string]map[string]struct{}
	}{
		{
			caseName: "normalize images",
			inputRawImages: map[string]map[string]struct{}{
				"rancher/rke-tools:v0.1.48": {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49": {"system": struct{}{}},
				// for mirror
				"prom/prometheus:v2.0.1":                           {"system": struct{}{}},
				"quay.io/coreos/flannel:v1.2.3":                    {"system": struct{}{}},
				"gcr.io/google_containers/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"test.io/test:v0.0.1":                              {"test": struct{}{}}, // not in mirror list
			},
			outputImagesShouldEqual: map[string]map[string]struct{}{
				"rancher/coreos-flannel:v1.2.3":   {"system": struct{}{}},
				"rancher/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"rancher/prom-prometheus:v2.0.1":  {"system": struct{}{}},
				"rancher/rke-tools:v0.1.48":       {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49":       {"system": struct{}{}},
				"test.io/test:v0.0.1":             {"test": struct{}{}},
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		imagesSet := cs.inputRawImages
		convertMirroredImages(imagesSet)
		assert.Equal(cs.outputImagesShouldEqual, imagesSet)
	}
}
