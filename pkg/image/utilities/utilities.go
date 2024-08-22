package utilities

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/go-semver/semver"
	img "github.com/rancher/rancher/pkg/image"
	ext "github.com/rancher/rancher/pkg/image/external"
	kd "github.com/rancher/rancher/pkg/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/types/image"
	"github.com/rancher/rke/types/kdm"
)

var (
	scriptMap = map[string]string{
		"linux-save":     linuxSaveScript,
		"linux-load":     linuxLoadScript,
		"linux-mirror":   linuxMirrorScript,
		"windows-save":   windowsSaveScript,
		"windows-load":   windowsLoadScript,
		"windows-mirror": windowsMirrorScript,
	}
	scriptNameMap = map[string]string{
		"linux-save":     "rancher-save-images.sh",
		"linux-load":     "rancher-load-images.sh",
		"linux-mirror":   "rancher-mirror-to-rancher-org.sh",
		"windows-save":   "rancher-save-images.ps1",
		"windows-load":   "rancher-load-images.ps1",
		"windows-mirror": "rancher-mirror-to-rancher-org.ps1",
	}
	filenameMap = map[string]string{
		"linux":   "rancher-images.txt",
		"windows": "rancher-windows-images.txt",
	}
	sourcesFilenameMap = map[string]string{
		"linux":   "rancher-images-sources.txt",
		"windows": "rancher-windows-images-sources.txt",
	}
)

// ImageTargetsAndSources is an aggregate type containing
// the list of images used by Rancher for Linux and Windows,
// as well as the source of these images.
type ImageTargetsAndSources struct {
	LinuxImagesFromArgs           []string
	TargetLinuxImages             []string
	TargetLinuxImagesAndSources   []string
	TargetWindowsImages           []string
	TargetWindowsImagesAndSources []string
}

// GatherTargetImagesAndSources queries KDM, charts and system-charts to gather all the images used by Rancher and their source.
// It returns an aggregate type, ImageTargetsAndSources, which contains the images required to run Rancher on Linux and Windows, as well
// as the source of each image.
func GatherTargetImagesAndSources(systemChartsPath, chartsPath string, imagesFromArgs []string) (ImageTargetsAndSources, error) {
	rancherVersion, ok := os.LookupEnv("TAG")
	if !ok {
		return ImageTargetsAndSources{}, fmt.Errorf("no tag defining current Rancher version, cannot gather target images and sources")
	}

	if !img.IsValidSemver(rancherVersion) || strings.HasPrefix(rancherVersion, "dev") || strings.HasPrefix(rancherVersion, "master") || strings.HasSuffix(rancherVersion, "-head") {
		rancherVersion = settings.RancherVersionDev
	}
	rancherVersion = strings.TrimPrefix(rancherVersion, "v")

	// already downloaded in dapper
	b, err := os.ReadFile(filepath.Join("data.json"))
	if os.IsNotExist(err) {
		b, err = os.ReadFile(filepath.Join(os.Getenv("HOME"), "bin", "data.json"))
	}
	if err != nil {
		return ImageTargetsAndSources{}, fmt.Errorf("could not read data.json: %w", err)
	}
	data, err := kdm.FromData(b)
	if err != nil {
		return ImageTargetsAndSources{}, fmt.Errorf("could not load KDM data: %w", err)
	}

	linuxInfo, windowsInfo := kd.GetK8sVersionInfo(
		rancherVersion,
		data.K8sVersionRKESystemImages,
		data.K8sVersionServiceOptions,
		data.K8sVersionWindowsServiceOptions,
		data.K8sVersionInfo,
	)

	var k8sVersions []string
	for k := range linuxInfo.RKESystemImages {
		k8sVersions = append(k8sVersions, k)
	}
	sort.Strings(k8sVersions)
	if err := writeSliceToFile(filepath.Join(os.Getenv("HOME"), "bin", "rancher-rke-k8s-versions.txt"), k8sVersions); err != nil {
		return ImageTargetsAndSources{}, fmt.Errorf("%s: %w", "could not write rancher-rke-k8s-versions.txt file", err)
	}

	k8sVersion1_21_0 := &semver.Version{
		Major: 1,
		Minor: 21,
		Patch: 0,
	}

	externalLinuxImages := make(map[string][]string)

	k3sUpgradeImages, err := ext.GetExternalImages(rancherVersion, data.K3S, ext.K3S, k8sVersion1_21_0, img.Linux)
	if err != nil {
		return ImageTargetsAndSources{}, fmt.Errorf("%s: %w", "could not get external images for K3s", err)
	}
	if k3sUpgradeImages != nil {
		externalLinuxImages["k3sUpgrade"] = k3sUpgradeImages
	}

	// RKE2 Provisioning will only be supported on Kubernetes v1.21+. In addition, only RKE2
	// releases corresponding to Kubernetes v1.21+ include the "rke2-images-all.linux-amd64.txt" file that we need.
	rke2LinuxImages, err := ext.GetExternalImages(rancherVersion, data.RKE2, ext.RKE2, k8sVersion1_21_0, img.Linux)
	if err != nil {
		return ImageTargetsAndSources{}, fmt.Errorf("%s: %w", "could not get external images for RKE2", err)

	}
	if rke2LinuxImages != nil {
		externalLinuxImages["rke2All"] = rke2LinuxImages
	}

	sort.Strings(imagesFromArgs)
	winsIndex := sort.SearchStrings(imagesFromArgs, "rancher/wins")
	if winsIndex > len(imagesFromArgs)-1 {
		return ImageTargetsAndSources{}, fmt.Errorf("rancher/wins upgrade image not found")
	}

	winsAgentUpdateImage := imagesFromArgs[winsIndex]
	linuxImagesFromArgs := append(imagesFromArgs[:winsIndex], imagesFromArgs[winsIndex+1:]...)

	exportConfig := img.ExportConfig{
		SystemChartsPath: systemChartsPath,
		ChartsPath:       chartsPath,
		OsType:           img.Linux,
		RancherVersion:   rancherVersion,
		GithubEndpoints:  img.ExtensionEndpoints,
	}
	targetImages, targetImagesAndSources, err := img.GetImages(exportConfig, externalLinuxImages, linuxImagesFromArgs, linuxInfo.RKESystemImages)
	if err != nil {
		return ImageTargetsAndSources{}, err
	}

	exportConfig.OsType = img.Windows
	targetWindowsImages, targetWindowsImagesAndSources, err := img.GetImages(exportConfig, nil, []string{getWindowsAgentImage(), winsAgentUpdateImage}, windowsInfo.RKESystemImages)
	if err != nil {
		return ImageTargetsAndSources{}, err
	}

	return ImageTargetsAndSources{
		LinuxImagesFromArgs:           linuxImagesFromArgs,
		TargetLinuxImages:             targetImages,
		TargetLinuxImagesAndSources:   targetImagesAndSources,
		TargetWindowsImages:           targetWindowsImages,
		TargetWindowsImagesAndSources: targetWindowsImagesAndSources,
	}, nil
}

