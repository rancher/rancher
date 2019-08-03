package catalog

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/rancher/norman/types/convert"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/templatecontent"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getChartIconURL(t TemplateWrapper, templateVersionName string) (string, error) {
	templateVersion, err := t.TemplateVersions.Get("", templateVersionName)
	if err != nil {
		return "", err
	}
	id := strings.Split(templateVersion.Name, "-")[1]
	tag := templateVersion.Spec.Files[id+"/Chart.yaml"]
	data, err := templatecontent.GetTemplateFromTag(tag, t.TemplateContentClients)
	if err != nil {
		return "", err
	}
	resp := findIconURL(data)
	if resp == "" {
		return "", fmt.Errorf("icon not found")
	}
	return resp, nil
}

func findIconURL(s string) string {
	i := strings.Index(s, "icon:")
	if i < 0 {
		return ""
	}
	s = s[i:]
	target := strings.Split(s, "\n")[0]
	i = strings.Index(target, "http")
	if i < 0 {
		return ""
	}
	target = target[i:]
	return target
}

// indexInterfaceSlice takes a list of interfaces, returns the first, asserts its a map[string]interface{}
// and uses the provided index to retrieve the value
func indexInterfaceSlice(value interface{}, key string) string {
	v := convert.Singular(value)
	if v != nil {
		v2, ok := v.(map[string]interface{})
		if ok {
			return v2[key].(string)
		}
	}
	return ""
}

func (t TemplateWrapper) TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	// version links
	resource.Values["versionLinks"] = extractVersionLinks(apiContext, resource)

	//icon
	delete(resource.Values, "icon")
	ic, ok := resource.Values["iconFilename"]
	if ok {
		if strings.HasPrefix(ic.(string), "http:") || strings.HasPrefix(ic.(string), "https:") {
			resource.Links["icon"] = ic.(string)
		} else {
			// before marking an icon as from file, ensure this is not a chart from an upgrade
			// if so, use the url in the chart.yaml
			ver, ok := resource.Values["versions"]
			if ok {
				v := indexInterfaceSlice(ver, "version")
				res, err := getChartIconURL(t, resource.ID+"-"+v)
				if err == nil {
					resource.Links["icon"] = res
					// also update the template itself
					template, err := t.TemplateClients.Get(resource.ID, metav1.GetOptions{})
					if err != nil {
						logrus.Warnf("unable to get template %s", err)
					} else {
						template = template.DeepCopy()
						template.Spec.IconFilename = res
						_, err = t.TemplateClients.Update(template)
						if err != nil {
							logrus.Warnf("unable to update template icon %s", template.Name)
						}
					}
				} else { //this is chart with a bundled icon
					resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
				}
			} else { //if no versions are found, fallback to hosting the icon
				resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
			}
		}
	} else {
		resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
	}

	//catalog link
	catalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.CatalogType)
	catalogName := strings.Split(resource.ID, "-")[0]
	resource.Links["catalog"] = apiContext.URLBuilder.ResourceLinkByID(catalogSchema, catalogName)

	// delete category
	delete(resource.Values, "category")

	// delete versions
	delete(resource.Values, "versions")
}

type TemplateWrapper struct {
	TemplateContentClients v3.TemplateContentInterface
	TemplateVersions       v3.TemplateVersionLister
	TemplateClients        v3.TemplateInterface
}

func (t TemplateWrapper) TemplateIconHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "icon":
		template := &client.Template{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
			return err
		}
		if strings.HasPrefix(template.IconFilename, "http:") || strings.HasPrefix(template.IconFilename, "https:") {
			http.Error(apiContext.Response, "", http.StatusNoContent)
			return nil
		}

		data, err := templatecontent.GetTemplateFromTag(template.Icon, t.TemplateContentClients)
		if err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339, template.Created)
		if err != nil {
			return err
		}
		value, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return err
		}
		iconReader := bytes.NewReader(value)
		apiContext.Response.Header().Set("Cache-Control", "private, max-age=604800")
		// add security headers (similar to raw.githubusercontent)
		apiContext.Response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		apiContext.Response.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, t, iconReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}
