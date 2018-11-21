package image

import (
	"path"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

func Resolve(image string) string {
	reg := settings.SystemDefaultRegistry.Get()
	if reg != "" && !strings.HasPrefix(image, reg) {
		//Images from Dockerhub Library repo, we add rancher prefix when using private registry
		if !strings.Contains(image, "/") {
			image = "rancher/" + image
		}
		return path.Join(reg, image)
	}

	return image
}