// LoadScript produces executable files for Linux and Windows
// which will load all images used by Rancher into a given image repository.
func LoadScript(arch string, targetImages []string) error {
	loadScriptName := getScriptFilename(arch, "load")
	log.Printf("Creating %s\n", loadScriptName)
	load, err := os.Create(loadScriptName)
	if err != nil {
		return err
	}
	defer load.Close()
	load.Chmod(0755)

	fmt.Fprintf(load, getScript(arch, "load"))
	return nil
}

// SaveScript produces executable files for Linux and Windows
// which will save all the images used by Rancher using the command
// `docker save`
func SaveScript(arch string, targetImages []string) error {
	filename := getScriptFilename(arch, "save")
	log.Printf("Creating %s\n", filename)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer save.Close()
	save.Chmod(0755)

	fmt.Fprintf(save, getScript(arch, "save"))

	return nil
}

// ImagesText will produce a file containing all the images
// used by Rancher for a particular arch.
func ImagesText(arch string, targetImages []string) error {
	filename := filenameMap[arch]
	log.Printf("Creating %s\n", filename)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer save.Close()
	save.Chmod(0755)

	for _, image := range saveImages(targetImages) {
		err := checkImage(image)
		if err != nil {
			return err
		}
		fmt.Fprintln(save, image)
	}

	return nil
}

// ImagesAndSourcesText writes data of the format "image source1,..." to the filename
// designated for the given arch
func ImagesAndSourcesText(arch string, targetImagesAndSources []string) error {
	filename := sourcesFilenameMap[arch]
	log.Printf("Creating %s\n", filename)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer save.Close()
	save.Chmod(0755)

	for _, imageAndSources := range saveImagesAndSources(targetImagesAndSources) {
		if err := checkImage(strings.Split(imageAndSources, " ")[0]); err != nil {
			return err
		}
		fmt.Fprintln(save, imageAndSources)
	}

	return nil
}

