package secret

import (
	"context"
	"strings"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/api/store/cert"
	client "github.com/rancher/rancher/pkg/types/client/project/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

type Store struct {
	types.Store
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	t := convert.ToString(data["kind"])
	t = strings.TrimPrefix(t, "namespaced")
	t = convert.Uncapitalize(t)
	data["kind"] = t
	return s.Store.Create(apiContext, schema, data)
}

func NewNamespacedSecretStore(ctx context.Context, clientGetter proxy.ClientGetter) *Store {
	secretsStore := proxy.NewProxyStore(ctx, clientGetter,
		config.UserStorageContext,
		[]string{"api"},
		"",
		"v1",
		"Secret",
		"secrets")
	return &Store{
		Store: &transform.Store{
			Store: secretsStore,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				if data == nil {
					return data, nil
				}
				anns, _ := data["annotations"].(map[string]interface{})
				if anns["secret.user.cattle.io/secret"] == "true" {
					return nil, nil
				}
				if data["projectId"] != nil {
					fieldProjectID, _ := data["projectId"].(string)
					projectID := strings.Split(fieldProjectID, ":")
					id := ""
					if len(projectID) == 2 {
						id = projectID[1]
					}
					if id == data["namespaceId"] {
						return nil, nil
					}
				}
				parts := strings.Split(convert.ToString(data["type"]), "/")
				parts[len(parts)-1] = "namespaced" + convert.Capitalize(parts[len(parts)-1])
				data["type"] = strings.Join(parts, "/")
				if data["type"] != client.NamespacedCertificateType {
					return data, nil
				}
				if err := cert.AddCertInfo(data); err != nil {
					logrus.Errorf("Error %v parsing cert %v. Will not display correctly in UI", err, data["name"])
					return data, nil
				}
				return data, nil
			},
		},
	}
}
