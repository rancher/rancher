package catalog

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

var (
	controlChars   = regexp.MustCompile("[[:cntrl:]]")
	controlEncoded = regexp.MustCompile("%[0-1][0-9,a-f,A-F]")
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	url, ok := data["url"].(string)
	if ok && len(url) > 0 {
		sanitizedURL, err := validateURL(url)
		if err != nil {
			return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
		}
		data["url"] = sanitizedURL
		return nil
	} else if request.Method == http.MethodPost {
		return httperror.NewAPIError(httperror.MissingRequired, "Catalog URL not specified")
	}
	return nil
}

func validateURL(pathURL string) (string, error) {
	// Remove inline control character
	pathURL = controlChars.ReplaceAllString(pathURL, "")
	// Remove control characters that have been urlencoded such as %0d, %1B
	pathURL = controlEncoded.ReplaceAllString(pathURL, "")
	// Validate scheme
	parsedURL, err := url.Parse(pathURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(parsedURL.Scheme, "http") {
		return "", errors.Errorf("unsupported protocol scheme '%s'", parsedURL.Scheme)
	}
	return parsedURL.String(), nil
}
