package image

import (
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rketypes "github.com/rancher/rke/types"
)

type System struct {
	Config ExportConfig
}

func (s System) FetchImages(rkeSystemImages map[string]rketypes.RKESystemImages, imagesSet map[string]map[string]struct{}) error {
	if len(rkeSystemImages) <= 0 {
		return nil
	}
	collectionImagesList := []interface{}{rkeSystemImages}
	if s.Config.OsType == Linux {
		collectionImagesList = append(collectionImagesList, v32.ToolsSystemImages)
	}
	images, err := flatImagesFromCollections(collectionImagesList...)
	if err != nil {
		return err
	}
	for _, image := range images {
		addSourceToImage(imagesSet, image, "system")
	}
	return nil
}

func flatImagesFromCollections(cols ...interface{}) (images []string, err error) {
	for _, col := range cols {
		colObj := map[string]interface{}{}
		if err := convert.ToObj(col, &colObj); err != nil {
			return []string{}, err
		}
		images = append(images, fetchImagesFromCollection(colObj)...)
	}
	return images, nil
}

func fetchImagesFromCollection(obj map[string]interface{}) (images []string) {
	for _, v := range obj {
		switch t := v.(type) {
		case string:
			images = append(images, t)
		case map[string]interface{}:
			images = append(images, fetchImagesFromCollection(t)...)
		}
	}
	return images
}
