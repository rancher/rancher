package multiclusterapp

import (
	"context"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
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
		Store: t,
		auth:  requests.NewAuthenticator(ctx, mgmt),
	}

	schema.Store = s
}

type Store struct {
	types.Store
	auth requests.Authenticator
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := s.setDisplayName(apiContext, schema, data)
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
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) setDisplayName(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	membersMapSlice, found := values.GetSlice(data, client.MultiClusterAppFieldMembers)
	if !found {
		return data, nil
	}
	for _, m := range membersMapSlice {
		if principalID, ok := m[client.MemberFieldUserPrincipalID].(string); ok && principalID != "" && !strings.HasPrefix(principalID, "local://") {
			token, err := s.auth.TokenFromRequest(apiContext.Request)
			if err != nil {
				return nil, err
			}
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
