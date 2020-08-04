package formatters

import (
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/norman/types/convert"
)

func DropHelmData(request *types.APIRequest, resource *types.RawResource) {
	data := resource.APIObject.Data()
	if data.String("metadata", "labels", "owner") == "helm" ||
		data.String("metadata", "labels", "OWNER") == "TILLER" {
		if data.String("data", "release") != "" {
			delete(data.Map("data"), "release")
		}
	}
}

func Pod(request *types.APIRequest, resource *types.RawResource) {
	data := resource.APIObject.Data()
	fields := data.StringSlice("metadata", "fields")
	if len(fields) > 2 {
		data.SetNested(convert.LowerTitle(fields[2]), "metadata", "state", "name")
	}
}
