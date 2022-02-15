package podsecuritypolicytemplate

import (
	"errors"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/apis/policy/validation"
)

// Validator uses k8s Pod Security Policy validation to prevent creation of pod security policy templates that are invalid
func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec policy.PodSecurityPolicySpec // k8s psp
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Pod Security Policy spec conversion error")
	}
	var allErrs field.ErrorList
	allErrs = validation.ValidatePodSecurityPolicySpec(&spec, &field.Path{})
	if len(allErrs) > 0 { // concatenate all errors to present in UI
		strs := make([]string, len(allErrs))
		for i, v := range allErrs {
			strs[i] = v.Detail
		}
		return httperror.WrapAPIError(errors.New(allErrs[0].Type.String()), httperror.InvalidBodyContent, strings.Join(strs, ", "))
	}

	return nil
}
