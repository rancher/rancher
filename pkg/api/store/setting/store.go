package setting

import (
	"fmt"
	"os"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/api/customization/setting"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/settings"
)

type Store struct {
	types.Store
	MetadataHandler kd.MetadataController
}

var MetadataSettings = map[string]bool{
	settings.KubernetesVersion.Name:            true,
	settings.KubernetesVersionsCurrent.Name:    true,
	settings.KubernetesVersionsDeprecated.Name: true,
}

func New(store types.Store, metadataHandler kd.MetadataController) types.Store {
	return &Store{
		&transform.Store{
			Store: store,
			Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
				v, ok := data["value"]
				id := convert.ToString(data["id"])
				value, ok2 := os.LookupEnv(settings.GetEnvKey(id))
				switch {
				case ok2:
					data["value"] = value
					data["customized"] = false
					data["source"] = "env"

				case !ok || v == "":
					data["value"] = data["default"]
					data["customized"] = false
					data["source"] = "default"
				default:
					data["customized"] = true
					data["source"] = "db"
				}
				return data, nil
			},
		},
		metadataHandler,
	}
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if slice.ContainsString(setting.ReadOnlySettings, id) {
		return nil, httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("Cannot delete readOnly setting %s.", id))
	}

	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if _, ok := MetadataSettings[id]; !ok {
		return s.Store.Update(apiContext, schema, data, id)
	}
	labels := map[string]interface{}{}
	if val, ok := data["labels"]; ok {
		labels = convert.ToMapInterface(val)
	}
	if val, ok := data["value"]; ok && convert.ToString(val) == "" {
		labels[kd.UserUpdateLabel] = "false"
	} else {
		if id == settings.KubernetesVersion.Name || id == settings.KubernetesVersionsCurrent.Name {
			if err := validate(id, convert.ToString(val)); err != nil {
				return nil, err
			}
		}
		labels[kd.UserUpdateLabel] = "true"
	}
	url, err := kd.GetURLSettingValue()
	if err != nil {
		msg := fmt.Sprintf("failed to get settings %v", err)
		return nil, httperror.WrapAPIError(err, httperror.ServerError, msg)
	}
	data["labels"] = labels
	data, err = s.Store.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}
	// trigger refresh for ui settings
	if err := s.MetadataHandler.Refresh(url, false); err != nil {
		msg := fmt.Sprintf("failed to refresh %v", err)
		return nil, httperror.WrapAPIError(err, httperror.ServerError, msg)
	}
	return data, nil
}

func validate(id, value string) error {
	var k8sVersion string
	var k8sCurrVersions []string

	if id == settings.KubernetesVersion.Name {
		k8sVersion = value
		k8sCurrVersions = strings.Split(settings.KubernetesVersionsCurrent.Get(), ",")
	} else {
		k8sCurrVersions = strings.Split(value, ",")
		k8sVersion = settings.KubernetesVersion.Get()
	}

	for _, curr := range k8sCurrVersions {
		if curr == k8sVersion {
			return nil
		}
	}

	return httperror.NewAPIError(httperror.MissingRequired, "default k8s-version must be present in k8s-versions-current")
}
