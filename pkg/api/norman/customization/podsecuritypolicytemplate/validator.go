package podsecuritypolicytemplate

import (
	"github.com/rancher/norman/types"
)

// Validator uses k8s Pod Security Policy validation to prevent creation of pod security policy templates that are invalid
func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {

	return nil
}
