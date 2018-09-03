package apiui

import (
	"strings"

	"github.com/rancher/rancher/pkg/settings"

	"github.com/rancher/norman/api"
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/api/writer"
	"github.com/rancher/norman/types"
)

var (
	start = `
<!DOCTYPE html>
<!-- If you are reading this, there is a good chance you would prefer sending an
"Accept: application/json" header and receiving actual JSON responses. -->
<link rel="stylesheet" type="text/css" href="%API_UI_HOST_URL%/api-ui/%API_UI_VERSION_PATH%ui.min.css" />
<script src="%API_UI_HOST_URL%/api-ui/%API_UI_VERSION_PATH%ui.min.js"></script>
<script>
var user = "admin";
var curlUser='${CATTLE_ACCESS_KEY}:${CATTLE_SECRET_KEY}';
var schemas="%SCHEMAS%";
var data =
`
	end = []byte(`</script>
`)
)

type ResponseWriter struct {
	writer.EncodingResponseWriter
}

func (h *ResponseWriter) start(apiContext *types.APIContext, code int, obj interface{}) {
	writer.AddCommonResponseHeader(apiContext)
	apiContext.Response.Header().Set("content-type", "text/html")
	apiContext.Response.WriteHeader(code)
}

func (h *ResponseWriter) Write(apiContext *types.APIContext, code int, obj interface{}) {
	local := true
	if !strings.HasPrefix(settings.ServerVersion.Get(), "v") {
		local = false
	}
	h.start(apiContext, code, obj)
	headerString := start
	hostURL := settings.APIUIHostURL.Get()
	apiUIVersion := settings.APIUIVersion.Get() + "/"
	if local {
		hostURL = ""
		apiUIVersion = ""
	}
	headerString = strings.Replace(headerString, "%API_UI_HOST_URL%", hostURL, 2)
	headerString = strings.Replace(headerString, "%API_UI_VERSION_PATH%", apiUIVersion, 2)

	schemaSchema := apiContext.Schemas.Schema(&builtin.Version, "schema")
	if schemaSchema != nil {
		headerString = strings.Replace(headerString, "%SCHEMAS%", apiContext.URLBuilder.Collection(schemaSchema, apiContext.Version), 1)
		apiContext.Response.Write([]byte(headerString))
	}
	h.Body(apiContext, apiContext.Response, obj)
	if schemaSchema != nil {
		apiContext.Response.Write(end)
	}
}

func AddAPIUIWriter(server *api.Server) {
	server.ResponseWriters["html"] = &ResponseWriter{
		EncodingResponseWriter: writer.EncodingResponseWriter{
			Encoder:     types.JSONEncoder,
			ContentType: "application/json",
		},
	}
}
