package main

import (
	"fmt"
	"log"
	"os"

	img "github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/image/utilities"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go [CHARTS_PATH]")
	}

	chartsVersion, ok := os.LookupEnv("CHARTS_TAG")
	if !ok {
		log.Fatal("CHARTS_TAG environment variable required")
	}

	if err := run(os.Args[1], chartsVersion); err != nil {
		log.Fatal(err)
	}
}

func run(chartsPath string, chartsVersion string) error {
	// Scan chart catalogs for image references
	targetsAndSources, err := utilities.GatherTargetArtifactsAndSources(
		chartsPath,
		"",         // No OCI charts
		[]string{}, // No additional images
		"",         // No OCI repository
		chartsVersion,
	)
	if err != nil {
		return err
	}

	// Generate rancher-charts-image-origins.txt
	// Pass empty slice for imagesFromArgs since charts don't have hardcoded system images
	err = img.GenerateImageOrigins([]string{}, targetsAndSources.TargetLinuxArtifacts, targetsAndSources.TargetWindowsArtifacts)
	if err != nil {
		return err
	}

	type imageTextLists struct {
		images           []string
		imagesAndSources []string
	}
	for arch, imageLists := range map[string]imageTextLists{
		"charts":         {images: targetsAndSources.TargetLinuxArtifacts, imagesAndSources: targetsAndSources.TargetLinuxArtifactsAndSources},
		"charts-windows": {images: targetsAndSources.TargetWindowsArtifacts, imagesAndSources: targetsAndSources.TargetWindowsArtifactsAndSources},
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

	fmt.Printf("Generated image lists for rancher-charts:%s\n", chartsVersion)
	fmt.Printf("  - rancher-charts-images.txt (%d images)\n", len(targetsAndSources.TargetLinuxArtifacts))
	fmt.Printf("  - rancher-charts-windows-images.txt (%d images)\n", len(targetsAndSources.TargetWindowsArtifacts))

	return nil
}
