package image

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	util "github.com/rancher/rancher/pkg/cluster"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"oras.land/oras-go/v2/registry/remote"
)

var Mirrors = map[string]string{}

// ExportConfig provides parameters you can define to configure image exporting for Rancher components
type ExportConfig struct {
	RancherVersion  string
	ChartsPath      string
	GithubEndpoints []GithubEndpoint
	OsType          OSType
}

type OSType int

const (
	Linux OSType = iota
	Windows
)

// Resolve calls ResolveWithCluster passing nil into the cluster argument.
// returns the image concatenated with the URL of the system default registry.
// if there is no system default registry it will return the image
func Resolve(image string) string {
	return ResolveWithCluster(image, nil)
}

// ResolveWithCluster returns the image concatenated with the URL of the private registry specified, adding rancher/ if is a private repo.
// It will use the cluster level registry if one is found, or the system default registry if no cluster level registry is found.
// If either is not found, it returns the image.
func ResolveWithCluster(image string, cluster *v3.Cluster) string {
	reg := util.GetPrivateRegistryURL(cluster)
	if reg != "" && !strings.HasPrefix(image, reg) {
		// Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}
	return image
}

// GetImages fetches the list of container images used in the sources provided in the exportConfig.
// Rancher charts, system images and extension images of Rancher are fetched.
// GetImages is called during runtime by Rancher catalog package which is deprecated.
// It is actually used for generation rancher-images.txt for airgap scenarios.
func GetImages(chartsPath string,
	osType OSType,
	rancherVersion string,
	extensionEndpoints []GithubEndpoint,
	externalImages map[string][]string,
	imagesFromArgs []string) ([]string, []string, error) {
	imagesSet := make(map[string]map[string]struct{})

	chartsPathList := strings.Split(chartsPath, ",")
	for _, chartPath := range chartsPathList {
		exportConfig := ExportConfig{
			ChartsPath:     chartPath,
			OsType:         osType,
			RancherVersion: rancherVersion,
		}

		charts := Charts{exportConfig}
		if err := charts.FetchImages(imagesSet); err != nil {
			return nil, nil, errors.Wrap(err, "failed to fetch images from charts")
		}
	}

	exportConfig := ExportConfig{
		OsType:          osType,
		RancherVersion:  rancherVersion,
		GithubEndpoints: extensionEndpoints,
	}
	// fetch images from extension catalog images
	extensions := ExtensionsConfig{exportConfig}
	if err := extensions.FetchExtensionImages(imagesSet); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch images from extensions")
	}

	setRequirementImages(exportConfig.OsType, imagesSet)

	// set rancher images from args
	setImages("rancher", imagesFromArgs, imagesSet)

	for source, sourceImages := range externalImages {
		setImages(source, sourceImages, imagesSet)
	}

	convertMirroredImages(imagesSet)

	imagesList, imagesAndSourcesList := generateImageAndSourceLists(imagesSet)

	return imagesList, imagesAndSourcesList, nil
}

