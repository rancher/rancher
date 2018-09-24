package clusterroletemplatebinding

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
	crtbsByPrincipalAndUserIndex = "auth.management.cattle.io/crtbByPrincipalAndUser"
)

func NewAuthzCRTBValidator(management *config.ScaledContext) types.Validator {
	crtbInformer := management.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		crtbsByPrincipalAndUserIndex: crtbsByPrincipalAndUser,
	}
	crtbInformer.AddIndexers(crtbIndexers)

	validator := &Validator{
		rtLister:    management.Management.RoleTemplates("").Controller().Lister(),
		crtbIndexer: crtbInformer.GetIndexer(),
		userManager: management.UserManager,
	}

	return validator.Validator
}

type Validator struct {
	rtLister    v3.RoleTemplateLister
	crtbIndexer cache.Indexer
	userManager user.Manager
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	currentUserID := v.userManager.GetUser(request)
	globalRoleID := data[client.ClusterRoleTemplateBindingFieldRoleTemplateID].(string)

	tgrRole, _ := v.rtLister.Get("", globalRoleID)
	targetRules := convertRules(tgrRole.Rules)

	grbRoles, err := v.crtbIndexer.ByIndex(crtbsByPrincipalAndUserIndex, currentUserID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting roles: %v", err))
	}

	ownerRules := []rbac.PolicyRule{}

	for _, grbRole := range grbRoles {
		roleName := grbRole.(*v3.ClusterRoleTemplateBinding).RoleTemplateName
		grRole, _ := v.rtLister.Get("", roleName)
		rules := convertRules(grRole.Rules)
		ownerRules = append(ownerRules, rules...)
	}

	rulesCover, failedRules := rbacregistryvalidation.Covers(ownerRules, targetRules)
	if rulesCover {
		return nil
	}

	logrus.Infof("Permission denied applying Cluster Role Binding...")

	targetRulesJSON, _ := json.MarshalIndent(targetRules, ": ", "  ")
	logrus.Infof("target rules: %s", string(targetRulesJSON))

	ownerRulesJSON, _ := json.MarshalIndent(ownerRules, ": ", "  ")
	logrus.Infof("owner rules: %s", string(ownerRulesJSON))

	failedRulesJSON, _ := json.MarshalIndent(failedRules, ": ", "  ")
	logrus.Infof("failed rules: %s", string(failedRulesJSON))

	userPrincipalId := data[client.ClusterRoleTemplateBindingFieldUserPrincipalID]
	userId := data[client.ClusterRoleTemplateBindingFieldUserID]
	targetUserID := ""
	if userPrincipalId != nil {
		targetUserID = userPrincipalId.(string)
	} else if userId != nil {
		targetUserID = userId.(string)
	}
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied for role '%s' on user '%s'", globalRoleID, targetUserID))
}

func crtbsByPrincipalAndUser(obj interface{}) ([]string, error) {
	var principals []string
	b, ok := obj.(*v3.ClusterRoleTemplateBinding)
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
