package globalrolebinding

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"

	"k8s.io/client-go/tools/cache"
)

const (
	grbByUserIndex = "auth.management.cattle.io/grbByUser"
)

func NewGRBValidator(management *config.ScaledContext) types.Validator {
	grbInformer := management.Management.GlobalRoleBindings("").Controller().Informer()
	grbIndexers := map[string]cache.IndexFunc{
		grbByUserIndex: grbByUser,
	}
	grbInformer.AddIndexers(grbIndexers)
	grbIndexer := grbInformer.GetIndexer()

	validator := &Validator{
		grbIndexer:  grbIndexer,
		userManager: management.UserManager,
	}

	return validator.Validator
}

type Validator struct {
	grbIndexer  cache.Indexer
	userManager user.Manager
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	currentUserID := v.userManager.GetUser(request)
	globalRoleID := data[client.GlobalRoleBindingFieldGlobalRoleID].(string)

	roles, err := v.grbIndexer.ByIndex(grbByUserIndex, currentUserID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting roles: %v", err))
	}

	rolesMap := make(map[string]bool)
	for _, role := range roles {
		currentRoleName := role.(*v3.GlobalRoleBinding).GlobalRoleName
		rolesMap[currentRoleName] = true
	}

	if rolesMap["admin"] || rolesMap[globalRoleID] {
		return nil
	}

	targetUserID := data[client.GlobalRoleBindingFieldUserID].(string)
	return httperror.NewAPIError(httperror.InvalidState, fmt.Sprintf("Permission denied adding role %s to user %s", globalRoleID, targetUserID))
}

func grbByUser(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.UserName}, nil
}
