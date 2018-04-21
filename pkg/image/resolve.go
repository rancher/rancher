package image

import (
	"path"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

func Resolve(image string) string {
	reg := settings.SystemDefaultRegistry.Get()
	if reg != "" && !strings.HasPrefix(image, reg) {
		return path.Join(reg, image)
	}

	return image
}
