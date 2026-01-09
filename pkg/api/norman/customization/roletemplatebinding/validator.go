package roletemplatebinding

import (
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/labels"
)

func NewPRTBValidator(management *config.ScaledContext) types.Validator {
	return newValidator(management, client.ProjectRoleTemplateBindingFieldRoleTemplateID, "project")
}

func NewCRTBValidator(management *config.ScaledContext) types.Validator {
	return newValidator(management, client.ClusterRoleTemplateBindingFieldRoleTemplateID, "cluster")
}

func newValidator(management *config.ScaledContext, field string, context string) types.Validator {
	validator := &validator{
		roleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
		field:              field,
		context:            context,
		crtbLister:         management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
	}

	return validator.validator
}

type validator struct {
	roleTemplateLister v3.RoleTemplateLister
	field              string
	context            string
	crtbLister         v3.ClusterRoleTemplateBindingLister
}

func (v *validator) validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	roleTemplateName := data[v.field]
	if roleTemplateName == nil && request.Method == http.MethodPut {
		return nil
	}

	roleTemplate, err := v.validateRoleTemplateBinding(roleTemplateName)
	if err != nil {
		return err
	}

	if roleTemplate.Context != v.context {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("Cannot reference context [%s] from [%s] context",
			roleTemplate.Context, v.context))
	}

	if request.Method == http.MethodPut {
		return nil
	}

	userID, _ := data["userId"].(string)
	userPrincipalID, _ := data["userPrincipalId"].(string)
	groupID, _ := data["groupId"].(string)
	groupPrincipalID, _ := data["groupPrincipalId"].(string)

	hasUserTarget := userID != "" || userPrincipalID != ""
	hasGroupTarget := groupID != "" || groupPrincipalID != ""

	if (hasUserTarget && hasGroupTarget) || (!hasUserTarget && !hasGroupTarget) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must target a user [userId]/[userPrincipalId] "+
			"OR a group [groupId]/[groupPrincipalId]")
	}

	// check for duplicate crtb
	if v.context == "cluster" && request.Method == http.MethodPost {
		if clusterID, ok := data[client.ClusterRoleTemplateBindingFieldClusterID].(string); ok && clusterID != "" {
			existingCRTBs, err := v.crtbLister.List(clusterID, labels.Everything())
			if err != nil {
				return err
			}

			for _, crtb := range existingCRTBs {
				if crtb.RoleTemplateName != roleTemplateName {
					continue
				}

				isDuplicate := (userID != "" && userID == crtb.UserName) ||
					(userPrincipalID != "" && userPrincipalID == crtb.UserPrincipalName) ||
					(groupID != "" && groupID == crtb.GroupName) ||
					(groupPrincipalID != "" && groupPrincipalID == crtb.GroupPrincipalName)

				if isDuplicate {
					if crtb.DeletionTimestamp != nil {
						// This handles the UI race condition where a binding is removed and re-added in the same "Save" operation.
						// We ignore the conflict if the conflicting object is already being deleted.
						continue
					}
					return httperror.NewAPIError(httperror.Conflict, "Cluster role template binding for this role already exists")
				}
			}
		}

	}
	return nil
}

func (v *validator) validateRoleTemplateBinding(obj interface{}) (*v3.RoleTemplate, error) {
	roleTemplateID, ok := obj.(string)
	if !ok {
		return nil, httperror.NewAPIError(httperror.MissingRequired, "Request does not have a valid roleTemplateId")
	}

	roleTemplate, err := v.roleTemplateLister.Get("", roleTemplateID)
	if err != nil {
		return nil, httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting role template: %v", err))
	}

	if roleTemplate.Locked {
		return nil, httperror.NewAPIError(httperror.InvalidState, "Role is locked and cannot be assigned")
	}

	return roleTemplate, nil
}