// GetOCIURLs gets list of images/helm artifacts from registry specified.
func GetOCIURLs(
	ociChartsPath string,
	rancherVersion string,
	osType OSType,
	externalImages map[string][]string,
	imagesFromArgs []string,
	registryHost string) ([]string, []string, error) {
	registryHost = "registry.suse.com/rancher" // TODO: remove this
	imagesSet := make(map[string]map[string]struct{})

	file, err := os.Open(ociChartsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open OCI charts file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	ctx := context.Background()
	for scanner.Scan() {
		chartName := strings.TrimSpace(scanner.Text())
		if chartName == "" {
			continue
		}

		repos := []string{
			fmt.Sprintf("%s/charts/%s", registryHost, chartName),
			fmt.Sprintf("%s/%s", registryHost, chartName),
		}

		for _, repoPath := range repos {
			repo, err := remote.NewRepository(repoPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create ORAS repo client for %s: %w", repoPath, err)
			}

			err = repo.Tags(ctx, "", func(tags []string) error {
				for _, tag := range tags {
					parts := strings.SplitN(repoPath, "/", 2)
					image := fmt.Sprintf("%s:%s", parts[1], tag)
					addSourceToImage(imagesSet, image, image)
				}
				return nil
			})
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list tags for %s: %w", repoPath, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading OCI charts file: %w", err)
	}

	convertMirroredImages(imagesSet)

	targetImages, targetImageSources := generateImageAndSourceLists(imagesSet)

	return targetImages, targetImageSources, nil
}

func IsValidSemver(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

func setRequirementImages(osType OSType, imagesSet map[string]map[string]struct{}) {
	coreLabel := "core"
	switch osType {
	case Linux:
		addSourceToImage(imagesSet, settings.SCCOperatorImage.Get(), coreLabel)
		addSourceToImage(imagesSet, settings.ShellImage.Get(), coreLabel)
		addSourceToImage(imagesSet, settings.MachineProvisionImage.Get(), coreLabel)
		addSourceToImage(imagesSet, "rancher/mirrored-bci-busybox:15.6.24.2", coreLabel)
		addSourceToImage(imagesSet, "rancher/mirrored-bci-micro:15.6.24.2", coreLabel)
	}
}

func setImages(source string, imagesFromArgs []string, imagesSet map[string]map[string]struct{}) {
	for _, image := range imagesFromArgs {
		addSourceToImage(imagesSet, image, source)
	}
}

func addSourceToImage(imagesSet map[string]map[string]struct{}, image string, sources ...string) {
	if image == "" {
		return
	}
	if imagesSet[image] == nil {
		imagesSet[image] = make(map[string]struct{})
	}
	for _, source := range sources {
		imagesSet[image][source] = struct{}{}
	}
}

func convertMirroredImages(imagesSet map[string]map[string]struct{}) {
	for image := range imagesSet {
		convertedImage := mirror(image)
		if image == convertedImage {
			continue
		}
		for source := range imagesSet[image] {
			addSourceToImage(imagesSet, convertedImage, source)
		}
		delete(imagesSet, image)
	}
}

func generateImageAndSourceLists(imagesSet map[string]map[string]struct{}) ([]string, []string) {
	var images, imagesAndSources []string
	// unique
	for image := range imagesSet {
		images = append(images, image)
	}

	// sort
	sort.Slice(images, func(i, j int) bool {
		return images[i] < images[j]
	})

	for _, image := range images {
		imagesAndSources = append(imagesAndSources, fmt.Sprintf("%s %s", image, getSourcesList(imagesSet[image])))
	}

	return images, imagesAndSources
}

func getSourcesList(imageSources map[string]struct{}) string {
	var sources []string

	for source := range imageSources {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return strings.Join(sources, ",")
}

func mirror(image string) string {
	orig := image
	if strings.HasPrefix(image, "weaveworks") || strings.HasPrefix(image, "noiro") {
		return image
	}

	image = strings.Replace(image, "gcr.io/google_containers", "rancher", 1)
	image = strings.Replace(image, "quay.io/coreos/", "rancher/coreos-", 1)
	image = strings.Replace(image, "quay.io/calico/", "rancher/calico-", 1)
	image = strings.Replace(image, "plugins/docker", "rancher/plugins-docker", 1)
	image = strings.Replace(image, "k8s.gcr.io/defaultbackend", "rancher/nginx-ingress-controller-defaultbackend", 1)
	image = strings.Replace(image, "k8s.gcr.io/k8s-dns-node-cache", "rancher/k8s-dns-node-cache", 1)
	image = strings.Replace(image, "plugins/docker", "rancher/plugins-docker", 1)
	image = strings.Replace(image, "kibana", "rancher/kibana", 1)
	image = strings.Replace(image, "jenkins/", "rancher/jenkins-", 1)
	image = strings.Replace(image, "alpine/git", "rancher/alpine-git", 1)
	image = strings.Replace(image, "prom/", "rancher/prom-", 1)
	image = strings.Replace(image, "quay.io/pires", "rancher", 1)
	image = strings.Replace(image, "coredns/", "rancher/coredns-", 1)
	image = strings.Replace(image, "minio/", "rancher/minio-", 1)

	Mirrors[image] = orig
	return image
}
