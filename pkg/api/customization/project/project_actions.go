package project

import (
	"fmt"

	"net/http"

	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "setpodsecuritypolicytemplate")
}

type Handler struct {
	ProjectClient v3.ProjectInterface
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "setpodsecuritypolicytemplate":
		return h.setPodSecurityPolicyTemplate(actionName, action, apiContext)
	}

	return errors.Errorf("bad action %v", actionName)
}

func (h *Handler) setPodSecurityPolicyTemplate(actionName string, action *types.Action, request *types.APIContext) error {
	actionInput, err := parse.ReadBody(request.Request)
	if err != nil {
		return err
	}

	schema := request.Schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateProjectBindingType)
	if schema == nil {
		return fmt.Errorf("no %v store available", client.PodSecurityPolicyTemplateProjectBindingType)
	}

	binding, err := h.createOrUpdateBinding(request, schema, actionInput)
	if err != nil {
		return err
	}

	err = h.updateProjectPSPTID(request, actionInput)
	if err != nil {
		return err
	}

	request.WriteResponse(http.StatusOK, binding)

	return nil
}

func (h *Handler) createOrUpdateBinding(request *types.APIContext, schema *types.Schema,
	actionInput map[string]interface{}) (map[string]interface{}, error) {
	bindings, err := schema.Store.List(request, request.Schema, &types.QueryOptions{})
	if err != nil {
		return nil, err
	}

	var binding map[string]interface{}
	for _, candidate := range bindings {
		if candidate["projectId"] == request.ID {
			binding = candidate
			break
		}
	}

	if binding == nil {
		binding, err = h.createNewBinding(request, schema, actionInput)
		if err != nil {
			return nil, fmt.Errorf("error creating binding: %v", err)
		}
	} else {
		binding, err = h.updateBinding(binding, request, schema, actionInput)
		if err != nil {
			return nil, err
		}
	}

	return binding, nil
}

func (h *Handler) updateProjectPSPTID(request *types.APIContext, actionInput map[string]interface{}) error {
	project, err := request.Schema.Store.ByID(request, request.Schema, request.ID)
	if err != nil {
		return fmt.Errorf("error getting current project: %v", err)
	}

	status, ok := project["status"]
	if !ok {
		project["status"] = map[string]interface{}{
			"podSecurityPolicyTemplate": actionInput["podSecurityPolicyTemplate"],
		}
	} else {
		if statusAsMap, ok := status.(map[string]interface{}); ok {
			statusAsMap["podSecurityPolicyTemplate"] = actionInput["podSecurityPolicyTemplate"]
		} else {
			return fmt.Errorf("error looking up podSecurityPolicyTemplate")
		}
	}

	_, err = request.Schema.Store.Update(request, request.Schema, project, request.ID)
	if err != nil {
		return fmt.Errorf("error updating project: %v", err)
	}

	return nil
}

func (h *Handler) createNewBinding(request *types.APIContext, schema *types.Schema,
	actionInput map[string]interface{}) (map[string]interface{}, error) {
	binding := make(map[string]interface{})
	binding["projectId"] = request.ID
	binding["podSecurityPolicyTemplate"] = actionInput["podSecurityPolicyTemplate"]

	name := fmt.Sprintf("%v-%v-pspt-binding", strings.Replace(request.ID, ":", "-",
		-1), actionInput["podSecurityPolicyTemplate"])
	binding["name"] = name
	binding["id"] = name

	return schema.Store.Create(request, schema, binding)
}

func (h *Handler) updateBinding(binding map[string]interface{}, request *types.APIContext, schema *types.Schema,
	actionInput map[string]interface{}) (map[string]interface{}, error) {
	binding["podSecurityPolicyTemplate"] = actionInput["podSecurityPolicyTemplate"]
	key := "id"
	if id, ok := binding[key].(string); ok && id != "" {
		var err error
		binding, err = schema.Store.Update(request, schema, binding, id)
		if err != nil {
			return nil, fmt.Errorf("error updating binding: %v", err)
		}
	} else {
		return nil, fmt.Errorf("could not parse: %v", binding[key])
	}

	return binding, nil
}
