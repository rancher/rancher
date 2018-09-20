package globalrolebinding

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
)

const (
	grbByUserIndex = "auth.management.cattle.io/grbByUser"
	gdbAdminRole   = "admin"
)

func NewGRBValidator(management *config.ScaledContext) types.Validator {
	grbInformer := management.Management.GlobalRoleBindings("").Controller().Informer()
	grbIndexers := map[string]cache.IndexFunc{
		grbByUserIndex: grbByUser,
	}
	grbInformer.AddIndexers(grbIndexers)

	validator := &Validator{
		grLister:    management.Management.GlobalRoles("").Controller().Lister(),
		grbIndexer:  grbInformer.GetIndexer(),
		userManager: management.UserManager,
	}

	return validator.Validator
}

type Validator struct {
	grLister    v3.GlobalRoleLister
	grbIndexer  cache.Indexer
	userManager user.Manager
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	currentUserID := v.userManager.GetUser(request)
	globalRoleID := data[client.GlobalRoleBindingFieldGlobalRoleID].(string)

	tgrRole, _ := v.grLister.Get("", globalRoleID)
	targetRules := tgrRole.Rules

	grbRoles, err := v.grbIndexer.ByIndex(grbByUserIndex, currentUserID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting roles: %v", err))
	}

	ownerRules := []rbacv1.PolicyRule{}

	for _, grbRole := range grbRoles {
		roleName := grbRole.(*v3.GlobalRoleBinding).GlobalRoleName
		grRole, _ := v.grLister.Get("", roleName)
		ownerRules = append(ownerRules, grRole.Rules...)
	}

	if isRuleListSubset(targetRules, ownerRules) {
		return nil
	}

	logrus.Infof("Permission denied applying Global Role Binding...")

	targetRulesJSON, _ := json.MarshalIndent(targetRules, ": ", "  ")
	logrus.Infof("target rules: %s", string(targetRulesJSON))

	ownerRulesJSON, _ := json.MarshalIndent(ownerRules, ": ", "  ")
	logrus.Infof("owner rules: %s", string(ownerRulesJSON))

	targetUserID := data[client.GlobalRoleBindingFieldUserID].(string)
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied for role '%s' on user '%s'", globalRoleID, targetUserID))
}

func grbByUser(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.UserName}, nil
}

func isRuleListSubset(targetRules []rbacv1.PolicyRule, ownerRules []rbacv1.PolicyRule) bool {
	for _, tRule := range targetRules {
		if !isRuleSubset(tRule, ownerRules) {
			return false
		}
	}
	return true
}

func isRuleSubset(targetRule rbacv1.PolicyRule, ownerRules []rbacv1.PolicyRule) bool {
	for _, oRule := range ownerRules {
		if !containsRule(oRule.NonResourceURLs, targetRule.NonResourceURLs) {
			logrus.Infof("GRB Failed NonResourceURLs check")
			continue
		}
		if !containsRule(oRule.Resources, targetRule.Resources) {
			logrus.Infof("GRB Failed Resources check")
			continue
		}
		if !containsRule(oRule.ResourceNames, targetRule.ResourceNames) {
			logrus.Infof("GRB Failed ResourceNames check")
			continue
		}
		if !containsRule(oRule.APIGroups, targetRule.APIGroups) {
			logrus.Infof("GRB Failed APIGroups check")
			continue
		}
		if !containsRule(oRule.Verbs, targetRule.Verbs) {
			logrus.Infof("GRB Failed Verbs check")
			continue
		}
		return true
	}
	return false
}

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func containsAll(a []string, b []string) bool {
	for _, n := range b {
		if !contains(a, n) {
			return false
		}
	}
	return true
}

func containsRule(owner []string, target []string) bool {
	return contains(owner, "*") || containsAll(owner, target)
}
