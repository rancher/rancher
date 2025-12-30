package main

import (
	"log"
	"os"

	img "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/image/utilities"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("\"main.go\" requires 1 argument. Usage: go run main.go [CHART_PATHS] [OPTIONAL]...")
	}

	if err := run(os.Args[1], os.Args[2:], os.Getenv("OCI_CHART_DIRS"), os.Getenv("OCI_CHART_REPOSITORY")); err != nil {
		log.Fatal(err)
	}
}

func run(chartsPath string, imagesFromArgs []string, ociChartsPath string, ociRepositoryURL string) error {
	targetsAndSources, err := utilities.GatherTargetArtifactsAndSources(chartsPath, ociChartsPath, imagesFromArgs, ociRepositoryURL)
	if err != nil {
		return err
	}

	// create rancher-image-origins.txt. Will fail if /pkg/image/origins.go
	// does not provide a mapping for each image.
	err = img.GenerateImageOrigins(targetsAndSources.LinuxImagesFromArgs, targetsAndSources.TargetLinuxArtifacts, targetsAndSources.TargetWindowsArtifacts)
	if err != nil {
		return err
	}

	type imageTextLists struct {
		images           []string
		imagesAndSources []string
	}
	for arch, imageLists := range map[string]imageTextLists{
		"linux":   {images: targetsAndSources.TargetLinuxArtifacts, imagesAndSources: targetsAndSources.TargetLinuxArtifactsAndSources},
		"windows": {images: targetsAndSources.TargetWindowsArtifacts, imagesAndSources: targetsAndSources.TargetWindowsArtifactsAndSources},
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
