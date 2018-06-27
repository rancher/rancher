package roletemplate

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strconv"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementV3 "github.com/rancher/types/client/management/v3"
	rbacv1 "k8s.io/api/rbac/v1"
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

	rt = rt.DeepCopy()

	if rt.Builtin == true {
		var newRT *managementV3.RoleTemplate
		err = convert.ToObj(data, &newRT)
		if err != nil {
			return err
		}

		e := httperror.NewAPIError(httperror.InvalidBodyContent, "Only field 'locked' can be updated on builtin roleTemplate")

		if newRT.External != rt.External ||
			newRT.Hidden != rt.Hidden ||
			newRT.Description != rt.Description ||
			newRT.Context != rt.Context ||
			newRT.Name != rt.Name {
			return e
		}

		if len(newRT.Rules) != len(rt.Rules) {
			return e
		} else if len(newRT.Rules) > 0 {
			newRTMap := rulesToMap(newRT.Rules)
			oldRTMap := rulesToMap(rt.Rules)
			if !reflect.DeepEqual(newRTMap, oldRTMap) {
				return e
			}
		}

		if len(newRT.RoleTemplateIds) != len(rt.RoleTemplateNames) {
			return e
		} else if len(newRT.RoleTemplateIds) > 0 {
			sort.Strings(newRT.RoleTemplateIds)
			sort.Strings(rt.RoleTemplateNames)
			if !reflect.DeepEqual(newRT.RoleTemplateIds, rt.RoleTemplateNames) {
				return e
			}
		}

		if len(newRT.Annotations) != len(rt.Annotations) {
			return e
		} else if len(newRT.Annotations) > 0 {
			if !reflect.DeepEqual(newRT.Annotations, rt.Annotations) {
				return e
			}
		}

		if len(newRT.Labels) != len(rt.Labels) {
			return e
		} else if len(newRT.Labels) > 0 {
			if !reflect.DeepEqual(newRT.Labels, rt.Labels) {
				return e
			}
		}

	}
	return nil
}

func rulesToMap(rules interface{}) map[string]bool {
	// ruleMap key is the rule marshaled to a string
	ruleMap := make(map[string]bool)
	var r []managementV3.PolicyRule

	switch ruleType := rules.(type) {
	case []managementV3.PolicyRule:
		// rules are of the required type, let them fall through
		r = ruleType
	case []rbacv1.PolicyRule:
		// rules need to be converted
		for _, rule := range ruleType {
			newRule := managementV3.PolicyRule{
				APIGroups:       rule.APIGroups,
				NonResourceURLs: rule.NonResourceURLs,
				ResourceNames:   rule.ResourceNames,
				Resources:       rule.Resources,
				Verbs:           rule.Verbs,
			}
			r = append(r, newRule)
		}
	default:
		return ruleMap
	}

	for i, rule := range r {
		// sort all fields on the rule
		sort.Strings(rule.APIGroups)
		sort.Strings(rule.Resources)
		sort.Strings(rule.ResourceNames)
		sort.Strings(rule.NonResourceURLs)
		sort.Strings(rule.Verbs)

		// marshal the rule for easy comparison in the map
		ruleJSON, err := json.Marshal(rule)
		if err != nil {
			ruleMap[strconv.Itoa(i)] = true
		}
		ruleMap[string(ruleJSON)] = true
	}
	return ruleMap
}
