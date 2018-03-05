package authn

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

func SetRTBStore(schema *types.Schema, mgmt *config.ScaledContext) {
	userLister := mgmt.Management.Users("").Controller().Lister()

	t := &transform.Store{
		Store: schema.Store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			if id, ok := data[client.ClusterRoleTemplateBindingFieldUserId].(string); ok {
				u, err := userLister.Get("", id)
				if err != nil {
					logrus.Errorf("problem retrieving user for CRTB %v from cache during CRTB transformation: %v", data, err)
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

	schema.Store = t
}
