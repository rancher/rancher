package main

import (
	"log"
	"os"

	img "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/image/utilities"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("\"main.go\" requires 2 arguments. Usage: go run main.go [SYSTEM_CHART_PATH] [CHART_PATH] [OPTIONAL]...")
	}

	if err := run(os.Args[1], os.Args[2], os.Args[3:]); err != nil {
		log.Fatal(err)
	}
}

func run(systemChartsPath, chartsPath string, imagesFromArgs []string) error {
	targetsAndSources, err := utilities.GatherTargetImagesAndSources(systemChartsPath, chartsPath, imagesFromArgs)
	if err != nil {
		return err
	}

	// create rancher-image-origins.txt. Will fail if /pkg/image/origins.go
	// does not provide a mapping for each image.
	err = img.GenerateImageOrigins(targetsAndSources.LinuxImagesFromArgs, targetsAndSources.TargetLinuxImages, targetsAndSources.TargetWindowsImages)
	if err != nil {
		return err
	}

	type imageTextLists struct {
		images           []string
		imagesAndSources []string
	}
	for arch, imageLists := range map[string]imageTextLists{
		"linux":   {images: targetsAndSources.TargetLinuxImages, imagesAndSources: targetsAndSources.TargetLinuxImagesAndSources},
		"windows": {images: targetsAndSources.TargetWindowsImages, imagesAndSources: targetsAndSources.TargetWindowsImagesAndSources},
	} {
		err = utilities.ImagesText(arch, imageLists.images)
		if err != nil {
			return err
		}

		if err = utilities.ImagesAndSourcesText(arch, imageLists.imagesAndSources); err != nil {
			return err
		}
		err = utilities.MirrorScript(arch, imageLists.images)
		if err != nil {
			return err
		}

		err = utilities.SaveScript(arch, imageLists.images)
		if err != nil {
			return err
		}

		err = utilities.LoadScript(arch, imageLists.images)
		if err != nil {
			return err
		}
	}

	return nil
}
