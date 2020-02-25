package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"
	"time"

	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/kdm"
	assertlib "github.com/stretchr/testify/assert"
)

const (
	testSystemChartBranch = "dev"
	testSystemChartCommit = "7c183b98ceb8a7a50892cdb9a991fe627a782fe6"
)

func TestFetchImagesFromCharts(t *testing.T) {
	systemChartPath := cloneTestSystemChart(t)

	bothImages := []string{
		"rancher/fluentd:v0.1.16",
	}
	linuxImagesOnly := []string{
		"rancher/prom-alertmanager:v0.17.0",
	}
	windowsImagesOnly := []string{
		"rancher/wmi_exporter-package:v0.0.2",
	}

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
			outputShouldContain: flatStringSlice(
				bothImages,
				linuxImagesOnly,
			),
			outputShouldNotContain: windowsImagesOnly,
		},
		{
			caseName:    "fetch windows images from charts",
			inputPath:   systemChartPath,
			inputOsType: Windows,
			outputShouldContain: flatStringSlice(
				bothImages,
				windowsImagesOnly,
			),
			outputShouldNotContain: linuxImagesOnly,
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
	linuxInfo, windowsInfo, err := getTestK8sVersionInfo()
	if err != nil {
		t.Error(err)
	}
	toolsSystemImages := v3.ToolsSystemImages

	bothImages := []string{
		selectFirstEntry(linuxInfo.RKESystemImages).NginxProxy,
	}
	linuxImagesOnly := []string{
		selectFirstEntry(linuxInfo.RKESystemImages).CoreDNS,
		toolsSystemImages.PipelineSystemImages.Jenkins, // from tools
	}
	windowsImagesOnly := []string{
		selectFirstEntry(windowsInfo.RKESystemImages).WindowsPodInfraContainer,
	}

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
			outputShouldContain: flatStringSlice(
				bothImages,
				linuxImagesOnly,
			),
			outputShouldNotContain: windowsImagesOnly,
		},
		{
			caseName:             "fetch windows images from system images",
			inputRkeSystemImages: windowsInfo.RKESystemImages,
			inputOsType:          Windows,
			outputShouldContain: flatStringSlice(
				bothImages,
				windowsImagesOnly,
			),
			outputShouldNotContain: linuxImagesOnly,
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
	linuxInfo, windowsInfo, err := getTestK8sVersionInfo()
	if err != nil {
		t.Error(err)
	}
	toolsSystemImages := v3.ToolsSystemImages

	bothImages := []string{
		selectFirstEntry(linuxInfo.RKESystemImages).NginxProxy, // from system
		"rancher/fluentd:v0.1.16",                              // from chart
	}
	linuxImagesOnly := append(
		getRequirementImages(Linux),                         // from requirement
		selectFirstEntry(linuxInfo.RKESystemImages).CoreDNS, // from system
		"rancher/prom-alertmanager:v0.17.0",                 // from chart
		toolsSystemImages.PipelineSystemImages.Jenkins,      // from tools
	)
	windowsImagesOnly := append(
		getRequirementImages(Windows),                                          // from requirement
		selectFirstEntry(windowsInfo.RKESystemImages).WindowsPodInfraContainer, // from system
		"rancher/wmi_exporter-package:v0.0.2",                                  // from chart
	)

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
			caseName:             "get linux images",
			inputSystemChartPath: systemChartPath,
			inputImagesFromArgs: []string{
				"rancher/rancher:master-head",
				"rancher/rancher-agent:master-head",
			},
			inputRkeSystemImages: linuxInfo.RKESystemImages,
			inputOsType:          Linux,
			outputShouldContain: flatStringSlice(
				linuxImagesOnly,
				bothImages,
			),
			outputShouldNotContain: windowsImagesOnly,
		},
		{
			caseName:             "get windows images",
			inputSystemChartPath: systemChartPath,
			inputImagesFromArgs: []string{
				"rancher/rancher-agent:master-head",
			},
			inputRkeSystemImages: windowsInfo.RKESystemImages,
			inputOsType:          Windows,
			outputShouldContain: flatStringSlice(
				windowsImagesOnly,
				bothImages,
			),
			outputShouldNotContain: linuxImagesOnly,
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

func getTestK8sVersionInfo() (*kd.VersionInfo, *kd.VersionInfo, error) {
	b, err := ioutil.ReadFile("/root/data.json")
	if err != nil {
		return nil, nil, err
	}
	data, err := kdm.FromData(b)
	if err != nil {
		return nil, nil, err
	}
	l, w := kd.GetK8sVersionInfo(
		kd.RancherVersionDev,
		data.K8sVersionRKESystemImages,
		data.K8sVersionServiceOptions,
		data.K8sVersionWindowsServiceOptions,
		data.K8sVersionInfo,
	)
	return l, w, nil
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

func selectFirstEntry(rkeSystemImages map[string]v3.RKESystemImages) v3.RKESystemImages {
	for _, rkeSystemImage := range rkeSystemImages {
		return rkeSystemImage
	}
	return v3.RKESystemImages{}
}
