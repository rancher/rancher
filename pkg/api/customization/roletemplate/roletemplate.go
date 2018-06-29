package roletemplate

import (
	"net/http"

	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type Wrapper struct {
	RoleTemplateLister v3.RoleTemplateLister
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut {
		return nil
	}

	rt, err := w.RoleTemplateLister.Get("", request.ID)
	if err != nil {
		return err
	}

	if rt.Builtin == true {
		// Drop everything but locked. If it's builtin nothing else can change.
		for k := range data {
			if k == "locked" {
				continue
			}
			delete(data, k)
		}

	}
	return nil
}
