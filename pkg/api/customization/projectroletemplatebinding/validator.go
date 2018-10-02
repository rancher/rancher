package projectroletemplatebinding

import (
	"encoding/json"
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

const (
	projectRoleTemplateBindingsByPrincipalAndUserIndex = "auth.management.cattle.io/projectRoleTemplateBindingByPrincipalAndUser"
)

func NewAuthzProjectRoleTemplateBindingValidator(management *config.ScaledContext) types.Validator {
	projectRoleTemplateBindingInformer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	projectRoleTemplateBindingIndexers := map[string]cache.IndexFunc{
		projectRoleTemplateBindingsByPrincipalAndUserIndex: projectRoleTemplateBindingsByPrincipalAndUser,
	}
	projectRoleTemplateBindingInformer.AddIndexers(projectRoleTemplateBindingIndexers)

	validator := &Validator{
		roleTemplateLister:                management.Management.RoleTemplates("").Controller().Lister(),
		projectRoleTemplateBindingIndexer: projectRoleTemplateBindingInformer.GetIndexer(),
		userManager:                       management.UserManager,
	}

	return validator.ProjectRoleTemplateBindingValidator
}

type Validator struct {
	roleTemplateLister                v3.RoleTemplateLister
	projectRoleTemplateBindingIndexer cache.Indexer
	userManager                       user.Manager
}

func (v *Validator) getProjectRoleTemplateBindingRules(userID string) ([]rbac.PolicyRule, error) {
	projectRoleTemplateBindings, err := v.projectRoleTemplateBindingIndexer.ByIndex(projectRoleTemplateBindingsByPrincipalAndUserIndex, userID)
	if err != nil {
		return nil, err
	}
	rules := []rbac.PolicyRule{}
	for _, projectRoleBinding := range projectRoleTemplateBindings {
		roleName := projectRoleBinding.(*v3.ProjectRoleTemplateBinding).RoleTemplateName
		roleTemplate, _ := v.roleTemplateLister.Get("", roleName)
		rules = append(rules, convertRules(roleTemplate.Rules)...)
	}
	return rules, nil
}

func (v *Validator) ProjectRoleTemplateBindingValidator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	currentUserID := v.userManager.GetUser(request)
	globalRoleID, ok := data[client.ProjectRoleTemplateBindingFieldRoleTemplateID].(string)

	if !ok {
		return httperror.NewAPIError(httperror.MissingRequired, "Request does not have a valid roleTemplateId")
	}

	targetRoleTemplate, err := v.roleTemplateLister.Get("", globalRoleID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting role template: %v", err))
	}

	targetRules := convertRules(targetRoleTemplate.Rules)

	ownerRules, err := v.getProjectRoleTemplateBindingRules(currentUserID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting roles: %v", err))
	}

	rulesCover, failedRules := rbacregistryvalidation.Covers(ownerRules, targetRules)
	if rulesCover {
		return nil
	}

	logrus.Infof("Permission denied applying Project Role Binding...")

	targetRulesJSON, _ := json.MarshalIndent(targetRules, ": ", "  ")
	logrus.Infof("target rules: %s", string(targetRulesJSON))

	ownerRulesJSON, _ := json.MarshalIndent(ownerRules, ": ", "  ")
	logrus.Infof("owner rules: %s", string(ownerRulesJSON))

	failedRulesJSON, _ := json.MarshalIndent(failedRules, ": ", "  ")
	logrus.Infof("failed rules: %s", string(failedRulesJSON))

	userPrincipalID := data[client.ClusterRoleTemplateBindingFieldUserPrincipalID]
	userID := data[client.ClusterRoleTemplateBindingFieldUserID]
	targetUserID := ""
	if userPrincipalID != nil {
		targetUserID = userPrincipalID.(string)
	} else if userID != nil {
		targetUserID = userID.(string)
	}
	return nil
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied for role '%s' on user '%s'", globalRoleID, targetUserID))
}

func projectRoleTemplateBindingsByPrincipalAndUser(obj interface{}) ([]string, error) {
	var principals []string
	b, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if b.GroupPrincipalName != "" {
		principals = append(principals, b.GroupPrincipalName)
	}
	if b.UserPrincipalName != "" {
		principals = append(principals, b.UserPrincipalName)
	}
	if b.UserName != "" {
		principals = append(principals, b.UserName)
	}
	return principals, nil
}

func convertRules(rbacv1Rules []rbacv1.PolicyRule) []rbac.PolicyRule {
	convertedRules := make([]rbac.PolicyRule, len(rbacv1Rules))
	for i := range rbacv1Rules {
		err := rbacv1helpers.Convert_v1_PolicyRule_To_rbac_PolicyRule(&rbacv1Rules[i], &convertedRules[i], nil)
		if err != nil {
			return nil
		}
	}
	return convertedRules
}
