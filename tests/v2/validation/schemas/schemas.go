package schemas

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/kubectl"
)

const (
	schema              = "schema"
	schemaDefinition    = "schemaDefinition"
	apiGroupEndpoint    = "apigroup"
	localCluster        = "local"
	projectSchemaID     = "management.cattle.io.project"
	namespaceSchemaID   = "namespace"
	autoscalingSchemaID = "autoscaling.horizontalpodautoscaler"
	customSchemaID      = "stable.example.com.crontab"
	customCrdName       = "crontabs.stable.example.com"
	crdCreateFilePath   = "./resources/crd_create.yaml"
	twoSecondTimeout    = 2 * time.Second
)

var exceptionMap = map[string]bool{
	"applyOutput":              true,
	"applyInput":               true,
	"apiRoot":                  true,
	"chartActionOutput":        true,
	"chartInstall":             true,
	"chartInstallAction":       true,
	"chartUpgrade":             true,
	"chartUpgradeAction":       true,
	"chartUninstallAction":     true,
	"count":                    true,
	"generateKubeconfigOutput": true,
	"schema":                   true,
	"schemaDefinition":         true,
	"subscribe":                true,
	"userpreference":           true,
}

func getSchemaByID(client *rancher.Client, clusterID, existingSchemaID string) (map[string]interface{}, error) {
	return getJSONResponse(client, clusterID, schema, existingSchemaID)
}

func getSchemaDefinitionByID(client *rancher.Client, clusterID, existingSchemaID string) (map[string]interface{}, error) {
	return getJSONResponse(client, clusterID, schemaDefinition, existingSchemaID)
}

func getAPIGroupInfoByAPIGroupName(client *rancher.Client, clusterID, apiGroupName string) (map[string]interface{}, error) {
	return getJSONResponse(client, clusterID, apiGroupEndpoint, apiGroupName)
}

func accessSchemaDefinitionForEachSchema(client *rancher.Client, schemasCollection *v1.SteveCollection, clusterID string) ([]string, error) {
	var failedSchemaDefinitions []string
	var err error
	var schemaID string

	for _, schema := range schemasCollection.Data {
		schemaID = schema.JSONResp["id"].(string)

		if _, exists := exceptionMap[schemaID]; !exists {
			_, err = getSchemaByID(client, clusterID, schemaID)
			if err != nil {
				return nil, err
			}
			_, err = getSchemaDefinitionByID(client, clusterID, schemaID)
			if err != nil {
				failedSchemaDefinitions = append(failedSchemaDefinitions, schemaID)
			}
		}
	}
	return failedSchemaDefinitions, nil
}

func checkPreferredVersion(client *rancher.Client, schemasCollection *v1.SteveCollection, clusterID string) ([][]string, error) {
	var failedSchemaPreferredVersionCheck [][]string

	for _, schema := range schemasCollection.Data {
		schemaID := schema.JSONResp["id"].(string)

		if _, exists := exceptionMap[schemaID]; !exists {
			schemaInfo, err := getSchemaByID(client, clusterID, schemaID)
			if err != nil {
				return nil, err
			}
			schemaVersion := schemaInfo["attributes"].(map[string]interface{})["version"].(string)

			apiGroupName := schemaInfo["attributes"].(map[string]interface{})["group"].(string)
			if apiGroupName == "" {
				continue
			}
			apiGroupInfo, err := getAPIGroupInfoByAPIGroupName(client, clusterID, apiGroupName)
			if err != nil {
				return nil, err
			}
			preferredVersion := apiGroupInfo["preferredVersion"].(map[string]interface{})["version"].(string)

			if schemaVersion != preferredVersion {
				failedSchemaPreferredVersionCheck = append(failedSchemaPreferredVersionCheck, []string{
					"Schema ID: " + schemaID + ",",
					"Schema Version: " + schemaVersion + ";",
					"API Group Name: " + apiGroupName + ",",
					"API Group Version: " + preferredVersion,
				})
			}
		}
	}
	return failedSchemaPreferredVersionCheck, nil
}

func getJSONResponse(client *rancher.Client, clusterID, endpointType, existingID string) (map[string]interface{}, error) {
	rancherURL := client.RancherConfig.Host
	token := client.RancherConfig.AdminToken

	baseURL := fmt.Sprintf("https://%s", rancherURL)
	if clusterID != localCluster {
		baseURL = fmt.Sprintf("%s/k8s/clusters/%s", baseURL, clusterID)
	}

	var httpURL string
	if endpointType == apiGroupEndpoint {
		httpURL = fmt.Sprintf("%s/apis/%s", baseURL, existingID)
	} else {
		httpURL = fmt.Sprintf("%s/v1/%s/%s", baseURL, endpointType, existingID)
	}

	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	byteObject, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonObject map[string]interface{}
	if err := json.Unmarshal(byteObject, &jsonObject); err != nil {
		return nil, err
	}

	return jsonObject, nil
}

