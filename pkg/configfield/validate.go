package configfield

import (
	"strings"

	"github.com/rancher/norman/types/convert"
)

func GetDriver(obj interface{}) string {
	data, _ := convert.EncodeToMap(obj)
	driver := ""

	for k, v := range data {
		if !strings.HasSuffix(k, "Config") || convert.IsAPIObjectEmpty(v) {
			continue
		}

		driver = strings.TrimSuffix(k, "Config")
		break
	}

	return driver
}
