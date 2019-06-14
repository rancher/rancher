package roletemplate

import (
	"fmt"
	"net/http"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
)

type Wrapper struct {
	RoleTemplateLister v3.RoleTemplateLister
}

func (w Wrapper) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if request.Method != http.MethodPut && request.Method != http.MethodPost {
		return nil
	}

	// Only cluster roles can be administrative for now. So this validation will prevent users from creating any "project" context
	// roleTemplates with administrative bit set to true
	if request.Method == http.MethodPost || request.Method == http.MethodPut {
		administrative := convert.ToBool(data[client.RoleTemplateFieldAdministrative])
		context := convert.ToString(data[client.RoleTemplateFieldContext])
		if administrative && context != "cluster" {
			return fmt.Errorf("Only cluster roles can be administrative")
		}
	}

	if request.Method != http.MethodPut {
		return nil
	}
	rt, err := w.RoleTemplateLister.Get("", request.ID)
	if err != nil {
		return err
	}

	if rt.Builtin == true {
		// Drop everything but locked and defaults. If it's builtin nothing else can change.
		for k := range data {
			if k == "locked" || k == "clusterCreatorDefault" || k == "projectCreatorDefault" {
				continue
			}
			delete(data, k)
		}

	}
	return nil
}
