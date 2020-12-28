package image

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	rketypes "github.com/rancher/rke/types"
	"github.com/rancher/rke/types/kdm"
	assertlib "github.com/stretchr/testify/assert"
)

const (
	testChartRepoURL       = "https://github.com/rancher/charts.git"
	testChartBranch        = "dev-v2.5"
	testChartCommit        = "3887f667bf6d8663032b7bd298d74d46018183ca"
	testSystemChartRepoURL = "https://github.com/rancher/system-charts.git"
	testSystemChartBranch  = "dev"
	testSystemChartCommit  = "7c183b98ceb8a7a50892cdb9a991fe627a782fe6"
)

func TestFetchImagesFromCharts(t *testing.T) {
	systemChartPath := cloneChartRepo(t, testSystemChartRepoURL, testSystemChartBranch, testSystemChartCommit)
	chartRepoPath := cloneChartRepo(t, testChartRepoURL, testChartBranch, testChartCommit)
	chartPath := prepareTestCharts(t, chartRepoPath)

	systemChartBothImages := []string{
		"rancher/fluentd:v0.1.16",
	}
	systemChartLinuxImagesOnly := []string{
		"rancher/prom-alertmanager:v0.17.0",
	}
	systemChartWindowsImagesOnly := []string{
		"rancher/wmi_exporter-package:v0.0.2",
	}
	chartLinuxImagesOnly := []string{
		"rancher/istio-kubectl:1.5.8",
		"rancher/opa-gatekeeper:v3.1.0-rc.1",
	}
	chartWindowsImagesOnly := []string{}

	testCases := []struct {
		caseName               string
		inputPath              string
		inputOsType            OSType
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		{
			caseName:    "fetch linux images from system charts",
			inputPath:   systemChartPath,
			inputOsType: Linux,
			outputShouldContain: flatStringSlice(
				systemChartBothImages,
				systemChartLinuxImagesOnly,
			),
			outputShouldNotContain: systemChartWindowsImagesOnly,
		},
		{
			caseName:    "fetch windows images from system charts",
			inputPath:   systemChartPath,
			inputOsType: Windows,
			outputShouldContain: flatStringSlice(
				systemChartBothImages,
				systemChartWindowsImagesOnly,
			),
			outputShouldNotContain: systemChartLinuxImagesOnly,
		},
		{
			caseName:               "fetch linux images from charts",
			inputPath:              chartPath,
			inputOsType:            Linux,
			outputShouldContain:    chartLinuxImagesOnly,
			outputShouldNotContain: chartWindowsImagesOnly,
		},
		{
			caseName:               "fetch windows images from charts",
			inputPath:              chartPath,
			inputOsType:            Windows,
			outputShouldContain:    chartWindowsImagesOnly,
			outputShouldNotContain: chartLinuxImagesOnly,
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
	toolsSystemImages := v32.ToolsSystemImages

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
		inputRkeSystemImages   map[string]rketypes.RKESystemImages
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
	systemChartPath := cloneChartRepo(t, testSystemChartRepoURL, testSystemChartBranch, testSystemChartCommit)
	chartRepoPath := cloneChartRepo(t, testChartRepoURL, testChartBranch, testChartCommit)
	chartPath := prepareTestCharts(t, chartRepoPath)

	linuxInfo, windowsInfo, err := getTestK8sVersionInfo()
	if err != nil {
		t.Error(err)
	}
	toolsSystemImages := v32.ToolsSystemImages

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
		inputChartPath         string
		inputImagesFromArgs    []string
		inputRkeSystemImages   map[string]rketypes.RKESystemImages
		inputOsType            OSType
		outputShouldContain    []string
		outputShouldNotContain []string
	}{
		{
			caseName:             "get linux images",
			inputSystemChartPath: systemChartPath,
			inputChartPath:       chartPath,
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
			inputChartPath:       chartPath,
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
		images, err := GetImages(cs.inputSystemChartPath, cs.inputChartPath, []string{}, cs.inputImagesFromArgs, cs.inputRkeSystemImages, cs.inputOsType)
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
	b, err := ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), "bin", "data.json"))
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

func cloneChartRepo(t *testing.T, repoURL, branch, commit string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	basename := path.Base(u.Path)
	tempDirName := strings.TrimSuffix(basename, filepath.Ext(basename))
	tempDir, err := ioutil.TempDir("", tempDirName)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	gitCloneCmd := exec.CommandContext(ctx, "git", "clone",
		"--branch", branch,
		"--single-branch",
		repoURL,
		tempDir,
	)
	fmt.Printf("Cloning repository %s (branch: %s) into %s\n", repoURL, branch, tempDir)
	gitCloneCmdOutput, err := gitCloneCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to clone repository: %s, %v", string(gitCloneCmdOutput), err)
	}
	fmt.Printf("Cloned repository %s (branch: %s) into %s\n", repoURL, branch, tempDir)

	gitCheckoutCmd := exec.CommandContext(ctx, "git",
		"-C", tempDir,
		"checkout",
		commit,
	)
	fmt.Printf("Checking out commit %s\n", commit)
	gitCheckoutOutput, err := gitCheckoutCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to checkout commit: %s, %v", string(gitCheckoutOutput), err)
	}
	fmt.Printf("Checked out commit %s\n", commit)

	return tempDir
}

func prepareTestCharts(t *testing.T, chartDir string) string {
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancel()

	// Remove indexes to force building a virtual index
	indexPath := filepath.Join(chartDir, "index.yaml")
	rmIndexCmd := exec.CommandContext(ctx, "rm", indexPath)
	if output, err := rmIndexCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to execute rm command: %s, %v", string(output), err)
	}
	if _, err := os.Stat(indexPath); err == nil {
		t.Fatalf("failed to delete: %s", indexPath)
	}

	assetsIndexPath := filepath.Join(chartDir, "assets/index.yaml")
	rmAssetsIndexCmd := exec.CommandContext(ctx, "rm", assetsIndexPath)
	if output, err := rmAssetsIndexCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to execute rm command: %s, %v", string(output), err)
	}
	if _, err := os.Stat(assetsIndexPath); err == nil {
		t.Fatalf("failed to delete: %s", assetsIndexPath)
	}

	// Extract chart tgz to test fetching images
	chartTgzDir := filepath.Join(chartDir, "assets/rancher-gatekeeper")
	chartOutputTgzDir := filepath.Join(chartTgzDir, "rancher-gatekeeper")
	chartTgz := filepath.Join(chartTgzDir, "rancher-gatekeeper-v3.1.0-rc.1.tgz")

	tarCmdOutput, err := exec.CommandContext(ctx, "tar", "-xvf", chartTgz, "-C", chartTgzDir).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to extract chart:  %s, %v", string(tarCmdOutput), err)
	}
	if _, err := os.Stat(chartOutputTgzDir); os.IsNotExist(err) {
		t.Fatalf("failed to extract chart: %s, %v", chartTgz, err)
	}

	return filepath.Join(chartDir, "assets")
}

func flatStringSlice(slices ...[]string) []string {
	var ret []string
	for _, s := range slices {
		ret = append(ret, s...)
	}
	return ret
}

func selectFirstEntry(rkeSystemImages map[string]rketypes.RKESystemImages) rketypes.RKESystemImages {
	for _, rkeSystemImage := range rkeSystemImages {
		return rkeSystemImage
	}
	return rketypes.RKESystemImages{}
}
