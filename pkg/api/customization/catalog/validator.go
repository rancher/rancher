package catalog

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

var (
	controlChars   = regexp.MustCompile("[[:cntrl:]]")
	controlEncoded = regexp.MustCompile("%[0-1][0-9,a-f,A-F]")
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if pathURL, _ := data["url"].(string); pathURL != "" {
		if err := validateURL(pathURL); err != nil {
			return err
		}
		if u, err := url.Parse(pathURL); err == nil {
			u.Scheme = strings.ToLower(u.Scheme) // git commands are case-sensitive
			data["url"] = u.String()
		}
		return nil
	} else if request.Method == http.MethodPost {
		return httperror.NewAPIError(httperror.MissingRequired, "Catalog URL not specified")
	}
	return nil
}

func validateURL(pathURL string) error {
	if controlChars.FindStringIndex(pathURL) != nil || controlEncoded.FindStringIndex(pathURL) != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Invalid characters in catalog URL")
	}
	return nil
}
