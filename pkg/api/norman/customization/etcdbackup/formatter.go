package etcdbackup

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	state := convert.ToString(resource.Values["state"])
	if state == "activating" {
		for _, cond := range convert.ToMapSlice(values.GetValueN(resource.Values, "status", "conditions")) {
			if cond["type"] == "Completed" {
				if cond["status"] == "False" && convert.ToString(cond["reason"]) == "Error" {
					resource.Values["state"] = "failed"
				}
				break
			}
		}
	}
}
