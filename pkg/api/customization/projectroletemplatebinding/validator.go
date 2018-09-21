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
	prtbsByPrincipalAndUserIndex = "auth.management.cattle.io/prtbByPrincipalAndUser"
)

func NewAuthzPRTBValidator(management *config.ScaledContext) types.Validator {
	prtbInformer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbsByPrincipalAndUserIndex: prtbsByPrincipalAndUser,
	}
	prtbInformer.AddIndexers(prtbIndexers)

	validator := &Validator{
		rtLister:    management.Management.RoleTemplates("").Controller().Lister(),
		prtbIndexer: prtbInformer.GetIndexer(),
		userManager: management.UserManager,
	}

	return validator.Validator
}

type Validator struct {
	rtLister    v3.RoleTemplateLister
	prtbIndexer cache.Indexer
	userManager user.Manager
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	currentUserID := v.userManager.GetUser(request)
	globalRoleID := data[client.ProjectRoleTemplateBindingFieldRoleTemplateID].(string)

	tgrRole, _ := v.rtLister.Get("", globalRoleID)
	targetRules := convertRules(tgrRole.Rules)

	grbRoles, err := v.prtbIndexer.ByIndex(prtbsByPrincipalAndUserIndex, currentUserID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting roles: %v", err))
	}

	ownerRules := []rbac.PolicyRule{}

	for _, grbRole := range grbRoles {
		roleName := grbRole.(*v3.ProjectRoleTemplateBinding).RoleTemplateName
		grRole, _ := v.rtLister.Get("", roleName)
		rules := convertRules(grRole.Rules)
		ownerRules = append(ownerRules, rules...)
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

	targetUserID := data[client.ProjectRoleTemplateBindingFieldUserPrincipalID].(string)
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied for role '%s' on user '%s'", globalRoleID, targetUserID))
}

func prtbsByPrincipalAndUser(obj interface{}) ([]string, error) {
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
