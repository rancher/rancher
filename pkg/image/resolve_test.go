package image

import (
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

func TestConvertMirroredImages(t *testing.T) {
	testCases := []struct {
		caseName                string
		inputRawImages          map[string]map[string]bool
		outputImagesShouldEqual map[string]map[string]bool
	}{
		{
			caseName: "normalize images",
			inputRawImages: map[string]map[string]bool{
				"rancher/rke-tools:v0.1.48": {"system": true},
				"rancher/rke-tools:v0.1.49": {"system": true},
				// for mirror
				"prom/prometheus:v2.0.1":                           {"system": true},
				"quay.io/coreos/flannel:v1.2.3":                    {"system": true},
				"gcr.io/google_containers/k8s-dns-kube-dns:1.15.0": {"system": true},
				"test.io/test:v0.0.1":                              {"test": true}, // not in mirror list
			},
			outputImagesShouldEqual: map[string]map[string]bool{
				"rancher/coreos-flannel:v1.2.3":   {"system": true},
				"rancher/k8s-dns-kube-dns:1.15.0": {"system": true},
				"rancher/prom-prometheus:v2.0.1":  {"system": true},
				"rancher/rke-tools:v0.1.48":       {"system": true},
				"rancher/rke-tools:v0.1.49":       {"system": true},
				"test.io/test:v0.0.1":             {"test": true},
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
