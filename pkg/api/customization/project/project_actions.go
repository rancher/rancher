package project

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "setpodsecuritypolicytemplate")
}

type Handler struct {
	Projects      v3.ProjectInterface
	ProjectLister v3.ProjectLister
}

func (h *Handler) Actions(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "setpodsecuritypolicytemplate":
		return h.setPodSecurityPolicyTemplate(actionName, action, apiContext)
	}

	return errors.Errorf("unrecognized action %v", actionName)
}

func (h *Handler) setPodSecurityPolicyTemplate(actionName string, action *types.Action,
	request *types.APIContext) error {
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.SetPodSecurityPolicyTemplateInputType))
	if err != nil {
		return fmt.Errorf("error parse/validate action body: %v", err)
	}

	podSecurityPolicyTemplateName, ok :=
		input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName].(string)
	if !ok && input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName] != nil {
		return fmt.Errorf("could not convert: %v",
			input[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName])
	}

	schema := request.Schemas.Schema(&managementschema.Version, client.PodSecurityPolicyTemplateProjectBindingType)
	if schema == nil {
		return fmt.Errorf("no %v store available", client.PodSecurityPolicyTemplateProjectBindingType)
	}

	err = h.createOrUpdateBinding(request, schema, podSecurityPolicyTemplateName)
	if err != nil {
		return err
	}

	project, err := h.updateProjectPSPTID(request, podSecurityPolicyTemplateName)
	if err != nil {
		return fmt.Errorf("error updating PSPT ID: %v", err)
	}

	request.WriteResponse(http.StatusOK, project)

	return nil
}

func (h *Handler) createOrUpdateBinding(request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	bindings, err := schema.Store.List(request, schema, &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString(client.PodSecurityPolicyTemplateProjectBindingFieldTargetProjectName,
				types.ModifierEQ, request.ID),
		},
	})
	if err != nil {
		return fmt.Errorf("error retrieving binding: %v", err)
	}

	if podSecurityPolicyTemplateName == "" {
		for _, binding := range bindings {
			namespace, okNamespace := binding[client.PodSecurityPolicyTemplateProjectBindingFieldNamespaceId].(string)
			name, okName := binding[client.PodSecurityPolicyTemplateProjectBindingFieldName].(string)

			if okNamespace && okName {
				_, err := schema.Store.Delete(request, schema, namespace+":"+name)
				if err != nil {
					return fmt.Errorf("error deleting binding: %v", err)
				}
			} else {
				return fmt.Errorf("could not convert name or namespace field: %v %v",
					binding[client.PodSecurityPolicyTemplateProjectBindingFieldNamespaceId],
					binding[client.PodSecurityPolicyTemplateProjectBindingFieldName])
			}
		}
	} else {
		if len(bindings) == 0 {
			err = h.createNewBinding(request, schema, podSecurityPolicyTemplateName)
			if err != nil {
				return fmt.Errorf("error creating binding: %v", err)
			}
		} else {
			binding := bindings[0]

			id, ok := binding["id"].(string)
			if ok {
				split := strings.Split(id, ":")

				binding, err = schema.Store.ByID(request, schema, split[0]+":"+split[len(split)-1])
				if err != nil {
					return fmt.Errorf("error retreiving binding: %v for %v", err, bindings[0])
				}
				err = h.updateBinding(binding, request, schema, podSecurityPolicyTemplateName)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("could not convert id field: %v", binding["id"])
			}
		}
	}

	return nil
}

func (h *Handler) updateProjectPSPTID(request *types.APIContext,
	podSecurityPolicyTemplateName string) (*v3.Project, error) {

	split := strings.Split(request.ID, ":")
	project, err := h.ProjectLister.Get(split[0], split[len(split)-1])
	if err != nil {
		return nil, fmt.Errorf("error getting project: %v", err)
	}

	project.Status.PodSecurityPolicyTemplateName = podSecurityPolicyTemplateName

	return h.Projects.Update(project)
}

func (h *Handler) createNewBinding(request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	binding := make(map[string]interface{})
	binding["targetProjectId"] = request.ID
	binding["podSecurityPolicyTemplateId"] = podSecurityPolicyTemplateName
	binding["namespaceId"] = strings.Split(request.ID, ":")[0]

	binding, err := schema.Store.Create(request, schema, binding)
	return err
}

func (h *Handler) updateBinding(binding map[string]interface{}, request *types.APIContext, schema *types.Schema,
	podSecurityPolicyTemplateName string) error {
	binding[client.PodSecurityPolicyTemplateProjectBindingFieldPodSecurityPolicyTemplateName] =
		podSecurityPolicyTemplateName
	id, err := getID(binding["id"])
	if err != nil {
		return err
	}
	binding["id"] = id

	if _, ok := binding["id"].(string); ok && id != "" {
		var err error
		binding, err = schema.Store.Update(request, schema, binding, id)
		if err != nil {
			return fmt.Errorf("error updating binding: %v", err)
		}
	} else {
		return fmt.Errorf("could not parse: %v", binding["id"])
	}

	return nil
}

func getID(id interface{}) (string, error) {
	s, ok := id.(string)
	if !ok {
		return "", fmt.Errorf("could not convert %v", id)
	}

	split := strings.Split(s, ":")
	return split[0] + ":" + split[len(split)-1], nil
}
