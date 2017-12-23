package parse

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
)

const (
	maxFormSize = 2 * 1 << 20
)

var (
	multiSlashRegexp = regexp.MustCompile("//+")
	allowedFormats   = map[string]bool{
		"html": true,
		"json": true,
	}
)

type ParsedURL struct {
	Version          string
	Type             string
	ID               string
	Link             string
	Method           string
	Action           string
	SubContext       map[string]string
	SubContextPrefix string
	Query            url.Values
}

type ResolverFunc func(typeName string, context *types.APIContext) error

type URLParser func(schema *types.Schemas, url *url.URL) (ParsedURL, error)

func DefaultURLParser(schemas *types.Schemas, url *url.URL) (ParsedURL, error) {
	result := ParsedURL{}

	version := Version(schemas, url.Path)
	if version == nil {
		return result, nil
	}

	path := url.Path
	path = multiSlashRegexp.ReplaceAllString(path, "/")

	parts := strings.SplitN(path[len(version.Path):], "/", 4)
	prefix, parts, subContext := parseSubContext(schemas, version, parts)

	result.Version = version.Path
	result.SubContext = subContext
	result.SubContextPrefix = prefix
	result.Action, result.Method = parseAction(url)
	result.Query = url.Query()

	result.Type = safeIndex(parts, 1)
	result.ID = safeIndex(parts, 2)
	result.Link = safeIndex(parts, 3)

	return result, nil
}

func Parse(rw http.ResponseWriter, req *http.Request, schemas *types.Schemas, urlParser URLParser, resolverFunc ResolverFunc) (*types.APIContext, error) {
	var err error

	result := &types.APIContext{
		Schemas:        schemas,
		Request:        req,
		Response:       rw,
		Method:         parseMethod(req),
		ResponseFormat: parseResponseFormat(req),
	}

	result.URLBuilder, _ = urlbuilder.New(req, types.APIVersion{}, schemas)

	// The response format is guarenteed to be set even in the event of an error
	parsedURL, err := urlParser(schemas, req.URL)
	// wait to check error, want to set as much as possible

	result.SubContext = parsedURL.SubContext
	result.Type = parsedURL.Type
	result.ID = parsedURL.ID
	result.Link = parsedURL.Link
	result.Action = parsedURL.Action
	result.Query = parsedURL.Query
	if parsedURL.Method != "" {
		result.Method = parsedURL.Method
	}

	for i, version := range schemas.Versions() {
		if version.Path == parsedURL.Version {
			result.Version = &schemas.Versions()[i]
			break
		}
	}

	if err != nil {
		return result, err
	}

	if result.Version == nil {
		result.Method = http.MethodGet
		result.URLBuilder, err = urlbuilder.New(req, types.APIVersion{}, result.Schemas)
		result.Type = "apiRoot"
		result.Schema = result.Schemas.Schema(&builtin.Version, "apiRoot")
		return result, nil
	}

	result.URLBuilder, err = urlbuilder.New(req, *result.Version, result.Schemas)
	if err != nil {
		return result, err
	}

	if parsedURL.SubContextPrefix != "" {
		result.URLBuilder.SetSubContext(parsedURL.SubContextPrefix)
	}

	if err := resolverFunc(result.Type, result); err != nil {
		return result, err
	}

	if result.Schema == nil {
		result.Method = http.MethodGet
		result.Type = "apiRoot"
		result.Schema = result.Schemas.Schema(&builtin.Version, "apiRoot")
		result.ID = result.Version.Path
		return result, nil
	}

	result.Type = result.Schema.ID

	if err := ValidateMethod(result); err != nil {
		return result, err
	}

	return result, nil
}

func parseSubContext(schemas *types.Schemas, version *types.APIVersion, parts []string) (string, []string, map[string]string) {
	subContext := ""
	result := map[string]string{}

	for len(parts) > 3 && version != nil && parts[3] != "" {
		resourceType := parts[1]
		resourceID := parts[2]

		if !version.SubContexts[resourceType] {
			break
		}

		subSchema := schemas.Schema(version, parts[3])
		if subSchema == nil {
			break
		}

		result[resourceType] = resourceID
		subContext = subContext + "/" + resourceType + "/" + resourceID
		parts = append(parts[:1], parts[3:]...)
	}

	return subContext, parts, result
}

func DefaultResolver(typeName string, apiContext *types.APIContext) error {
	if typeName == "" {
		return nil
	}

	schema := apiContext.Schemas.Schema(apiContext.Version, typeName)
	if schema == nil && (typeName == builtin.Schema.ID || typeName == builtin.Schema.PluralName) {
		// Schemas are special, we include it as though part of the API request version
		schema = apiContext.Schemas.Schema(&builtin.Version, typeName)
	}
	if schema == nil {
		return nil
	}

	apiContext.Schema = schema
	return nil
}

func safeIndex(slice []string, index int) string {
	if index >= len(slice) {
		return ""
	}
	return slice[index]
}

func parseResponseFormat(req *http.Request) string {
	format := req.URL.Query().Get("_format")

	if format != "" {
		format = strings.TrimSpace(strings.ToLower(format))
	}

	/* Format specified */
	if allowedFormats[format] {
		return format
	}

	// User agent has Mozilla and browser accepts */*
	if IsBrowser(req, true) {
		return "html"
	}
	return "json"
}

func parseMethod(req *http.Request) string {
	method := req.URL.Query().Get("_method")
	if method == "" {
		method = req.Method
	}
	return method
}

func parseAction(url *url.URL) (string, string) {
	action := url.Query().Get("action")
	if action == "remove" {
		return "", http.MethodDelete
	}

	return action, ""
}

func Version(schemas *types.Schemas, path string) *types.APIVersion {
	path = multiSlashRegexp.ReplaceAllString(path, "/")
	for _, version := range schemas.Versions() {
		if version.Path == "" {
			continue
		}
		if strings.HasPrefix(path, version.Path) {
			return &version
		}
	}

	return nil
}

func Body(req *http.Request) (map[string]interface{}, error) {
	req.ParseMultipartForm(maxFormSize)
	if req.MultipartForm != nil {
		return valuesToBody(req.MultipartForm.Value), nil
	}

	if req.Form != nil && len(req.Form) > 0 {
		return valuesToBody(map[string][]string(req.Form)), nil
	}

	return ReadBody(req)
}

func valuesToBody(input map[string][]string) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range input {
		result[k] = v
	}
	return result
}
