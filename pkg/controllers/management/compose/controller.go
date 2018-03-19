package compose

import (
	"encoding/json"
	"strings"

	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	hutils "github.com/rancher/rancher/pkg/controllers/user/helm/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementClient "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	composeTokenPrefix = "compose-token-"
	url                = "https://localhost:%v/v3"
)

// Lifecycle for GlobalComposeConfig is a controller which watches composeConfig and execute the yaml config and create a bunch of global resources. There is no sync logic between yaml file and resources, which means config is only executed once. And resource is not deleted even if the compose config is deleted.
type Lifecycle struct {
	TokenClient     v3.TokenInterface
	UserClient      v3.UserInterface
	HTTPSPortGetter common.KubeConfigGetter
	ComposeClient   v3.GlobalComposeConfigInterface
}

func Register(managementContext *config.ManagementContext, portGetter common.KubeConfigGetter) {
	composeClient := managementContext.Management.GlobalComposeConfigs("")
	tokenClient := managementContext.Management.Tokens("")
	userClient := managementContext.Management.Users("")
	l := Lifecycle{
		HTTPSPortGetter: portGetter,
		TokenClient:     tokenClient,
		UserClient:      userClient,
		ComposeClient:   composeClient,
	}
	composeClient.AddHandler("compose-controller", l.sync)
}

func (l Lifecycle) sync(key string, obj *v3.GlobalComposeConfig) error {
	if key == "" || obj == nil {
		return nil
	}
	obj, err := l.Create(obj)
	if err != nil {
		return &controller.ForgetError{
			Err: err,
		}
	}
	_, err = l.ComposeClient.Update(obj)
	return err
}

func (l Lifecycle) Create(obj *v3.GlobalComposeConfig) (*v3.GlobalComposeConfig, error) {
	userID := obj.Annotations["field.cattle.io/creatorId"]
	user, err := l.UserClient.Get(userID, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}
	token := ""
	if t, err := l.TokenClient.Get(composeTokenPrefix+user.Name, metav1.GetOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return obj, err
	} else if kerrors.IsNotFound(err) {
		token, err = hutils.GenerateToken(user, composeTokenPrefix, l.TokenClient)
		if err != nil {
			return obj, err
		}
	} else {
		token = t.Name + ":" + t.Token
	}
	globalClient, err := constructGlobalClient(token, l.HTTPSPortGetter.GetHTTPSPort())
	if err != nil {
		return obj, err
	}
	config := &compose.Config{}
	if err := yaml.Unmarshal([]byte(obj.Spec.RancherCompose), config); err != nil {
		return obj, err
	}
	if err := up(globalClient, config); err != nil {
		return obj, err
	}
	v3.ComposeConditionExecuted.True(obj)
	return obj, nil
}

func CreateGlobalResources(globalCLient *managementClient.Client, config *compose.Config) error {
	// schema map contains all the schemas
	schemas := GetSchemaMap(globalCLient)

	// referenceMap is a map of schemaType with name -> id value
	referenceMap := map[string]map[string]string{}

	rawData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	rawMap := map[string]interface{}{}
	if err := json.Unmarshal(rawData, &rawMap); err != nil {
		return err
	}
	delete(rawMap, "version")
	// find all resources that has no references
	sortedSchemas := common.SortSchema(schemas)
	for _, schemaKey := range sortedSchemas {
		key := schemaKey + "s"
		if v, ok := rawMap[key]; ok {
			if !isGlobalResource(schemaKey, globalCLient) {
				logrus.Warnf("%s is not a global resource. Skipping", schemaKey)
				continue
			}
			value, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			baseClient := &globalCLient.APIBaseClient
			for name, data := range value {
				dataMap, ok := data.(map[string]interface{})
				if !ok {
					break
				}
				if err := common.ReplaceGlobalReference(schemas[schemaKey], dataMap, referenceMap, baseClient); err != nil {
					return err
				}
				dataMap["name"] = name
				respObj := map[string]interface{}{}
				// in here we have to make sure the same name won't be created twice
				// todo: right now the global resource can be created with the same name
				created := map[string]string{}
				if err := baseClient.List(schemaKey, &types.ListOpts{}, &respObj); err != nil {
					return err
				}
				if data, ok := respObj["data"]; ok {
					if collections, ok := data.([]interface{}); ok {
						for _, obj := range collections {
							if objMap, ok := obj.(map[string]interface{}); ok {
								createdName := common.GetValue(objMap, "name")
								if createdName != "" {
									created[createdName] = common.GetValue(objMap, "id")
								}
							}
						}
					}
				}

				id := ""
				if v, ok := created[name]; ok {
					id = v
				} else {
					if err := baseClient.Create(schemaKey, dataMap, &respObj); err != nil && !strings.Contains(err.Error(), "already exist") {
						return err
					} else if err != nil && strings.Contains(err.Error(), "already exist") {
						break
					}
					v, ok := respObj["id"]
					if !ok {
						return errors.Errorf("id is missing after creating %s obj", schemaKey)
					}
					id = v.(string)
				}
				if f, ok := WaitCondition[schemaKey]; ok {
					if err := f(baseClient, id, schemaKey); err != nil {
						return err
					}
				}
			}
			// fill in reference map name -> id
			if err := common.FillInReferenceMap(baseClient, schemaKey, referenceMap, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func isGlobalResource(schemaType string, globalClient *managementClient.Client) bool {
	schema, ok := globalClient.Types[schemaType]
	if !ok {
		return false
	}
	if _, ok := schema.ResourceFields["clusterId"]; ok {
		return false
	}
	if _, ok := schema.ResourceFields["projectId"]; ok {
		return false
	}
	return true
}

func GetSchemaMap(globalClient *managementClient.Client) map[string]types.Schema {
	schemas := map[string]types.Schema{}
	for k, s := range globalClient.Types {
		if _, ok := s.ResourceFields["creatorId"]; !ok {
			continue
		}
		schemas[k] = s
	}
	return schemas
}

func up(globalClient *managementClient.Client, config *compose.Config) error {
	return CreateGlobalResources(globalClient, config)
}

type ClientSet struct {
	mClient *managementClient.Client
}

func constructGlobalClient(token string, port int) (*managementClient.Client, error) {
	mClient, err := managementClient.NewClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port),
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return nil, err
	}
	return mClient, nil
}
