package machine

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	requestedHostNameField = "requestedHostname"
)

func Validator(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	value, ok := data[requestedHostNameField]
	if !ok {
		return nil
	}

	if errs := validation.IsDNS1123Subdomain(value.(string)); len(errs) != 0 {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid %s %s: %s",
			requestedHostNameField, value, strings.Join(errs, ",")))
	}
	return nil
}
