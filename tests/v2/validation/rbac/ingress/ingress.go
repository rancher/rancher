package ingress

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	normantypes "github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/ingresses"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v2 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
)

// getSettingValueByName retrieves a setting value from Rancher by its name.
func getSettingValueByName(name string) string {
	session := session.NewSession()
	client, err := rancher.NewClient("", session)
	if err != nil {
		log.Errorf("Failed to create client for getting setting %s: %v", name, err)
		return ""
	}

	setting, err := client.Management.Setting.ByID(name)
	if err != nil {
		log.Errorf("Failed to get setting %s: %v", name, err)
		return ""
	}

	if setting == nil {
		log.Errorf("Setting %s not found", name)
		return ""
	}

	return setting.Value
}

// getIngressIPDomain determines the appropriate ingress IP domain based on server version.
func getIngressIPDomain() string {
	serverVersion := getSettingValueByName("server-version")
	if strings.HasPrefix(serverVersion, "v") {
		if strings.Contains(serverVersion, "head") {
			serverVersion = strings.Split(serverVersion, "-")[0]
		} else {
			serverVersion = strings.Join(strings.Split(serverVersion, ".")[:3], ".")
		}
	}

	parsedVersion, err := version.ParseSemantic(serverVersion)
	if err != nil {
		log.Errorf("Error parsing server version %s: %v", serverVersion, err)
		return ingresses.IngressIPDomainV25
	}

	if parsedVersion.GreaterThan(version.MustParseSemantic("v2.5.9")) {
		return ingresses.IngressIPDomainV26
	}
	return ingresses.IngressIPDomainV25
}

// validateIngressIPDomain validates that the ingress host has the correct IP domain suffix.
func validateIngressIPDomain(t *testing.T, k8sIngress *networkingv1.Ingress) {
	expectedDomain := getIngressIPDomain()
	actualHost := k8sIngress.Spec.Rules[0].Host

	log.Infof("Validating ingress IP domain - Expected domain suffix: %s, Actual host: %s", expectedDomain, actualHost)

	require.True(t, strings.HasSuffix(actualHost, expectedDomain),
		"Host %s should end with %s", actualHost, expectedDomain)
}

// convertClusterToSteveObject converts a management Cluster to a SteveAPIObject.
func convertClusterToSteveObject(cluster *management.Cluster) *v1.SteveAPIObject {
	return &v1.SteveAPIObject{
		Resource: normantypes.Resource{
			ID:   cluster.ID,
			Type: "management.cattle.io.cluster",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "management.cattle.io/v3",
		},
		ObjectMeta: v1.ObjectMeta{
			ObjectMeta: metav1.ObjectMeta{
				Name: cluster.Name,
			},
		},
	}
}

// verifyCleanup checks if all resources in the given namespace have been properly cleaned up.
func verifyCleanup(steveClient *v1.Client, namespace *corev1.Namespace) error {
	if namespace == nil {
		log.Debug("No namespace provided for cleanup verification")
		return nil
	}

	namespaceName := namespace.Name
	fieldSelector := fmt.Sprintf("metadata.namespace=%s", namespaceName)
	log.Infof("Verifying cleanup for namespace: %s", namespaceName)

	ingressList, err := steveClient.SteveType("networking.k8s.io.ingress").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err == nil && len(ingressList.Data) > 0 {
		return fmt.Errorf("found %d remaining ingresses in namespace %s", len(ingressList.Data), namespaceName)
	}

	serviceList, err := steveClient.SteveType("v1.service").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err == nil && len(serviceList.Data) > 0 {
		return fmt.Errorf("found %d remaining services in namespace %s", len(serviceList.Data), namespaceName)
	}

	deployList, err := steveClient.SteveType("apps.deployment").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err == nil && len(deployList.Data) > 0 {
		return fmt.Errorf("found %d remaining deployments in namespace %s", len(deployList.Data), namespaceName)
	}

	podList, err := steveClient.SteveType("v1.pod").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err == nil && len(podList.Data) > 0 {
		return fmt.Errorf("found %d remaining pods in namespace %s", len(podList.Data), namespaceName)
	}

	log.Infof("All resources successfully cleaned up in namespace: %s", namespaceName)
	return nil
}

