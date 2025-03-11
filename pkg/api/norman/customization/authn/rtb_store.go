package authn

import (
	"context"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/requests"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

func SetRTBStore(ctx context.Context, schema *types.Schema, mgmt *config.ScaledContext) {
	providers.Configure(ctx, mgmt)
	userLister := mgmt.Management.Users("").Controller().Lister()

	t := &transform.Store{
		Store: schema.Store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			if id, ok := data[client.ClusterRoleTemplateBindingFieldUserID].(string); ok && id != "" {
				u, err := userLister.Get("", id)
				if err != nil {
					if !errors.IsNotFound(err) {
						logrus.Errorf("problem retrieving user for CRTB %v from cache during CRTB transformation: %v", data, err)
					}
					return data, nil
				}

				for _, pid := range u.PrincipalIDs {
					if strings.HasPrefix(pid, "system://") {
						if opt != nil && opt.Options["ByID"] == "true" {
							return nil, httperror.NewAPIError(httperror.NotFound, "resource not found")
						}
						return nil, nil
					}
				}
			}
			return data, nil
		},
	}

	s := &Store{
		Store: t,
		auth:  requests.NewAuthenticator(ctx, clusterrouter.GetClusterID, mgmt),
	}

	schema.Store = s
}

type Store struct {
	types.Store
	auth requests.Authenticator
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if principalID, ok := data[client.ClusterRoleTemplateBindingFieldUserPrincipalID].(string); ok && principalID != "" && !strings.HasPrefix(principalID, "local://") {
		token, err := s.auth.TokenFromRequest(apiContext.Request)
		if err != nil {
			return nil, err
		}
		princ, err := providers.GetPrincipal(principalID, *token)
		if err != nil {
			return nil, err
		}
		if princ.DisplayName != "" {
			values.PutValue(data, princ.DisplayName, "annotations", "auth.cattle.io/principal-display-name")
		}
	}

	return s.Store.Create(apiContext, schema, data)
}
