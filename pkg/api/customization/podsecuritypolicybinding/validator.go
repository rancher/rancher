package podsecuritypolicybinding

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type validator struct {
	psptTemplateLister v3.PodSecurityPolicyTemplateLister
}

func NewValidator(context *config.ScaledContext) types.Validator {
	v := &validator{
		psptTemplateLister: context.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
	}
	return v.validate
}

// validate checks that the podSecurityPolicyTemplateId is in the request and is an existing template
func (v *validator) validate(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	p, ok := data["podSecurityPolicyTemplateId"].(string)
	if !ok {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "missing required podSecurityPolicyTemplateId")
	}

	_, err := v.psptTemplateLister.Get("", p)
	if k8serrors.IsNotFound(err) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "podSecurityPolicyTemplate not found")
	}

	// If the error is anything else just return it
	return err
}
