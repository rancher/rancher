package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/image"
)

func main() {
	if err := run(os.Args[1:]...); err != nil {
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

	err = imagesText(targetImages)
	if err != nil {
		return err
	}

	err = mirrorScript(targetImages)
	if err != nil {
		return err
	}

	err = saveScript(targetImages)
	if err != nil {
		return err
	}

	return loadScript(targetImages)
}

func loadScript(targetImages []string) error {
	log.Println("Creating rancher-load-images.sh")
	load, err := os.Create("rancher-load-images.sh")
	if err != nil {
		return err
	}
	defer load.Close()
	load.Chmod(0755)

	fmt.Fprintln(load, `#!/bin/sh

if [ -z "$1" ]; then
  echo Usage: $0 [REGISTRY]
  exit 1
fi

set -e -x

tar xvzf rancher-images.tar.gz | docker load

`)

	for _, saveImage := range saveImages(targetImages) {
		fmt.Fprintf(load, "docker tag %s ${REGISTRY}/%s\n", saveImage, saveImage)
		fmt.Fprintf(load, "docker push ${REGISTRY}/%s\n", saveImage)
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

func saveScript(targetImages []string) error {
	log.Println("Creating rancher-save-images.sh")
	save, err := os.Create("rancher-save-images.sh")
	if err != nil {
		return err
	}
	defer save.Close()
	save.Chmod(0755)

	fmt.Fprintf(save, "#!/bin/sh\nset -e -x\n\n")

	for _, targetImage := range saveImages(targetImages) {
		fmt.Fprintf(save, "docker pull %s\n", targetImage)
	}

	fmt.Fprintf(save, "docker save %s | gzip -c > rancher-images.tar.gz\n",
		strings.Join(saveImages(targetImages), " "))

	return nil
}

func imagesText(targetImages []string) error {
	log.Println("Creating rancher-images.txt")
	save, err := os.Create("rancher-images.txt")
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

func mirrorScript(targetImages []string) error {
	log.Println("Creating rancher-mirror-to-rancher-org.sh")
	mirror, err := os.Create("rancher-mirror-to-rancher-org.sh")
	if err != nil {
		return err
	}
	defer mirror.Close()
	mirror.Chmod(0755)

	fmt.Fprintf(mirror, "#!/bin/sh\nset -e -x\n\n")

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
