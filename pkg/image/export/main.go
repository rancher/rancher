package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rancher/norman/types/convert"
	libhelm "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/image"
	"gopkg.in/yaml.v2"
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
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("system charts path is required, please set it as the first parameter")
	}
	images, err := getImagesFromCharts(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	images = append(images, os.Args[2:]...)

	if err := run(images...); err != nil {
		log.Fatal(err)
	}
}

func run(images ...string) error {
	targetImages, err := collectionImages(v3.K8sVersionToRKESystemImages, v3.ToolsSystemImages)
	if err != nil {
		return err
	}

	for _, i := range images {
		targetImages = append(targetImages, image.Mirror(i))
	}

	targetWindowsImages, err := collectionImages(v3.K8sVersionWindowsSystemImages)
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

func collectionImages(objs ...interface{}) ([]string, error) {
	images := map[string]bool{}

	for _, obj := range objs {
		data := map[string]interface{}{}
		if err := convert.ToObj(obj, &data); err != nil {
			return nil, err
		}
		findStrings(data, images)
	}

	var result []string
	for k := range images {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result, nil
}

func findStrings(obj map[string]interface{}, found map[string]bool) {
	for _, v := range obj {
		switch t := v.(type) {
		case string:
			found[t] = true
		case map[string]interface{}:
			findStrings(t, found)
		}
	}
}

func getImagesFromCharts(path string) ([]string, error) {
	var images []string
	imageMap := map[string]struct{}{}
	chartVersion, err := getChartAndVersion(path)
	if err != nil {
		return nil, err
	}
	if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		return walkFunc(imageMap, chartVersion, path, p, info, err)
	}); err != nil {
		return images, err
	}
	for value := range imageMap {
		images = append(images, value)
	}
	return images, nil
}

func getChartAndVersion(path string) (map[string]string, error) {
	rtn := map[string]string{}
	helm := libhelm.Helm{
		LocalPath: path,
		IconPath:  path,
		Hash:      "",
	}
	index, err := helm.LoadIndex()
	if err != nil {
		return nil, err
	}
	for k, versions := range index.IndexFile.Entries {
		// because versions is sorted in reverse order, the first one will be the latest version
		if len(versions) > 0 {
			rtn[k] = versions[0].Dir
		}
	}

	return rtn, nil
}

func walkFunc(images map[string]struct{}, versions map[string]string, basePath, path string, info os.FileInfo, err error) error {
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return err
	}
	var found bool
	for _, v := range versions {
		if strings.HasPrefix(relPath, v) {
			found = true
			break
		}
	}
	if !found || info.Name() != "values.yaml" {
		return nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	dataInterface := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(data, &dataInterface); err != nil {
		return err
	}

	walkthroughMap(dataInterface, func(inputMap map[interface{}]interface{}) {
		generateImages(inputMap, images)
	})
	return nil
}

func generateImages(inputMap map[interface{}]interface{}, output map[string]struct{}) {
	r, repoOk := inputMap["repository"]
	t, tagOk := inputMap["tag"]
	if !repoOk || !tagOk {
		return
	}
	repo, repoOk := r.(string)
	tag, tagOk := t.(string)
	if !repoOk || !tagOk {
		return
	}

	output[fmt.Sprintf("%s:%s", repo, tag)] = struct{}{}

	return
}

func walkthroughMap(inputMap map[interface{}]interface{}, walkFunc func(map[interface{}]interface{})) {
	walkFunc(inputMap)
	for _, value := range inputMap {
		if v, ok := value.(map[interface{}]interface{}); ok {
			walkthroughMap(v, walkFunc)
		}
	}
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
list="rancher-images.txt"
images="rancher-images.tar.gz"

POSITIONAL=()
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
    esac
done

usage () {
    echo "USAGE: $0 [--image-list rancher-images.txt] [--images rancher-images.tar.gz] --registry my.registry.com:5000"
    echo "  [-l|--images-list path] text file with list of images. 1 per line."
    echo "  [-l|--images path] tar.gz generated by docker save."
    echo "  [-r|--registry registry:port] target private registry:port."
    echo "  [-h|--help] Usage message"
}

if [[ -z $reg ]]; then
    usage
    exit 1
fi
if [[ $help ]]; then
    usage
    exit 0
fi

set -e -x

docker load --input ${images}

for i in $(cat ${list}); do
    case $i in
    */*)
        docker tag ${i} ${reg}/${i}
        docker push ${reg}/${i}
        ;;
    *)
        docker tag ${i} ${reg}/rancher/${i}
        docker push ${reg}/rancher/${i}
        ;;
    esac
done
`
	linuxSaveScript = `#!/bin/bash
list="rancher-images.txt"
images="rancher-images.tar.gz"

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
	esac
done

usage () {
	echo "USAGE: $0 [--image-list rancher-images.txt] [--images rancher-images.tar.gz]"
	echo "  [-l|--images-list path] text file with list of images. 1 per line."
	echo "  [-l|--images path] tar.gz generated by docker save."
	echo "  [-h|--help] Usage message"
}

if [[ $help ]]; then
	usage
	exit 0
fi

set -e -x

for i in $(cat ${list}); do
	docker pull ${i}
done

docker save $(cat ${list} | tr '\n' ' ') | gzip -c > ${images}
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
