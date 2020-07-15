package node

import (
	"bytes"
	"net/http"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"encoding/json"

	"github.com/ghodss/yaml"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/generated/compose"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configKey = "extractedConfig"
)

type DriverHandlers struct {
	NodeDriverClient v3.NodeDriverInterface
}

func (h *DriverHandlers) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	m, err := h.NodeDriverClient.GetNamespaced("", apiContext.ID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// passing nil as the resource only works because just namespace is grabbed from it and nodedriver is not namespaced
	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, nil, apiContext.Schema); err != nil {
		return err
	}

	switch actionName {
	case "activate":
		m.Spec.Active = true
		v32.NodeDriverConditionActive.Unknown(m)
	case "deactivate":
		m.Spec.Active = false
		v32.NodeDriverConditionInactive.Unknown(m)
		var newConditions []v32.Condition
		for _, cond := range m.Status.Conditions {
			// remove downloaded and installed conditions if stuck in unknown,
			// this allows state to correctly show up as inactive vs downloading/installing
			if cond.Type == "Downloaded" && cond.Status == "Unknown" {
				continue
			}
			if cond.Type == "Installed" && cond.Status == "Unknown" {
				continue
			}
			newConditions = append(newConditions, cond)
		}
		m.Status.Conditions = newConditions
	}

	_, err = h.NodeDriverClient.Update(m)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

// Formatter for NodeDriver
func (h *DriverHandlers) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		resource.AddAction(apiContext, "activate")
		resource.AddAction(apiContext, "deactivate")
	}
	resource.Links["exportYaml"] = apiContext.URLBuilder.Link("exportYaml", resource)
}

func (h DriverHandlers) ExportYamlHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "exportyaml":
		nodeDriver, err := h.NodeDriverClient.Get(apiContext.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		topkey := compose.Config{}
		topkey.Version = "v3"
		nd := client.NodeDriver{}
		if err := convert.ToObj(nodeDriver.Spec, &nd); err != nil {
			return err
		}
		topkey.NodeDrivers = map[string]client.NodeDriver{}
		topkey.NodeDrivers[nodeDriver.Spec.DisplayName] = nd
		m, err := convert.EncodeToMap(topkey)
		if err != nil {
			return err
		}
		delete(m["nodeDrivers"].(map[string]interface{})[nodeDriver.Spec.DisplayName].(map[string]interface{}), "actions")
		delete(m["nodeDrivers"].(map[string]interface{})[nodeDriver.Spec.DisplayName].(map[string]interface{}), "links")
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}

		buf, err := yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(buf)
		apiContext.Response.Header().Set("Content-Type", "text/yaml")
		http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
		return nil
	}
	return nil
}
