package image

import (
	"path"
	"sort"
	"strings"

	"github.com/rancher/norman/types/convert"
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

func CollectionImages(objs ...interface{}) ([]string, error) {
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