func checkSchemaFields(schemasCollection *v1.SteveCollection) error {
	for _, schema := range schemasCollection.Data {
		schemaID := schema.JSONResp["id"].(string)
		if !exceptionMap[schemaID] {
			if schema.JSONResp["resourceFields"] != nil {
				return errors.New("resourceFields should be null for schema " + schemaID)
			}

			if match, _ := regexp.MatchString(`\.[vV][0-9]+`, schemaID); match {
				return errors.New("child schema should not be listed")
			}
		}
	}
	return nil
}

func checkChildSchemasNotListed(client *rancher.Client, clusterID string, childSchemas map[string]bool, schemaResponse map[string]interface{}) error {
	for childSchemaID := range childSchemas {
		if _, found := schemaResponse[childSchemaID]; found {
			return errors.New("child schema should not be listed")
		}

		_, err := getSchemaByID(client, clusterID, childSchemaID)
		if err == nil {
			return errors.New("expected an error when fetching child schema")
		}
		errStatus := strings.Split(err.Error(), ": ")[1]
		if errStatus != "404" {
			return errors.New("unexpected error status: " + errStatus)
		}
	}
	return nil
}

func checkExpectedDefinitions(expectedDefinitions map[string]bool, schemaResponse map[string]interface{}) error {
	for definitionID := range expectedDefinitions {
		definitionData, exists := schemaResponse["definitions"].(map[string]interface{})[definitionID]
		if !exists {
			return fmt.Errorf("Expected definition %s not found in schemaResponse", definitionID)
		}

		resourceFields, resourceFieldsExist := definitionData.(map[string]interface{})["resourceFields"]
		if !resourceFieldsExist || resourceFields == nil {
			return fmt.Errorf("ResourceFields are nil for definition %s", definitionID)
		}

		_, resourceMethodsExist := definitionData.(map[string]interface{})["resourceMethods"]
		if resourceMethodsExist {
			return fmt.Errorf("ResourceMethods field exists for definition %s", definitionID)
		}

		_, collectionMethodsExist := definitionData.(map[string]interface{})["collectionMethods"]
		if collectionMethodsExist {
			return fmt.Errorf("CollectionMethods field exists for definition %s", definitionID)
		}

		for fieldName, fieldData := range resourceFields.(map[string]interface{}) {
			fieldType := fieldData.(map[string]interface{})["type"].(string)
			if fieldType == "integer" || fieldType == "number" {
				return fmt.Errorf("field %s should not be of type %s in definition %s", fieldName, fieldType, definitionID)
			}
		}
	}
	return nil
}

func getChildSchemasForProject() map[string]bool {
	return map[string]bool{
		"io.cattle.management.v3.Project":                                          true,
		"io.cattle.management.v3.Project.spec":                                     true,
		"io.cattle.management.v3.Project.spec.resourceQuota":                       true,
		"io.cattle.management.v3.Project.spec.resourceQuota.usedLimit":             true,
		"io.cattle.management.v3.Project.spec.resourceQuota.limit":                 true,
		"io.cattle.management.v3.Project.spec.namespaceDefaultResourceQuota":       true,
		"io.cattle.management.v3.Project.spec.namespaceDefaultResourceQuota.limit": true,
		"io.cattle.management.v3.Project.spec.containerDefaultResourceLimit":       true,
		"io.cattle.management.v3.Project.status":                                   true,
		"io.cattle.management.v3.Project.status.conditions":                        true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ManagedFieldsEntry":                  true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":                          true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference":                      true,
	}
}

func getChildSchemasForNamespace() map[string]bool {
	return map[string]bool{
		"io.k8s.api.core.v1.NamespaceCondition":                   true,
		"io.k8s.api.core.v1.NamespaceSpec":                        true,
		"io.k8s.api.core.v1.NamespaceStatus":                      true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ManagedFieldsEntry": true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":         true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference":     true,
	}
}

func getChildSchemasForCronTab() map[string]bool {
	return map[string]bool{
		"com.example.stable.v2.CronTab":                           true,
		"com.example.stable.v2.CronTab.spec":                      true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ManagedFieldsEntry": true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta":         true,
		"io.k8s.apimachinery.pkg.apis.meta.v1.OwnerReference":     true,
	}
}

func createCRD(client *rancher.Client, crdCreateFilePath string) error {
	crdYAML, err := os.ReadFile(crdCreateFilePath)
	if err != nil {
		return err
	}

	yamlInput := &management.ImportClusterYamlInput{
		YAML: string(crdYAML),
	}
	apply := []string{"kubectl", "apply", "-f", "/root/.kube/my-pod.yaml"}
	_, err = kubectl.Command(client, yamlInput, localCluster, apply, "")
	if err != nil {
		return err
	}

	return nil
}
