package globalrole

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Wrapper struct {
	GlobalRoleLister v3.GlobalRoleLister
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut {
		return nil
	}

	gr, err := w.GlobalRoleLister.Get("", request.ID)
	if err != nil {
		if errors.IsNotFound(err) {
			return httperror.NewAPIError(httperror.NotFound, err.Error())
		}
		return err
	}

	if gr.Builtin == true {
		// Drop everything but locked and defaults. If it's builtin nothing else can change.
		for k := range data {
			if k == "newUserDefault" {
				continue
			}
			delete(data, k)
		}

	}
	return nil
}
