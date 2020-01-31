package parse

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/schemaserver/urlbuilder"
)

const (
	maxFormSize = 2 * 1 << 20
)

var (
	allowedFormats = map[string]bool{
		"html": true,
		"json": true,
		"yaml": true,
	}
)

type ParsedURL struct {
	Type       string
	Name       string
	Link       string
	Method     string
	Action     string
	Prefix     string
	SubContext map[string]string
	Query      url.Values
}

type URLParser func(rw http.ResponseWriter, req *http.Request, schemas *types.APISchemas) (ParsedURL, error)

type Parser func(apiOp *types.APIRequest, urlParser URLParser) error

func Parse(apiOp *types.APIRequest, urlParser URLParser) error {
	var err error

	if apiOp.Request == nil {
		apiOp.Request, err = http.NewRequest("GET", "/", nil)
		if err != nil {
			return err
		}
	}

	apiOp = types.StoreAPIContext(apiOp)

	if apiOp.Method == "" {
		apiOp.Method = parseMethod(apiOp.Request)
	}
	if apiOp.ResponseFormat == "" {
		apiOp.ResponseFormat = parseResponseFormat(apiOp.Request)
	}

	// The response format is guaranteed to be set even in the event of an error
	parsedURL, err := urlParser(apiOp.Response, apiOp.Request, apiOp.Schemas)
	// wait to check error, want to set as much as possible

	if apiOp.Type == "" {
		apiOp.Type = parsedURL.Type
	}
	if apiOp.Name == "" {
		apiOp.Name = parsedURL.Name
	}
	if apiOp.Link == "" {
		apiOp.Link = parsedURL.Link
	}
	if apiOp.Action == "" {
		apiOp.Action = parsedURL.Action
	}
	if apiOp.Query == nil {
		apiOp.Query = parsedURL.Query
	}
	if apiOp.Method == "" && parsedURL.Method != "" {
		apiOp.Method = parsedURL.Method
	}
	if apiOp.URLPrefix == "" {
		apiOp.URLPrefix = parsedURL.Prefix
	}

	if apiOp.URLBuilder == nil {
		// make error local to not override the outer error we have yet to check
		var err error
		apiOp.URLBuilder, err = urlbuilder.New(apiOp.Request, &urlbuilder.DefaultPathResolver{
			Prefix: apiOp.URLPrefix,
		}, apiOp.Schemas)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	if apiOp.Schema == nil && apiOp.Schemas != nil {
		apiOp.Schema = apiOp.Schemas.LookupSchema(apiOp.Type)
	}

	if apiOp.Schema != nil && apiOp.Type == "" {
		apiOp.Type = apiOp.Schema.ID
	}

	if err := ValidateMethod(apiOp); err != nil {
		return err
	}

	return nil
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

	if isYaml(req) {
		return "yaml"
	}
	return "json"
}

func isYaml(req *http.Request) bool {
	return strings.Contains(req.Header.Get("Accept"), "application/yaml")
}

func parseMethod(req *http.Request) string {
	method := req.URL.Query().Get("_method")
	if method == "" {
		method = req.Method
	}
	return method
}

func Body(req *http.Request) (types.APIObject, error) {
	req.ParseMultipartForm(maxFormSize)
	if req.MultipartForm != nil {
		return valuesToBody(req.MultipartForm.Value), nil
	}

	if req.PostForm != nil && len(req.PostForm) > 0 {
		return valuesToBody(map[string][]string(req.Form)), nil
	}

	return ReadBody(req)
}

func valuesToBody(input map[string][]string) types.APIObject {
	result := map[string]interface{}{}
	for k, v := range input {
		result[k] = v
	}
	return toAPI(result)
}