// cleanupResources performs a thorough cleanup of all resources in the given namespace.
func cleanupResources(steveClient *v1.Client, namespace *corev1.Namespace) error {
	if steveClient == nil {
		return fmt.Errorf("steve client is nil")
	}

	if namespace == nil {
		log.Warn("No namespace provided for cleanup")
		return nil
	}

	namespaceName := namespace.Name
	fieldSelector := fmt.Sprintf("metadata.namespace=%s", namespaceName)
	log.Infof("Starting cleanup for namespace: %s", namespaceName)

	ingressList, err := steveClient.SteveType("networking.k8s.io.ingress").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err != nil {
		return fmt.Errorf("failed to list ingresses: %v", err)
	}
	for _, ing := range ingressList.Data {
		if err := steveClient.SteveType("networking.k8s.io.ingress").Delete(&ing); err != nil {
			log.Errorf("Failed to delete ingress %s: %v", ing.Name, err)
			continue
		}
	}
	log.Infof("Deleted %d ingresses", len(ingressList.Data))
	time.Sleep(2 * time.Second)

	serviceList, err := steveClient.SteveType("v1.service").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}
	for _, svc := range serviceList.Data {
		if err := steveClient.SteveType("v1.service").Delete(&svc); err != nil {
			log.Errorf("Failed to delete service %s: %v", svc.Name, err)
			continue
		}
	}
	log.Infof("Deleted %d services", len(serviceList.Data))
	time.Sleep(2 * time.Second)

	deployList, err := steveClient.SteveType("apps.deployment").List(url.Values{
		"fieldSelector": []string{fieldSelector},
	})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %v", err)
	}
	for _, deploy := range deployList.Data {
		if err := steveClient.SteveType("apps.deployment").Delete(&deploy); err != nil {
			log.Errorf("Failed to delete deployment %s: %v", deploy.Name, err)
			continue
		}
	}
	log.Infof("Deleted %d deployments", len(deployList.Data))

	log.Info("Waiting for pods to terminate...")
	waitErr := wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
		podList, err := steveClient.SteveType("v1.pod").List(url.Values{
			"fieldSelector": []string{fieldSelector},
		})
		if err != nil {
			log.Errorf("Failed to list pods: %v", err)
			return false, nil
		}
		if len(podList.Data) > 0 {
			log.Infof("Waiting for %d pods to terminate...", len(podList.Data))
			return false, nil
		}
		return true, nil
	})
	if waitErr != nil {
		log.Errorf("Timeout waiting for pods to terminate in namespace %s", namespaceName)
	}

	log.Infof("Deleting namespace: %s", namespaceName)
	nsObj, err := steveClient.SteveType("v1.namespace").ByID(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %v", namespaceName, err)
	}
	if err := steveClient.SteveType("v1.namespace").Delete(nsObj); err != nil {
		return fmt.Errorf("failed to delete namespace %s: %v", namespaceName, err)
	}

	waitErr = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
		_, err := steveClient.SteveType("v1.namespace").ByID(namespaceName)
		if err != nil {
			return true, nil
		}
		log.Infof("Waiting for namespace %s to be removed...", namespaceName)
		return false, nil
	})
	if waitErr != nil {
		return fmt.Errorf("timeout waiting for namespace %s to be removed", namespaceName)
	}

	log.Infof("Cleanup completed successfully for namespace: %s", namespaceName)
	return nil
}

// setupTest creates necessary test resources including project, namespace and downstream client.
func setupTest(client *rancher.Client, clusterID string, name string) (*v1.Client, *management.Project, *corev1.Namespace, error) {
	log.Infof("Setting up test resources for: %s", name)

	project, namespace, err := projects.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		log.Errorf("Failed to create project and namespace for test %s: %v", name, err)
		return nil, nil, nil, fmt.Errorf("project and namespace creation failed: %v", err)
	}
	log.Infof("Created project %s and namespace %s", project.Name, namespace.Name)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		log.Errorf("Failed to get downstream client for test %s: %v", name, err)
		return nil, nil, nil, fmt.Errorf("downstream client creation failed: %v", err)
	}

	return steveClient, project, namespace, nil
}

// createWorkload creates a deployment workload with the specified parameters.
func createWorkload(client *rancher.Client, clusterID, namespace string, replicas int) (*v2.Deployment, error) {
	log.Infof("Creating workload in namespace %s with %d replicas", namespace, replicas)
	deployment, err := deployment.CreateDeployment(client, clusterID, namespace, replicas, "nginx", "", false, false, true, false)
	if err != nil {
		log.Errorf("Failed to create workload: %v", err)
		return nil, fmt.Errorf("workload creation failed: %v", err)
	}
	log.Infof("Successfully created workload: %s", deployment.Name)
	return deployment, nil
}