// MirrorScript creates executable files for Linux and Windows
// which will perform `docker pull`'s for each image used by Rancher
func MirrorScript(arch string, targetImages []string) error {
	filename := getScriptFilename(arch, "mirror")
	log.Printf("Creating %s\n", filename)
	mirror, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer mirror.Close()
	mirror.Chmod(0755)

	scriptStarter := getScript(arch, "mirror")
	fmt.Fprintf(mirror, scriptStarter)

	var saveImages []string
	for _, targetImage := range targetImages {
		srcImage, ok := image.Mirrors[targetImage]
		if !ok {
			continue
		}

		saveImages = append(saveImages, targetImage)
		fmt.Fprintf(mirror, "docker pull %s\n", srcImage)
		if targetImage != srcImage {
			fmt.Fprintf(mirror, "docker tag %s %s\n", srcImage, targetImage)
			fmt.Fprintf(mirror, "docker push %s\n", targetImage)
		}
	}

	return nil
}

func saveImages(targetImages []string) []string {
	var saveImages []string
	for _, targetImage := range targetImages {
		_, ok := image.Mirrors[targetImage]
		if !ok {
			continue
		}

		saveImages = append(saveImages, targetImage)
	}
	return saveImages
}

func saveImagesAndSources(imagesAndSources []string) []string {
	var saveImagesAndSources []string
	for _, imageAndSources := range imagesAndSources {
		targetImage := strings.Split(imageAndSources, " ")[0]
		_, ok := image.Mirrors[targetImage]
		if !ok {
			continue
		}

		saveImagesAndSources = append(saveImagesAndSources, imageAndSources)
	}
	return saveImagesAndSources
}

func checkImage(image string) error {
	// ignore non prefixed images, also in types (image/mirror.go)
	if strings.HasPrefix(image, "weaveworks") || strings.HasPrefix(image, "noiro") {
		return nil
	}
	imageNameTag := strings.Split(image, ":")
	if len(imageNameTag) != 2 {
		return fmt.Errorf("Can't extract tag from image [%s]", image)
	}
	if imageNameTag[1] == "" {
		return fmt.Errorf("Extracted tag from image [%s] is empty", image)
	}
	if !strings.HasPrefix(imageNameTag[0], "rancher/") {
		return fmt.Errorf("Image [%s] does not start with rancher/", image)
	}
	if strings.HasSuffix(imageNameTag[0], "-") {
		return fmt.Errorf("Image [%s] has trailing '-', probably an error in image substitution", image)
	}
	return nil
}

