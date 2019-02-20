package multiclusterapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

func SetMemberStore(ctx context.Context, schema *types.Schema, mgmt *config.ScaledContext) {
	providers.Configure(ctx, mgmt)
	userLister := mgmt.Management.Users("").Controller().Lister()

	t := &transform.Store{
		Store: schema.Store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			membersMapSlice, found := values.GetSlice(data, "members")
			if !found {
				return data, nil
			}
			for _, m := range membersMapSlice {
				if userID, ok := m["userId"].(string); ok && userID != "" {
					u, err := userLister.Get("", userID)
					if err != nil {
						logrus.Errorf("problem retrieving user for member %v from cache during member transformation: %v", data, err)
						return data, nil
					}
					// filtering and keeping system user accounts out of the members list
					for _, pid := range u.PrincipalIDs {
						if strings.HasPrefix(pid, "system://") {
							if opt != nil && opt.Options["ByID"] == "true" {
								return nil, httperror.NewAPIError(httperror.NotFound, "resource not found")
							}
							return nil, nil
						}
					}
				}
			}
			return data, nil
		},
	}

	s := &Store{
		Store:      t,
		auth:       requests.NewAuthenticator(ctx, mgmt),
		crtbLister: mgmt.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		prtbLister: mgmt.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
	}

	schema.Store = s
}

type Store struct {
	types.Store
	auth       requests.Authenticator
	crtbLister v3.ClusterRoleTemplateBindingLister
	prtbLister v3.ProjectRoleTemplateBindingLister
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := s.setDisplayName(apiContext, schema, data)
	if err != nil {
		return nil, err
	}
	data, err = s.checkAndSetRoles(apiContext, data)
	if err != nil {
		return nil, err
	}
	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	data, err := s.setDisplayName(apiContext, schema, data)
	if err != nil {
		return nil, err
	}
	data, err = s.checkAndSetRoles(apiContext, data)
	if err != nil {
		return nil, err
	}
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) setDisplayName(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	membersMapSlice, found := values.GetSlice(data, client.MultiClusterAppFieldMembers)
	if !found {
		return data, nil
	}
	token, err := s.auth.TokenFromRequest(apiContext.Request)
	if err != nil {
		return nil, err
	}
	if token.AuthProvider == providers.LocalProvider {
		// getPrincipal if called, will be called on local auth provider, and will not find a user with external auth principal ID
		// hence returning, since the only thing this method does is setting display name by getting it from the external auth provider.
		return data, nil
	}
	for _, m := range membersMapSlice {
		if principalID, ok := m[client.MemberFieldUserPrincipalID].(string); ok && principalID != "" && !strings.HasPrefix(principalID, "local://") {
			princ, err := providers.GetPrincipal(principalID, *token)
			if err != nil {
				return nil, err
			}
			if princ.DisplayName != "" {
				values.PutValue(m, princ.DisplayName, "displayName")
			}
		}
	}
	return data, nil
}

func (s *Store) checkAndSetRoles(apiContext *types.APIContext, data map[string]interface{}) (map[string]interface{}, error) {
	// if no roles are provided, default to roles that the creator of mcapp has in all of its target projects/clusters
	roles := convert.ToStringSlice(data[client.MultiClusterAppFieldRoles])
	if len(roles) == 0 {
		callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
		targetProjects := make(map[string]bool)
		clustersOfTargetProjects := make(map[string]bool)
		rolesToAddMap := make(map[string]bool)
		targInterface := convert.ToMapSlice(data[client.MultiClusterAppFieldTargets])
		for _, t := range targInterface {
			target := convert.ToString(t[client.TargetFieldProjectID])
			split := strings.SplitN(target, ":", 2)
			if len(split) != 2 {
				return nil, fmt.Errorf("invalid project name: %v", target)
			}
			clusterName := split[0]
			projectName := split[1]
			if !clustersOfTargetProjects[clusterName] {
				// get roles from this cluster for this creator
				crtbs, err := s.crtbLister.List(clusterName, labels.NewSelector())
				if err != nil {
					return nil, err
				}
				for _, crtb := range crtbs {
					if crtb.UserName == callerID && !rolesToAddMap[crtb.RoleTemplateName] {
						rolesToAddMap[crtb.RoleTemplateName] = true
					}
				}
				clustersOfTargetProjects[clusterName] = true
			}
			if !targetProjects[projectName] {
				// get roles from this project for this creator
				prtbs, err := s.prtbLister.List(projectName, labels.NewSelector())
				if err != nil {
					return nil, err
				}
				for _, prtb := range prtbs {
					if prtb.UserName == callerID && !rolesToAddMap[prtb.RoleTemplateName] {
						rolesToAddMap[prtb.RoleTemplateName] = true
					}
				}
				targetProjects[projectName] = true
			}
		}
		rolesToAdd := make([]string, len(rolesToAddMap))
		i := 0
		for role := range rolesToAddMap {
			rolesToAdd[i] = role
			i++
		}
		values.PutValue(data, rolesToAdd, client.MultiClusterAppFieldRoles)
	}
	return data, nil
}