// generateUniqueHost generates a unique hostname for ingress testing.
func generateUniqueHost(t *testing.T, prefix string) string {
	testName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	randomStr := namegen.AppendRandomString("")
	if prefix == "" {
		prefix = "test"
	}
	host := fmt.Sprintf("%s-%s-%s.example.com", prefix, testName, randomStr)
	log.Infof("Generated unique host: %s", host)
	return host
}

// findIngressSchema finds and returns the ingress schema from the schema collection.
func findIngressSchema(schemas *v1.SteveCollection) *v1.SteveAPIObject {
	log.Info("Searching for ingress schema")
	for _, schema := range schemas.Data {
		if schema.ID == "networking.k8s.io.ingress" {
			log.Info("Found ingress schema")
			return &schema
		}
	}
	log.Warn("Ingress schema not found")
	return nil
}

// validateSchemaVerbs validates that the schema contains all required verbs.
func validateSchemaVerbs(t *testing.T, attributes map[string]interface{}) {
	log.Info("Validating schema verbs")
	verbs, ok := attributes["verbs"].([]interface{})
	require.True(t, ok, "Failed to get verbs from attributes")

	verbsStr := make([]string, len(verbs))
	for i, v := range verbs {
		verbsStr[i] = v.(string)
	}

	requiredVerbs := []string{"create", "update", "get", "list"}
	for _, verb := range requiredVerbs {
		require.Contains(t, verbsStr, verb, "Required verb %s not found in schema", verb)
	}
	log.Info("Schema verbs validation successful")
}

// validateSchemaColumns validates schema columns and returns the field mapping.
func validateSchemaColumns(t *testing.T, attributes map[string]interface{}) map[string]map[string]interface{} {
	log.Info("Validating schema columns")
	columns, ok := attributes["columns"].([]interface{})
	require.True(t, ok, "Failed to get columns from attributes")

	fields := make(map[string]map[string]interface{})
	for _, col := range columns {
		column := col.(map[string]interface{})
		fieldName := column["name"].(string)
		fields[fieldName] = column
		log.Debugf("Found field: %s", fieldName)
	}
	log.Infof("Validated %d schema columns", len(fields))
	return fields
}

// validateExpectedFields validates that all expected fields are present in the schema.
func validateExpectedFields(t *testing.T, fields map[string]map[string]interface{}) {
	log.Info("Validating expected fields")
	expectedFields := []string{"Name", "Class", "Hosts", "Address", "Ports", "Age"}
	for _, field := range expectedFields {
		_, exists := fields[field]
		require.True(t, exists, "Required field %s not found in schema", field)
	}

	nameField, ok := fields["Name"]
	require.True(t, ok, "Name field not found in schema")
	require.Equal(t, "string", nameField["type"], "Name field should be of type string")
	log.Info("Expected fields validation successful")
}

// validateNamespaceScope validates that the schema is namespace scoped.
func validateNamespaceScope(t *testing.T, attributes map[string]interface{}) {
	log.Info("Validating namespace scope")
	namespaced, ok := attributes["namespaced"].(bool)
	require.True(t, ok && namespaced, "Schema should be namespaced")
	log.Info("Namespace scope validation successful")
}

// UpdateDeploymentWorkload updates a deployment's replicas and image
func updateDeploymentWorkload(client *rancher.Client, clusterID, namespace, name string, replicas int, image string) (*v1.SteveAPIObject, error) {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream client: %v", err)
	}

	deployObj, err := steveClient.SteveType("apps.deployment").ByID(fmt.Sprintf("%s/%s", namespace, name))
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %v", err)
	}

	var deployment appsv1.Deployment
	if err := v1.ConvertToK8sType(deployObj, &deployment); err != nil {
		return nil, fmt.Errorf("failed to convert deployment: %v", err)
	}

	replicas32 := int32(replicas)
	deployment.Spec.Replicas = &replicas32
	deployment.Spec.Template.Spec.Containers[0].Image = image

	updatedDeployObj, err := steveClient.SteveType("apps.deployment").Update(deployObj, &deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment: %v", err)
	}

	return updatedDeployObj, nil
}
