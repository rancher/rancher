package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/controllers/management/k3supgrade"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	img "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/types/image"
	"github.com/rancher/types/kdm"
	"github.com/sirupsen/logrus"
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
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("system charts path is required, please set it as the first parameter")
	}

	if err := run(os.Args[1], os.Args[2:]); err != nil {
		log.Fatal(err)
	}
}

func run(systemChartPath string, imagesFromArgs []string) error {
	tag, ok := os.LookupEnv("TAG")
	if !ok {
		return fmt.Errorf("no tag %s", tag)
	}
	rancherVersion := tag
	if strings.HasPrefix(rancherVersion, "dev") || strings.HasPrefix(rancherVersion, "master") {
		rancherVersion = kd.RancherVersionDev
	}
	if strings.HasPrefix(rancherVersion, "v") {
		rancherVersion = rancherVersion[1:]
	}

	// already downloaded in dapper
	b, err := ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), "bin", "data.json"))
	if err != nil {
		return err
	}
	data, err := kdm.FromData(b)
	if err != nil {
		return err
	}

	linuxInfo, windowsInfo := kd.GetK8sVersionInfo(
		rancherVersion,
		data.K8sVersionRKESystemImages,
		data.K8sVersionServiceOptions,
		data.K8sVersionWindowsServiceOptions,
		data.K8sVersionInfo,
	)

	k3sUpgradeImages := getK3sUpgradeImages(rancherVersion, data.K3S)

	targetImages, err := img.GetImages(systemChartPath, k3sUpgradeImages, imagesFromArgs, linuxInfo.RKESystemImages, img.Linux)
	if err != nil {
		return err
	}

	targetWindowsImages, err := img.GetImages(systemChartPath, []string{}, []string{getWindowsAgentImage()}, windowsInfo.RKESystemImages, img.Windows)
	if err != nil {
		return err
	}

	for arch, images := range map[string][]string{
		"linux":   targetImages,
		"windows": targetWindowsImages,
	} {
		err = imagesText(arch, images)
		if err != nil {
			return err
		}

		err = mirrorScript(arch, images)
		if err != nil {
			return err
		}

		err = saveScript(arch, images)
		if err != nil {
			return err
		}

		err = loadScript(arch, images)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadScript(arch string, targetImages []string) error {
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

func saveScript(arch string, targetImages []string) error {
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

func imagesText(arch string, targetImages []string) error {
	filename := filenameMap[arch]
	log.Printf("Creating %s\n", filename)
	save, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer save.Close()
	save.Chmod(0755)

	for _, image := range saveImages(targetImages) {
		log.Println("Image:", image)
		fmt.Fprintln(save, image)
	}

	return nil
}

func mirrorScript(arch string, targetImages []string) error {
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

// getK3sUpgradeImages returns k3s-upgrade images for every k3s release that supports
// current rancher version
func getK3sUpgradeImages(rancherVersion string, k3sData map[string]interface{}) []string {
	logrus.Infof("generating k3s image list...")
	k3sImagesMap := make(map[string]bool)
	releases, _ := k3sData["releases"].([]interface{})
	var compatibleReleases []string

	for _, release := range releases {
		releaseMap, _ := release.(map[string]interface{})
		version, _ := releaseMap["version"].(string)
		if version == "" {
			continue
		}

		if rancherVersion != "dev" {
			maxVersion, _ := releaseMap["maxChannelServerVersion"].(string)
			maxVersion = strings.TrimPrefix(maxVersion, "v")
			if maxVersion == "" {
				continue
			}
			minVersion, _ := releaseMap["minChannelServerVersion"].(string)
			minVersion = strings.Trim(minVersion, "v")
			if minVersion == "" {
				continue
			}

			versionGTMin, err := k3supgrade.IsNewerVersion(minVersion, rancherVersion)
			if err != nil {
				continue
			}
			if rancherVersion != minVersion && !versionGTMin {
				// rancher version not equal to or greater than minimum supported rancher version
				continue
			}

			versionLTMax, err := k3supgrade.IsNewerVersion(rancherVersion, maxVersion)
			if err != nil {
				continue
			}
			if rancherVersion != maxVersion && !versionLTMax {
				// rancher version not equal to or greater than maximum supported rancher version
				continue
			}
		}

		compatibleReleases = append(compatibleReleases, version)
	}

	for _, release := range compatibleReleases {
		// registries don't allow +, so image names will have these substituted
		upgradeImage := fmt.Sprintf("rancher/k3s-upgrade:%s", strings.Replace(release, "+", "-", -1))
		k3sImagesMap[upgradeImage] = true

		images, err := downloadK3sSupportingImages(release)
		if err != nil {
			logrus.Infof("could not find supporting images for k3s release [%s]: %v", release, err)
			continue
		}

		supportingImages := strings.Split(images, "\n")
		if supportingImages[len(supportingImages)-1] == "" {
			supportingImages = supportingImages[:len(supportingImages)-1]
		}

		for _, imageName := range supportingImages {
			imageName = strings.TrimPrefix(imageName, "docker.io/")
			k3sImagesMap[imageName] = true
		}
	}

	var k3sImages []string
	for imageName := range k3sImagesMap {
		k3sImages = append(k3sImages, imageName)
	}

	sort.Strings(k3sImages)
	logrus.Infof("finished generating k3s image list...")
	return k3sImages
}

// DownloadK3s supporting images attempt to download k3s-images.txt files that contains a list
// of its dependencies.
func downloadK3sSupportingImages(release string) (string, error) {
	url := fmt.Sprintf("https://github.com/rancher/k3s/releases/download/%s/k3s-images.txt", release)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get url: %v", string(body))
	}
	defer resp.Body.Close()

	if err != nil {
		return "", err
	}

	return string(body), nil
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
windows_versions="1903"
usage () {
    echo "USAGE: $0 [--images rancher-images.tar.gz] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry:port."
    echo "  [--windows-image-list path] text file with list of images used in Windows. Windows image mirroring is skipped when this is empty"
    echo "  [--windows-versions version] Comma separated Windows versions. e.g., \"1809,1903\". (Default \"1903\")"
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
        reg="$2"
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
if [[ -z $reg ]]; then
    usage
    exit 1
fi
if [[ $help ]]; then
    usage
    exit 0
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
                image_name="${reg}/${i}"
                ;;
            *)
                image_name="${reg}/rancher/${i}"
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
        image_name="${reg}/${i}"
        ;;
    *)
        image_name="${reg}/rancher/${i}"
        ;;
    esac

    docker tag "${i}" "${image_name}${arch_suffix}"
    docker push "${image_name}${arch_suffix}"

    if $use_manifest; then
        push_manifest "${image_name}"
    fi
done
`
	linuxSaveScript = `#!/bin/bash
list="rancher-images.txt"
images="rancher-images.tar.gz"

usage () {
    echo "USAGE: $0 [--image-list rancher-images.txt] [--images rancher-images.tar.gz]"
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

pulled=""
while IFS= read -r i; do
    [ -z "${i}" ] && continue
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
$registry = $null
$help = $false

function usage {
    echo "USAGE: $script_name [--images rancher-windows-images.tar.gz] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry:port."
    echo "  [-o|--os-release-id (1809|1903|...)] release id of OS, gets detected automatically if not passed."
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
            $registry = ($vals | Select-Object -First 1)
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

if (-not $registry)
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

Get-Content -Force -Path $image_list | ForEach-Object {
    if ($_) {
        $fullname_image = ('{0}-windows-{1}' -f $_, $os_release_id)
		echo "Tagging $registry/$fullname_image"
	
		switch -regex ($fullname_image)
		{
			'.+/.+' {
				docker tag $fullname_image $registry/$fullname_image
                if ($?) {
                    docker push $registry/$fullname_image
                }
			}
			default {
				docker tag $fullname_image $registry/rancher/$fullname_image
				if ($?) {
                    docker push $registry/rancher/$fullname_image
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
$os_release_id = $(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' | Select-Object -ExpandProperty ReleaseId)
$help = $false

function usage {
    echo "USAGE: $script_name [--image-list rancher-windows-images.txt] [--images rancher-windows-images.tar.gz]"
    echo "  [-l|--image-list path] text file with list of images; one image per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-o|--os-release-id (1809|1903|...)] release id of OS, gets detected automatically if not passed."
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

$fullname_images = @()
Get-Content -Force -Path $image_list | ForEach-Object {
    if ($_) {
        $fullname_image = ('{0}-windows-{1}' -f $_, $os_release_id)
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
