package namespacecompose

import (
	"encoding/json"
	"strings"

	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/compose"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	composeTokenPrefix = "compose-token-"
	description        = "token for compose"
	url                = "https://localhost:%v/v3"
)

type Lifecycle struct {
	HTTPSPortGetter common.KubeConfigGetter
	UserManager     user.Manager
	TokenClient     v3.TokenInterface
	UserClient      v3.UserInterface
	ComposeClient   projectv3.NamespaceComposeConfigInterface
	ClusterName     string
}

func Register(user *config.UserContext, portGetter common.KubeConfigGetter) {
	composeClient := user.Management.Project.NamespaceComposeConfigs("")
	tokenClient := user.Management.Management.Tokens("")
	userClient := user.Management.Management.Users("")
	l := Lifecycle{
		HTTPSPortGetter: portGetter,
		UserManager:     user.Management.UserManager,
		TokenClient:     tokenClient,
		UserClient:      userClient,
		ComposeClient:   composeClient,
		ClusterName:     user.ClusterName,
	}
	composeClient.AddHandler("namespace-compose-controller", l.sync)
}

func (l Lifecycle) sync(key string, obj *projectv3.NamespaceComposeConfig) error {
	if key == "" || obj == nil {
		return nil
	}
	clusterName := strings.Split(obj.Spec.ProjectName, ":")[0]
	if clusterName != l.ClusterName {
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

func (l Lifecycle) Create(obj *projectv3.NamespaceComposeConfig) (*projectv3.NamespaceComposeConfig, error) {
	userID := obj.Annotations["field.cattle.io/creatorId"]
	user, err := l.UserClient.Get(userID, metav1.GetOptions{})
	if err != nil {
		return obj, err
	}
	token, err := l.UserManager.EnsureToken(composeTokenPrefix+user.Name, description, user.Name)
	if err != nil {
		return obj, err
	}
	var dataMap interface{}
	if err := Unmarshal([]byte(obj.Spec.RancherCompose), &dataMap); err != nil {
		return obj, err
	}

	bytes, err := json.Marshal(dataMap)
	if err != nil {
		return obj, err
	}
	config := &compose.Config{}
	if err := json.Unmarshal(bytes, config); err != nil {
		return obj, err
	}
	if err := namespaceUp(token, l.HTTPSPortGetter.GetHTTPSPort(), obj.Spec.InstallNamespace, obj.Spec.ProjectName, config); err != nil {
		return obj, err
	}
	v3.ComposeConditionExecuted.True(obj)
	return obj, nil
}

var (
	supportedSchemas = map[string]bool{
		"ingresses":              true,
		"services":               true,
		"dnsRecords":             true,
		"pods":                   true,
		"deployments":            true,
		"replicationControllers": true,
		"replicaSets":            true,
		"statefulSets":           true,
		"jobs":                   true,
		"cronJobs":               true,
		"workloads":              true,
		"configMaps":             true,
	}

	preferOrder = []string{"configMap", "workload", "deployment", "cronJob", "job", "statefulSet", "replicaSet", "replicationController", "service", "dnsRecord", "ingress"}
)

func namespaceUp(token string, port int, namespace, projectID string, config *compose.Config) error {
	rawData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	rawMap := map[string]interface{}{}
	if err := json.Unmarshal(rawData, &rawMap); err != nil {
		return err
	}
	delete(rawMap, "version")
	baseProjectClient, err := clientbase.NewAPIClient(&clientbase.ClientOpts{
		URL:      fmt.Sprintf(url, port) + "/project/" + projectID,
		TokenKey: token,
		Insecure: true,
	})
	if err != nil {
		return err
	}

	// referenceMap is a map of schemaType with name -> id value
	referenceMap := map[string]map[string]string{}

	for _, schemaKey := range preferOrder {
		schema, ok := baseProjectClient.Types[schemaKey]
		if !ok {
			continue
		}
		v, ok := rawMap[schema.PluralName]
		if !ok {
			continue
		}
		value, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		for name, data := range value {
			dataMap, ok := data.(map[string]interface{})
			if !ok {
				break
			}
			if err := common.ReplaceGlobalReference(schema, dataMap, referenceMap, &baseProjectClient); err != nil {
				return err
			}
			dataMap["name"] = name
			dataMap["namespaceId"] = namespace
			// have to deal with special case for ingress
			if schemaKey == "ingress" {
				var ingress client.Ingress
				if err := convert.ToObj(dataMap, &ingress); err != nil {
					return err
				}
				for _, rule := range ingress.Rules {
					for _, path := range rule.Paths {
						for i := range path.WorkloadIDs {
							path.WorkloadIDs[i] = referenceMap["workload"][path.WorkloadIDs[i]]
						}
					}
				}
				dataMap, err = convert.EncodeToMap(ingress)
				if err != nil {
					return err
				}
			}
			id := ""
			respObj := map[string]interface{}{}
			if err := baseProjectClient.Create(schemaKey, dataMap, &respObj); err != nil && !strings.Contains(err.Error(), "already exist") {
				return err
			} else if err != nil && strings.Contains(err.Error(), "already exist") {
				continue
			}
			v, ok := respObj["id"]
			if !ok {
				return errors.Errorf("id is missing after creating %s obj", schemaKey)
			}
			id = v.(string)
			if f, ok := WaitCondition[schemaKey]; ok {
				if err := f(&baseProjectClient, id, schemaKey); err != nil {
					return err
				}
			}
		}
		// fill in reference map name -> id
		if err := common.FillInReferenceMap(&baseProjectClient, schemaKey, referenceMap, nil); err != nil {
			return err
		}
	}
	return nil
}

var WaitCondition = map[string]func(baseClient *clientbase.APIBaseClient, id, schemaType string) error{}
