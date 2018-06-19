package roletemplate

import (
	"net/http"
	"reflect"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
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
		var newRT *v3.RoleTemplate
		err = convert.ToObj(data, &newRT)
		if err != nil {
			return err
		}

		e := httperror.NewAPIError(httperror.InvalidBodyContent, "Only field 'locked' can be updated on builtin roleTemplate")

		if newRT.External != rt.External ||
			newRT.Hidden != rt.Hidden ||
			newRT.Description != rt.Description ||
			newRT.Context != rt.Context {
			return e
		}

		if len(newRT.Rules) != len(rt.Rules) {
			return e
		} else if len(newRT.Rules) > 0 {
			if !reflect.DeepEqual(newRT.Rules, rt.Rules) {
				return e
			}
		}

		if len(newRT.RoleTemplateNames) != len(rt.RoleTemplateNames) {
			return e
		} else if len(newRT.RoleTemplateNames) > 0 {
			if !reflect.DeepEqual(newRT.RoleTemplateNames, rt.RoleTemplateNames) {
				return e
			}
		}

		annotations, ok := data["annotations"]
		if ok {
			anno := mapConvert(annotations)
			if !reflect.DeepEqual(anno, rt.Annotations) {
				return e
			}
		}

		labels, ok := data["labels"]
		if ok {
			l := mapConvert(labels)
			if !reflect.DeepEqual(l, rt.Labels) {
				return e
			}
		}
	}
	return nil
}

func mapConvert(m interface{}) map[string]string {
	newMap := make(map[string]string)
	a := m.(map[string]interface{})
	for key, value := range a {
		newMap[key] = value.(string)
	}
	return newMap
}
