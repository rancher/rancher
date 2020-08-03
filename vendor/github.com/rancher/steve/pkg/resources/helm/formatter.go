package helm

import (
	"github.com/rancher/apiserver/pkg/types"
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