func writeSliceToFile(filename string, versions []string) error {
	log.Printf("Creating %s\n", filename)

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	save, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	defer func() {
		if cerr := save.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	if err := save.Chmod(0755); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	for _, version := range versions {
		if _, err := fmt.Fprintln(save, version); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return nil
}

func getWindowsAgentImage() string {
	tag, ok := os.LookupEnv("TAG")
	if !ok {
		return ""
	}
	repo, ok := os.LookupEnv("REPO")
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s/rancher-agent:%s", repo, tag)
}

func getScript(arch, fileType string) string {
	return scriptMap[fmt.Sprintf("%s-%s", arch, fileType)]
}

func getScriptFilename(arch, fileType string) string {
	return scriptNameMap[fmt.Sprintf("%s-%s", arch, fileType)]
}

const (
	linuxLoadScript = `#!/bin/bash
images="rancher-images.tar.gz"
list="rancher-images.txt"
windows_image_list=""
windows_versions="1809"
source_registry=""
usage () {
    echo "USAGE: $0 [--images rancher-images.tar.gz] [--source-registry index.docker.io] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry in the registry:port format."
    echo "  [-s|--source-registry registry:port] source registry in the registry:port format."
    echo "  [--windows-image-list path] text file with list of images used in Windows. Windows image mirroring is skipped when this is empty."
    echo "  [--windows-versions version] Comma separated Windows versions. e.g., \"1809,ltsc2022\". (Default \"1809\")"
    echo "  [-h|--help] Usage message"
}

push_manifest () {
    export DOCKER_CLI_EXPERIMENTAL=enabled
    manifest_list=()
    for i in "${arch_list[@]}"
    do
        manifest_list+=("$1-${i}")
    done

    echo "Preparing manifest $1, list[${arch_list[@]}]"
    docker manifest create "$1" "${manifest_list[@]}" --amend
    docker manifest push "$1" --purge
}

while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -r|--registry)
        target_registry="$2"
        shift # past argument
        shift # past value
        ;;
        -s|--source-registry)
        source_registry="$2"
        shift # past argument
        shift # past value
        ;;
        -l|--image-list)
        list="$2"
        shift # past argument
        shift # past value
        ;;
        -i|--images)
        images="$2"
        shift # past argument
        shift # past value
        ;;
        --windows-image-list)
        windows_image_list="$2"
        shift # past argument
        shift # past value
        ;;
        --windows-versions)
        windows_versions="$2"
        shift # past argument
        shift # past value
        ;;
        -h|--help)
        help="true"
        shift
        ;;
        *)
        usage
        exit 1
        ;;
    esac
done
if [[ -z "${target_registry}" ]]; then
    usage
    exit 1
fi
if [[ $help ]]; then
    usage
    exit 0
fi

target_registry="${target_registry%/}/"
source_registry="${source_registry%/}"
if [ ! -z "${source_registry}" ]; then
    source_registry="${source_registry}/"
fi

docker load --input ${images}

linux_images=()
while IFS= read -r i; do
    [ -z "${i}" ] && continue
    linux_images+=("${i}");
done < "${list}"

arch_list=()
if [[ -n "${windows_image_list}" ]]; then
    IFS=',' read -r -a versions <<< "$windows_versions"
    for version in "${versions[@]}"
    do
        arch_list+=("windows-${version}")
    done

    windows_images=()
    while IFS= read -r i; do
        [ -z "${i}" ] && continue
        windows_images+=("${i}")
    done < "${windows_image_list}"

    # use manifest to publish images only used in Windows
    for i in "${windows_images[@]}"; do
        if [[ ! " ${linux_images[@]}" =~ " ${i}" ]]; then
            case $i in
            */*)
                image_name="${target_registry}${i}"
                ;;
            *)
                image_name="${target_registry}rancher/${i}"
                ;;
            esac
            push_manifest "${image_name}"
        fi
    done
fi

arch_list+=("linux-amd64")
for i in "${linux_images[@]}"; do
    [ -z "${i}" ] && continue
    arch_suffix=""
    use_manifest=false
    if [[ (-n "${windows_image_list}") && " ${windows_images[@]}" =~ " ${i}" ]]; then
        # use manifest to publish images when it is used both in Linux and Windows
        use_manifest=true
        arch_suffix="-linux-amd64"
    fi
    case $i in
    */*)
        image_name="${target_registry}${i}"
        ;;
    *)
        image_name="${target_registry}rancher/${i}"
        ;;
    esac

    docker tag "${source_registry}${i}" "${image_name}${arch_suffix}"
    docker push "${image_name}${arch_suffix}"

    if $use_manifest; then
        push_manifest "${image_name}"
    fi
done
`
	linuxSaveScript = `#!/bin/bash
list="rancher-images.txt"
images="rancher-images.tar.gz"
source_registry=""

usage () {
    echo "USAGE: $0 [--image-list rancher-images.txt] [--images rancher-images.tar.gz]"
    echo "  [-s|--source-registry] source registry to pull images from in registry:port format."
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-h|--help] Usage message"
}

POSITIONAL=()
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -i|--images)
        images="$2"
        shift # past argument
        shift # past value
        ;;
        -l|--image-list)
        list="$2"
        shift # past argument
        shift # past value
        ;;
        -s|--source-registry)
        source_registry="$2"
        shift # past argument
        shift # past value
        ;;
        -h|--help)
        help="true"
        shift
        ;;
        *)
        usage
        exit 1
        ;;
    esac
done

if [[ $help ]]; then
    usage
    exit 0
fi

source_registry="${source_registry%/}"
if [ ! -z "${source_registry}" ]; then
    source_registry="${source_registry}/"
fi

pulled=""
while IFS= read -r i; do
    [ -z "${i}" ] && continue
    i="${source_registry}${i}"
    if docker pull "${i}" > /dev/null 2>&1; then
        echo "Image pull success: ${i}"
        pulled="${pulled} ${i}"
    else
        if docker inspect "${i}" > /dev/null 2>&1; then
            pulled="${pulled} ${i}"
        else
            echo "Image pull failed: ${i}"
        fi
    fi
done < "${list}"

echo "Creating ${images} with $(echo ${pulled} | wc -w | tr -d '[:space:]') images"
docker save $(echo ${pulled}) | gzip --stdout > ${images}
`
	linuxMirrorScript = "#!/bin/sh\nset -e -x\n\n"
	windowsLoadScript = `$ErrorActionPreference = 'Stop'

$script_name = $MyInvocation.InvocationName
$image_list = "rancher-windows-images.txt"
$images = "rancher-windows-images.tar.gz"
$os_release_id = $(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' | Select-Object -ExpandProperty ReleaseId)

$target_registry = $null
$source_registry = ""
$help = $false

function usage {
    echo "USAGE: $script_name [--images rancher-windows-images.tar.gz] [--source-registry index.docker.io] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry in the format registry:port."
    echo "  [-s|--source-registry registry:port] source registry in the format registry:port."
    echo "  [-o|--os-release-id (1809|lstc2022|...)] release id of OS, gets detected automatically if not passed."
    echo "  [-h|--help] Usage message."
}

# parse arguments
$vals = $null
for ($i = $args.Length; $i -ge 0; $i--)
{
    $arg = $args[$i]
    switch -regex ($arg)
    {
        '^(-i|--images)$' {
            $images = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-l|--image-list)$' {
            $image_list = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-r|--registry)$' {
            $target_registry = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-s|--source-registry)$' {
            $source_registry = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-o|--os-release-id)$' {
            $os_release_id = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-h|--help)$' {
            $help = $true
            $vals = $null
        }
        default {
            if ($vals) {
                $vals = ,$arg + $vals
            } else {
                $vals = @($arg)
            }
        }
    }
}

if ($help)
{
    usage
    exit 0
}

if (-not $target_registry)
{
    echo "Registry address is required"
    usage
    exit 1
}

if (-not (Test-Path $images))
{
    echo "Could not find '$images'"
    usage
    exit 1
}

docker load --input $images
if (-not $?)
{
    echo "Could not load '$images'"
    exit 1
}

if (-not (Test-Path $image_list))
{
    exit 0
}

$target_registry = $target_registry.TrimEnd("/") + "/"

if ([string]::IsNullOrEmpty($source_registry))
{
    $source_registry = ""
}
else
{
    $source_registry = $source_registry.TrimEnd("/") + "/"
}

Get-Content -Force -Path $image_list | ForEach-Object {
    if ($_) {
        $fullname_image = ('{0}-windows-{1}' -f $_, $os_release_id)
        $source_image = -join ($source_registry, $fullname_image)

        switch -regex ($fullname_image)
        {
            '.+/.+' {
                $target_image = -join ($target_registry, $fullname_image)
                echo "Tagging $target_image"
                docker tag $source_image $target_image
                if ($?) {
                    docker push $target_image
                }
            }
            default {
                $target_image = -join ($target_registry, "rancher/", $fullname_image)
                echo "Tagging $target_image"
                docker tag $source_image $target_image
                if ($?) {
                    docker push $target_image
                }
            }
        }
    }
}

`
	windowsSaveScript = `$ErrorActionPreference = 'Stop'

$script_name = $MyInvocation.InvocationName
$image_list = "rancher-windows-images.txt"
$images = "rancher-windows-images.tar.gz"
$source_registry = ""
$os_release_id = $(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' | Select-Object -ExpandProperty ReleaseId)

$help = $false

function usage {
    echo "USAGE: $script_name [--image-list rancher-windows-images.txt] [--images rancher-windows-images.tar.gz] [--source-registry index.docker.io]"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-s|--source-registry registry:port] source registry to pull images from, in the registry:port format."
    echo "  [-o|--os-release-id (1809|ltsc2022|...)] release id of OS, gets detected automatically if not passed."
    echo "  [-h|--help] Usage message."
}

# parse arguments
$vals = $null
for ($i = $args.Length; $i -ge 0; $i--)
{
    $arg = $args[$i]
    switch -regex ($arg)
    {
        '^(-l|--image-list)$' {
            $image_list = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-i|--images)$' {
            $images = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-o|--os-release-id)$' {
            $os_release_id = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-s|--source-registry)$' {
            $source_registry = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-h|--help)$' {
            $help = $true
            $vals = $null
        }
        default {
            if ($vals) {
                $vals = ,$arg + $vals
            } else {
                $vals = @($arg)
            }
        }
    }
}

if ($help)
{
    usage
    exit 0
}

if (-not (Test-Path $image_list))
{
    echo "Could not find '$image_list' file"
    usage
    exit 1
}

if ([string]::IsNullOrEmpty($source_registry))
{
    $source_registry = ""
}
else
{
    $source_registry = $source_registry.TrimEnd("/") + "/"
}

$fullname_images = @()
Get-Content -Force -Path $image_list | ForEach-Object {
    if ($_) {
        $fullname_image = ('{0}{1}-windows-{2}' -f $source_registry, $_, $os_release_id)
        echo "Pulling $fullname_image"
        docker pull $fullname_image
        if ($?) {
            $fullname_images += @($fullname_image)
        }
    }
}

if (-not $fullname_images)
{
    echo "Could not save empty images to host"
    echo "Please verify the images of '$image_list' existing or not"
    exit 1
}
docker save $($fullname_images) -o $images

`
	windowsMirrorScript = ``
)
