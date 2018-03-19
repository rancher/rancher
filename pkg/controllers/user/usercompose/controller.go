package usercompose

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
	clusterClient "github.com/rancher/types/client/cluster/v3"
	managementClient "github.com/rancher/types/client/management/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/compose"
	"github.com/rancher/types/config"
	yaml "gopkg.in/yaml.v2"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	composeTokenPrefix = "compose-token-"
	url                = "https://localhost:%v/v3"
)

type Lifecycle struct {
	HTTPSPortGetter common.KubeConfigGetter
	TokenClient     v3.TokenInterface
	UserClient      v3.UserInterface
	ComposeClient   v3.ClusterComposeConfigInterface
	ClusterName     string
}

func Register(user *config.UserContext, portGetter common.KubeConfigGetter) {
	composeClient := user.Management.Management.ClusterComposeConfigs(user.ClusterName)
	tokenClient := user.Management.Management.Tokens("")
	userClient := user.Management.Management.Users("")
	l := Lifecycle{
		HTTPSPortGetter: portGetter,
		TokenClient:     tokenClient,
		UserClient:      userClient,
		ComposeClient:   composeClient,
		ClusterName:     user.ClusterName,
	}
	composeClient.AddHandler("cluster-compose-controller", l.sync)
}

func (l Lifecycle) sync(key string, obj *v3.ClusterComposeConfig) error {
	if key == "" || obj == nil {
		return nil
	}
	if obj.Spec.ClusterName != l.ClusterName {
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

func (l Lifecycle) Create(obj *v3.ClusterComposeConfig) (*v3.ClusterComposeConfig, error) {
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
	config := &compose.Config{}
	if err := yaml.Unmarshal([]byte(obj.Spec.RancherCompose), config); err != nil {
		return obj, err
	}
	if err := clusterUp(token, l.HTTPSPortGetter.GetHTTPSPort(), obj.Spec.ClusterName, config); err != nil {
		return obj, err
	}
	v3.ComposeConditionExecuted.True(obj)
	return obj, nil
}

func clusterUp(token string, port int, clusterName string, config *compose.Config) error {
	clusterSchemas, managementSchemas, projectSchemas, err := GetSchemas(token, port)
	if err != nil {
		return err
	}
	/*
		ClusterUp should create resources:
		clusterScoped resources: namespace, persistVolume, StorageClasses
		ManagementResources that belongs to cluster: projects, clusterRole...
		ManagementResources that belongs to project: apps, projectLogging...
	*/
	schemas, err := GetSchemaMap(clusterSchemas, managementSchemas, projectSchemas)
	if err != nil {
		return err
	}

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
	allSchemas := getAllSchemas(clusterSchemas, managementSchemas, projectSchemas)
	sortedSchemas := common.SortSchema(allSchemas)

	baseClusterClient, err := clientbase.NewAPIClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port) + "/cluster",
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return err
	}
	baseManagementClient, err := clientbase.NewAPIClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port),
		TokenKey: token,
		Insecure: true,
	})
	baseProjectClient, err := clientbase.NewAPIClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port) + "/project",
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf(url, port)
	configManager := configClientManager{
		clusterSchemas:       clusterSchemas,
		managementSchemas:    managementSchemas,
		projectSchemas:       projectSchemas,
		baseClusterClient:    &baseClusterClient,
		baseManagementClient: &baseManagementClient,
		baseProjectClient:    &baseProjectClient,
		baseURL:              baseURL,
		clusterID:            clusterName,
	}

	for _, schemaKey := range sortedSchemas {
		schema, ok := schemas[schemaKey]
		if !ok {
			continue
		}
		key := schema.PluralName
		v, ok := rawMap[key]
		if !ok {
			continue
		}
		value, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		var baseClient *clientbase.APIBaseClient
		for name, data := range value {
			dataMap, ok := data.(map[string]interface{})
			if !ok {
				break
			}
			baseClient, err = configManager.ConfigBaseClient(schemaKey, dataMap, referenceMap)
			if err != nil {
				return err
			}
			if err := common.ReplaceGlobalReference(schemas[schemaKey], dataMap, referenceMap, baseClient); err != nil {
				return err
			}
			dataMap["name"] = name
			if _, ok := schemas[schemaKey].ResourceFields["clusterId"]; ok {
				dataMap["clusterId"] = clusterName
			}
			respObj := map[string]interface{}{}
			// in here we have to make sure the same name won't be created twice
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
	return nil
}

func getAllSchemas(clusterSchemas, managementSchemas, projectSchemas map[string]types.Schema) map[string]types.Schema {
	r := map[string]types.Schema{}
	for k, schema := range clusterSchemas {
		if _, ok := schema.ResourceFields["creatorId"]; !ok {
			continue
		}
		r[k] = schema
	}
	for k, schema := range managementSchemas {
		if _, ok := schema.ResourceFields["creatorId"]; !ok {
			continue
		}
		r[k] = schema
	}
	for k, schema := range projectSchemas {
		if _, ok := schema.ResourceFields["creatorId"]; !ok {
			continue
		}
		r[k] = schema
	}
	return r
}

func GetSchemas(token string, port int) (map[string]types.Schema, map[string]types.Schema, map[string]types.Schema, error) {
	cc, err := clusterClient.NewClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port) + "/clusters",
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	mc, err := managementClient.NewClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port),
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	pc, err := projectClient.NewClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port) + "/projects",
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return cc.Types, mc.Types, pc.Types, nil
}

