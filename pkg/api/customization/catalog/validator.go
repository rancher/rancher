package catalog

import (
	"net/http"
	"regexp"

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
		data["url"] = sanitizeURL(url)
		return nil
	} else if request.Method == http.MethodPost {
		return httperror.NewAPIError(httperror.MissingRequired, "Catalog URL not specified")
	}
	return nil
}

func sanitizeURL(pathURL string) string {
	// Remove inline control character
	pathURL = controlChars.ReplaceAllString(pathURL, "")
	// Remove control characters that have been urlencoded such as %0d, %1B
	pathURL = controlEncoded.ReplaceAllString(pathURL, "")
	return pathURL
}
