package image

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rketypes "github.com/rancher/rke/types"
	assertlib "github.com/stretchr/testify/assert"
)

func TestFetchImagesFromSystem(t *testing.T) {
	k8sVersion := "v1.17.9-rancher1-2"
	linuxRKESystemImages := map[string]rketypes.RKESystemImages{
		k8sVersion: {
			NginxProxy: "rancher/rke-tools:v0.1.63",
			CoreDNS:    "rancher/coredns-coredns:1.6.5",
		},
	}
	windowsRKESystemImages := map[string]rketypes.RKESystemImages{
		k8sVersion: {
			NginxProxy:               "rancher/rke-tools:v0.1.63",
			WindowsPodInfraContainer: "rancher/kubelet-pause:v0.1.4",
		},
	}
	toolsSystemImages := v32.ToolsSystemImages
	bothImages := []string{
		linuxRKESystemImages[k8sVersion].NginxProxy,
	}
	linuxImagesOnly := []string{
		linuxRKESystemImages[k8sVersion].CoreDNS,
		toolsSystemImages.PipelineSystemImages.Jenkins, // from tools
	}
	windowsImagesOnly := []string{
		windowsRKESystemImages[k8sVersion].WindowsPodInfraContainer,
	}

	testCases := []struct {
		caseName                  string
		inputRkeSystemImages      map[string]rketypes.RKESystemImages
		inputOsType               OSType
		outputShouldContainImages []string
		outputShouldNotContain    []string
	}{
		{
			caseName:             "fetch linux images from system images",
			inputRkeSystemImages: linuxRKESystemImages,
			inputOsType:          Linux,
			outputShouldContainImages: flatStringSlice(
				bothImages,
				linuxImagesOnly,
			),
			outputShouldNotContain: windowsImagesOnly,
		},
		{
			caseName:             "fetch windows images from system images",
			inputRkeSystemImages: windowsRKESystemImages,
			inputOsType:          Windows,
			outputShouldContainImages: flatStringSlice(
				bothImages,
				windowsImagesOnly,
			),
			outputShouldNotContain: linuxImagesOnly,
		},
	}

	assert := assertlib.New(t)

	for _, cs := range testCases {
		imagesSet := make(map[string]map[string]struct{})
		exportConfig := ExportConfig{
			OsType: cs.inputOsType,
		}
		systemExport := System{exportConfig}
		err := systemExport.FetchImages(cs.inputRkeSystemImages, imagesSet)
		images, imageSources := getImagesAndSourcesLists(imagesSet)
		assert.Nilf(err, "%s, failed to fetch images from system images", cs.caseName)
		assert.Subset(images, cs.outputShouldContainImages, cs.caseName)
		for _, nc := range cs.outputShouldNotContain {
			assert.NotContains(images, nc, cs.caseName)
		}
		for _, source := range imageSources {
			assert.Equal("system", source)
		}
	}
}

func getImagesAndSourcesLists(imagesSet map[string]map[string]struct{}) ([]string, []string) {
	var images, imageSources []string
	for image, sources := range imagesSet {
		images = append(images, image)
		for source := range sources {
			imageSources = append(imageSources, source)
		}
	}
	return images, imageSources
}

func flatStringSlice(slices ...[]string) []string {
	var ret []string
	for _, s := range slices {
		ret = append(ret, s...)
	}
	return ret
}