type configClientManager struct {
	clusterSchemas       map[string]types.Schema
	managementSchemas    map[string]types.Schema
	projectSchemas       map[string]types.Schema
	baseClusterClient    *clientbase.APIBaseClient
	baseManagementClient *clientbase.APIBaseClient
	baseProjectClient    *clientbase.APIBaseClient
	clusterID            string
	baseURL              string
}

// GetBaseClient config a baseClient with a special base url based on schema type
func (c configClientManager) ConfigBaseClient(schemaType string, data map[string]interface{}, referenceMap map[string]map[string]string) (*clientbase.APIBaseClient, error) {
	if _, ok := c.clusterSchemas[schemaType]; ok {
		c.baseClusterClient.Opts.URL = c.baseURL + fmt.Sprintf("/cluster/%s", c.clusterID)
		return c.baseClusterClient, nil
	}

	if _, ok := c.managementSchemas[schemaType]; ok {
		return c.baseManagementClient, nil
	}

	if _, ok := c.projectSchemas[schemaType]; ok {
		projectName := common.GetValue(data, "projectId")
		if _, ok := referenceMap["project"]; !ok {
			filter := map[string]string{
				"clusterId": c.clusterID,
			}
			if err := common.FillInReferenceMap(c.baseManagementClient, "project", referenceMap, filter); err != nil {
				return nil, err
			}
		}
		projectID := referenceMap["project"][projectName]
		c.baseProjectClient.Opts.URL = c.baseURL + fmt.Sprintf("/projects/%s", projectID)
		return c.baseProjectClient, nil
	}
	return nil, errors.Errorf("schema type %s not supported", schemaType)
}

func GetSchemaMap(clusterSchemas, managementSchemas, projectSchemas map[string]types.Schema) (map[string]types.Schema, error) {
	r := map[string]types.Schema{}
	for k, schema := range clusterSchemas {
		r[k] = schema
	}
	for k, schema := range managementSchemas {
		_, underCluster := schema.ResourceFields["clusterId"]
		_, underProject := schema.ResourceFields["projectId"]
		if underCluster || underProject {
			r[k] = schema
		}
	}
	for k, schema := range projectSchemas {
		if isManagedTypes(schema) {
			r[k] = schema
		}
	}
	return r, nil
}

func isManagedTypes(schema types.Schema) bool {
	// hard-code for now
	types := map[string]bool{
		"app":                        true,
		"secret":                     true,
		"dockerCredential":           true,
		"certificate":                true,
		"pipelineExecutionLog":       true,
		"pipelineExecution":          true,
		"pipeline":                   true,
		"projectAlert":               true,
		"projectLogging":             true,
		"projectNetworkPolicie":      true,
		"projectRoleTemplateBinding": true,
	}
	return types[schema.ID]
}
