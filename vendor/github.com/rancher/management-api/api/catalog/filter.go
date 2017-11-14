package catalog

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
)

func TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	delete(resource.Values, "icon")
	resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
}

func TemplateIconHandler(apiContext *types.APIContext) error {
	if apiContext.Link != "icon" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	template := &client.Template{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
		return err
	}

	icon, err := base64.StdEncoding.DecodeString(template.Icon)
	if err != nil {
		return err
	}
	iconReader := bytes.NewReader(icon)
	http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, time.Time{}, iconReader)

	return nil
}
