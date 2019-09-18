package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"
	"time"

	metadata "github.com/rancher/kontainer-driver-metadata/rke"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	assertlib "github.com/stretchr/testify/assert"
)

const (
	testRancherVersion    = "v1.15.4-rancher1-1"
	testSystemChartBranch = "dev"
	testSystemChartCommit = "7c183b98ceb8a7a50892cdb9a991fe627a782fe6"
)

func TestFetchImagesFromCharts(t *testing.T) {
	systemChartPath := cloneTestSystemChart(t)

	testCases := []struct {
		caseName               string
		inputPath              string
		inputOsType            OSType
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		{
			caseName:    "fetch linux images from charts",
			inputPath:   systemChartPath,
			inputOsType: Linux,
			outputShouldContain: []string{
				"rancher/prom-alertmanager:v0.17.0",
				"rancher/configmap-reload:v0.3.0-rancher2", // both windows and linux
				"rancher/fluentd:v0.1.16",                  // both windows and linux
			},
			outputShouldNotContain: []string{
				"rancher/wmi_exporter-package:v0.0.2",
			},
		},
		{
			caseName:    "fetch windows images from charts",
			inputPath:   systemChartPath,
			inputOsType: Windows,
			outputShouldContain: []string{
				"rancher/wmi_exporter-package:v0.0.2",
				"rancher/configmap-reload:v0.3.0-rancher2", // both windows and linux
				"rancher/fluentd:v0.1.16",                  // both windows and linux
			},
			outputShouldNotContain: []string{
				"rancher/prom-alertmanager:v0.17.0",
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		images, err := fetchImagesFromCharts(cs.inputPath, cs.inputOsType)
		assert.Nilf(err, "%s, failed to fetch images from charts", cs.caseName)
		if len(cs.outputShouldContain) > 0 {
			assert.Subset(images, cs.outputShouldContain, cs.caseName)
		}
		if len(cs.outputShouldNotContain) > 0 {
			for _, nc := range cs.outputShouldNotContain {
				assert.NotContains(images, nc, cs.caseName)
			}
		}
	}
}

func TestFetchImagesFromSystem(t *testing.T) {
	linuxInfo, windowsInfo := getTestK8sVersionInfo()
	toolsSystemImages := v3.ToolsSystemImages
	rkeSystemImages := getTestRKESystemImages()

	testCases := []struct {
		caseName               string
		inputRkeSystemImages   map[string]v3.RKESystemImages
		inputOsType            OSType
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		{
			caseName:             "fetch linux images from system images",
			inputRkeSystemImages: linuxInfo.RKESystemImages,
			inputOsType:          Linux,
			outputShouldContain: []string{
				toolsSystemImages.PipelineSystemImages.Jenkins,
				toolsSystemImages.PipelineSystemImages.JenkinsJnlp,
			},
			outputShouldNotContain: []string{
				rkeSystemImages.WindowsPodInfraContainer,
			},
		},
		{
			caseName:             "fetch windows images from system images",
			inputRkeSystemImages: windowsInfo.RKESystemImages,
			inputOsType:          Windows,
			outputShouldContain: []string{
				rkeSystemImages.WindowsPodInfraContainer,
				rkeSystemImages.NginxProxy,
			},
			outputShouldNotContain: []string{
				toolsSystemImages.PipelineSystemImages.Jenkins,
				toolsSystemImages.PipelineSystemImages.JenkinsJnlp,
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		images, err := fetchImagesFromSystem(cs.inputRkeSystemImages, cs.inputOsType)
		assert.Nilf(err, "%s, failed to fetch images from system images", cs.caseName)
		if len(cs.outputShouldContain) > 0 {
			assert.Subset(images, cs.outputShouldContain, cs.caseName)
		}
		if len(cs.outputShouldNotContain) > 0 {
			for _, nc := range cs.outputShouldNotContain {
				assert.NotContains(images, nc, cs.caseName)
			}
		}
	}
}

func TestNormalizeImages(t *testing.T) {
	testCases := []struct {
		caseName          string
		inputRawImages    []string
		outputShouldEqual []string
	}{
		{
			caseName: "normalize images",
			inputRawImages: []string{
				// for sort
				"rancher/rke-tools:v0.1.48",
				// for unique
				"rancher/rke-tools:v0.1.49",
				"rancher/rke-tools:v0.1.49",
				"rancher/rke-tools:v0.1.49",
				// for mirror
				"prom/prometheus:v2.0.1",
				"quay.io/coreos/flannel:v1.2.3",
				"gcr.io/google_containers/k8s-dns-kube-dns:1.15.0",
				"test.io/test:v0.0.1", // not in mirror list
			},
			outputShouldEqual: []string{
				"rancher/coreos-flannel:v1.2.3",
				"rancher/k8s-dns-kube-dns:1.15.0",
				"rancher/prom-prometheus:v2.0.1",
				"rancher/rke-tools:v0.1.48",
				"rancher/rke-tools:v0.1.49",
				"test.io/test:v0.0.1",
			},
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		images := normalizeImages(cs.inputRawImages)
		if len(cs.outputShouldEqual) > 0 {
			assert.Equal(cs.outputShouldEqual, images, cs.caseName)
		}
	}
}

func TestGetImages(t *testing.T) {
	systemChartPath := cloneTestSystemChart(t)
	linuxInfo, windowsInfo := getTestK8sVersionInfo()
	toolsSystemImages := v3.ToolsSystemImages
	rkeSystemImages := getTestRKESystemImages()

	testCases := []struct {
		caseName               string
		inputSystemChartPath   string
		inputImagesFromArgs    []string
		inputRkeSystemImages   map[string]v3.RKESystemImages
		inputOsType            OSType
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		{
			caseName:             "linux",
			inputSystemChartPath: systemChartPath,
			inputImagesFromArgs: []string{
				"rancher/rancher:master-head",
				"rancher/rancher-agent:master-head",
			},
			inputRkeSystemImages: linuxInfo.RKESystemImages,
			inputOsType:          Linux,
			outputShouldContain: flatStringSlice(
				[]string{
					toolsSystemImages.PipelineSystemImages.Jenkins,
					toolsSystemImages.PipelineSystemImages.JenkinsJnlp,
				},
				getRequirementImages(Linux),
			),
			outputShouldNotContain: []string{
				rkeSystemImages.WindowsPodInfraContainer,
				"rancher/wmi_exporter-package:v0.0.2",
			},
		},
		{
			caseName:             "windows",
			inputSystemChartPath: systemChartPath,
			inputImagesFromArgs: []string{
				"rancher/rancher-agent:master-head",
			},
			inputRkeSystemImages: windowsInfo.RKESystemImages,
			inputOsType:          Windows,
			outputShouldContain: []string{
				rkeSystemImages.WindowsPodInfraContainer,
				rkeSystemImages.NginxProxy,
				"rancher/wmi_exporter-package:v0.0.2",
			},
			outputShouldNotContain: flatStringSlice(
				[]string{
					toolsSystemImages.PipelineSystemImages.Jenkins,
					toolsSystemImages.PipelineSystemImages.JenkinsJnlp,
				},
				getRequirementImages(Windows),
			),
		},
	}

	assert := assertlib.New(t)
	for _, cs := range testCases {
		images, err := GetImages(cs.inputSystemChartPath, cs.inputImagesFromArgs, cs.inputRkeSystemImages, cs.inputOsType)
		assert.Nilf(err, "%s, failed to get images", cs.caseName)
		if len(cs.outputShouldContain) > 0 {
			assert.Subset(images, cs.outputShouldContain, cs.caseName)
		}
		if len(cs.outputShouldNotContain) > 0 {
			for _, nc := range cs.outputShouldNotContain {
				assert.NotContains(images, nc, cs.caseName)
			}
		}
	}
}

func getTestK8sVersionInfo() (linuxInfo, windowsInfo *kd.VersionInfo) {
	return kd.GetK8sVersionInfo(
		testRancherVersion,
		metadata.DriverData.K8sVersionRKESystemImages,
		metadata.DriverData.K8sVersionServiceOptions,
		metadata.DriverData.K8sVersionWindowsServiceOptions,
		metadata.DriverData.K8sVersionInfo,
	)
}

func getTestRKESystemImages() v3.RKESystemImages {
	return metadata.DriverData.K8sVersionRKESystemImages[testRancherVersion]
}

func cloneTestSystemChart(t *testing.T) string {
	tempDir, err := ioutil.TempDir("", "system-chart")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	gitCloneCmd := exec.CommandContext(ctx, "git", "clone",
		"--branch", testSystemChartBranch,
		"--single-branch",
		"https://github.com/rancher/system-charts.git",
		tempDir,
	)
	fmt.Printf("Cloning system charts (branch: %s) into %s\n", testSystemChartBranch, tempDir)
	gitCloneCmdOutput, err := gitCloneCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to clone system chart: %s, %v", string(gitCloneCmdOutput), err)
	}
	fmt.Printf("Cloned system charts (branch: %s) into %s\n", testSystemChartBranch, tempDir)

	gitCheckoutCmd := exec.CommandContext(ctx, "git",
		"-C", tempDir,
		"checkout",
		testSystemChartCommit,
	)
	fmt.Printf("Checking out system charts to %s\n", testSystemChartCommit)
	gitCheckoutOutput, err := gitCheckoutCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to checkout system chart: %s, %v", string(gitCheckoutOutput), err)
	}
	fmt.Printf("Checked out system charts to %s\n", testSystemChartCommit)

	return tempDir
}

func flatStringSlice(slices ...[]string) []string {
	var ret []string
	for _, s := range slices {
		ret = append(ret, s...)
	}
	return ret
}
