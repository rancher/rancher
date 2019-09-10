package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	metadata "github.com/rancher/kontainer-driver-metadata/rke"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	img "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/types/image"
)

var (
	scriptMap = map[string]string{
		"linux-save":     linuxSaveScript,
		"linux-load":     linuxLoadScript,
		"linux-mirror":   linuxMirrorScript,
		"windows-save":   windowsSaveScirpt,
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
	requiredImagesNotInSystemCharts = []string{
		"busybox",
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
	linuxInfo, windowsInfo := kd.GetK8sVersionInfo(
		rancherVersion,
		metadata.DriverData.K8sVersionRKESystemImages,
		metadata.DriverData.K8sVersionServiceOptions,
		metadata.DriverData.K8sVersionWindowsServiceOptions,
		metadata.DriverData.K8sVersionInfo,
	)

	targetImages, err := img.GetLinuxImages(systemChartPath, imagesFromArgs, linuxInfo.RKESystemImages)
	if err != nil {
		return err
	}

	targetWindowsImages, err := img.GetWindowsImages(windowsInfo.RKESystemImages)
	if err != nil {
		return err
	}
	if agentImage := getWindowsAgentImage(); agentImage != "" {
		targetWindowsImages = append(targetWindowsImages, image.Mirror(agentImage))
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
usage () {
    echo "USAGE: $0 [--images rancher-images.tar.gz] --registry my.registry.com:5000"
    echo "  [-l|--image-list path] text file with list of images. 1 per line."
    echo "  [-i|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry:port."
    echo "  [-h|--help] Usage message"
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

while IFS= read -r i; do
    [ -z "${i}" ] && continue
    echo "Tagging ${reg}/${i}"
    case $i in
    */*)
        docker tag "${i}" "${reg}/${i}"
        docker push "${reg}/${i}"
        ;;
    *)
        docker tag "${i}" "${reg}/rancher/${i}"
        docker push "${reg}/rancher/${i}"
        ;;
    esac
done < "${list}"
`
	linuxSaveScript = `#!/bin/bash
list="rancher-images.txt"
images="rancher-images.tar.gz"

usage () {
    echo "USAGE: $0 [--image-list rancher-images.txt] [--images rancher-images.tar.gz]"
    echo "  [-l|--image-list path] text file with list of images. 1 per line."
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

set -e

pulled=""
while IFS= read -r i; do
    [ -z "${i}" ] && continue
    if [ $(docker pull --quiet "${i}") ]; then
        echo "Image pull success: ${i}"
        pulled="${pulled} ${i}"
    else
        echo "Image pull failed: ${i}"
    fi
done < "${list}"

echo "Creating ${images} with $(echo ${pulled} | wc -w | tr -d '[:space:]') images"
docker save $(echo ${pulled}) | gzip --stdout > ${images}
`
	linuxMirrorScript = "#!/bin/sh\nset -e -x\n\n"
	windowsLoadScript = `<#
    .PARAMETER  registry
    target private registry:port.
    .PARAMETER image-list
    text file with list of images. 1 per line. default is %s
    .PARAMETER images
    tar.gz generated by docker save. default is rancher-windows-images.tar.gz
#>
param(
    [PARAMETER(Mandatory=$true,Position=0,HelpMessage="target private registry:port.")][string]$registry,
    [string]${image-list}="%s",
    [string]$images="rancher-windows-images.tar.gz"
)

$content=Get-Content -path ${image-list}

docker load --input ${images}

foreach ($item in $content) {
    docker tag $item $Registry/$item
    docker push $Registry/$item
}
`
	windowsSaveScirpt = `#Requires -Version 5.0
<#
	.PARAMETER image-list
	text file with list of images. 1 per line.
	.PARAMETER images
	tar.gz generated by docker save.
#>
param(
	[string]${image-list}="rancher-windows-images.txt",
	[string]$images="rancher-windows-images.tar.gz"
)

$content=Get-Content -path ${image-list}

foreach ($item in $content) {
	docker pull $item
}

docker save $($content) -o $images
`
	windowsMirrorScript = ``
)
