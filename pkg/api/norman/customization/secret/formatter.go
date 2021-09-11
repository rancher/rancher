package secret

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

const (
	maxNumBytesInDataParam = "truncateBytes"
	truncatedField         = "isTruncated"
)

func TruncateBytesInData(apiContext *types.APIContext, resources *types.GenericCollection) {
	maxNumBytesInDataValue := apiContext.Query.Get(maxNumBytesInDataParam)
	if len(maxNumBytesInDataValue) == 0 {
		return
	}
	maxNumBytes, err := convert.ToNumber(maxNumBytesInDataValue)
	if err != nil {
		return
	}
	for i := 0; i < len(resources.Data); i++ {
		r := resources.Data[i]
		resource, ok := r.(*types.RawResource)
		if !ok {
			continue
		}
		data := resource.Values
		resourceDataInt, ok := data["data"]
		if !ok {
			continue
		}
		resourceData := resourceDataInt.(map[string]interface{})
		for k, v := range resourceData {
			if len(v.(string)) > int(maxNumBytes) {
				data[truncatedField] = true
				resourceData[k] = v.(string)[:maxNumBytes]
			}
		}
	}
}
