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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

const (
	projectRoleTemplateBindingsByPrincipalAndUserIndex = "auth.management.cattle.io/projectRoleTemplateBindingByPrincipalAndUser"
	roleBindingsByNamespaceAndNameIndex                = "auth.management.cattle.io/roleBindingByNamespaceAndName"
	namespaceByProjectIDIndex                          = "auth.management.cattle.io/namespaceByProjectID"
)

func NewAuthzProjectRoleTemplateBindingValidator(management *config.ScaledContext) types.Validator {
	projectRoleTemplateBindingInformer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	projectRoleTemplateBindingIndexers := map[string]cache.IndexFunc{
		projectRoleTemplateBindingsByPrincipalAndUserIndex: projectRoleTemplateBindingsByPrincipalAndUser,
	}
	projectRoleTemplateBindingInformer.AddIndexers(projectRoleTemplateBindingIndexers)

	roleBindingInformer := management.RBAC.RoleBindings("").Controller().Informer()
	roleBindingIndexers := map[string]cache.IndexFunc{
		roleBindingsByNamespaceAndNameIndex: roleBindingsByNamespaceAndName,
	}
	roleBindingInformer.AddIndexers(roleBindingIndexers)

	namespaceInformer := management.Core.Namespaces("").Controller().Informer()
	namespaceIndexers := map[string]cache.IndexFunc{
		namespaceByProjectIDIndex: namespaceByProjectID,
	}
	namespaceInformer.AddIndexers(namespaceIndexers)

	validator := &Validator{
		roleTemplateLister:                management.Management.RoleTemplates("").Controller().Lister(),
		projectRoleTemplateBindingIndexer: projectRoleTemplateBindingInformer.GetIndexer(),
		userManager:                       management.UserManager,
		namespaceIndexer:                  namespaceInformer.GetIndexer(),
	}

	return validator.ProjectRoleTemplateBindingValidator
}

type Validator struct {
	roleTemplateLister                v3.RoleTemplateLister
	projectRoleTemplateBindingIndexer cache.Indexer
	userManager                       user.Manager
	namespaceIndexer                  cache.Indexer
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
	roleTemplateID, ok := data[client.ProjectRoleTemplateBindingFieldRoleTemplateID].(string)
	if !ok {
		return httperror.NewAPIError(httperror.MissingRequired, "Request does not have a valid roleTemplateId")
	}

	projectID, ok := data[client.ProjectRoleTemplateBindingFieldProjectID].(string)
	namespaceRecords, err := v.namespaceIndexer.ByIndex(namespaceByProjectIDIndex, projectID)
	logrus.Infof("Found %d spaces", len(namespaceRecords))
	for _, namespaceRecord := range namespaceRecords {
		logrus.Infof("Found space %v", namespaceRecord)
	}
	if len(namespaceRecords) != 1 {
		return httperror.NewAPIError(httperror.MissingRequired, "Request does not have a valid namespace")
	}

	namespace := namespaceRecords[0].(corev1.Namespace).Name
	logrus.Infof("Found space %s", namespace)

	targetRoleTemplate, err := v.roleTemplateLister.Get("", roleTemplateID)
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
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied for role '%s' on user '%s'", roleTemplateID, targetUserID))
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

func roleBindingsByNamespaceAndName(obj interface{}) ([]string, error) {
	var bindings []string
	roleBinding, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		return []string{}, nil
	}
	for _, subject := range roleBinding.Subjects {
		bindings = append(bindings, roleBinding.ObjectMeta.Namespace+"."+subject.Name)
	}
	return bindings, nil
}

func namespaceByProjectID(obj interface{}) ([]string, error) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		logrus.Infof("Failed to convert namespace")
		return []string{}, nil
	}
	projectID := namespace.ObjectMeta.Annotations["field.cattle.io/projectId"]
	logrus.Infof("Found namespace %s %v", projectID, namespace)
	return []string{projectID}, nil
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
