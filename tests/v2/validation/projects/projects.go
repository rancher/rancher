package projects

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	quotas "github.com/rancher/rancher/tests/v2/actions/kubeapi/resourcequotas"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/workloads"
	pod "github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wrangler"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dummyFinalizer                  = "dummy"
	systemProjectLabel              = "authz.management.cattle.io/system-project"
	resourceQuotaAnnotation         = "field.cattle.io/resourceQuota"
	containerDefaultLimitAnnotation = "field.cattle.io/containerDefaultResourceLimit"
	resourceQuotaStatusAnnotation   = "cattle.io/status"
)

var prtb = v3.ProjectRoleTemplateBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "",
		Namespace: "",
	},
	ProjectName:       "",
	RoleTemplateName:  "",
	UserPrincipalName: "",
}

func createProjectAndNamespace(client *rancher.Client, clusterID string, project *v3.Project) (*v3.Project, *corev1.Namespace, error) {
	createdProject, err := client.WranglerContext.Mgmt.Project().Create(project)
	if err != nil {
		return nil, nil, err
	}

	namespaceName := namegen.AppendRandomString("testns-")
	createdNamespace, err := namespaces.CreateNamespace(client, clusterID, createdProject.Name, namespaceName, "", map[string]string{}, map[string]string{})
	if err != nil {
		return nil, nil, err
	}

	return createdProject, createdNamespace, nil
}

func createProjectAndNamespaceWithQuotas(client *rancher.Client, clusterID string, namespacePodLimit, projectPodLimit string) (*v3.Project, *corev1.Namespace, error) {
	projectTemplate := projects.NewProjectTemplate(clusterID)
	projectTemplate.Spec.NamespaceDefaultResourceQuota.Limit.Pods = namespacePodLimit
	projectTemplate.Spec.ResourceQuota.Limit.Pods = projectPodLimit
	createdProject, createdNamespace, err := createProjectAndNamespace(client, clusterID, projectTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create project and namespace: %v", err)
	}

	return createdProject, createdNamespace, nil
}

func createProjectAndNamespaceWithLimits(client *rancher.Client, clusterID string, cpuLimit, cpuReservation, memoryLimit, memoryReservation string) (*v3.Project, *corev1.Namespace, error) {
	projectTemplate := projects.NewProjectTemplate(clusterID)
	projectTemplate.Spec.ContainerDefaultResourceLimit.LimitsCPU = cpuLimit
	projectTemplate.Spec.ContainerDefaultResourceLimit.RequestsCPU = cpuReservation
	projectTemplate.Spec.ContainerDefaultResourceLimit.LimitsMemory = memoryLimit
	projectTemplate.Spec.ContainerDefaultResourceLimit.RequestsMemory = memoryReservation

	createdProject, createdNamespace, err := createProjectAndNamespace(client, clusterID, projectTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create project and namespace: %v", err)
	}

	return createdProject, createdNamespace, nil
}

func createProjectRoleTemplateBinding(client *rancher.Client, user *management.User, project *v3.Project, projectRole string) (*v3.ProjectRoleTemplateBinding, error) {
	projectName := fmt.Sprintf("%s:%s", project.Namespace, project.Name)
	prtb.Name = namegen.AppendRandomString("prtb-")
	prtb.Namespace = project.Name
	prtb.ProjectName = projectName
	prtb.RoleTemplateName = projectRole
	prtb.UserPrincipalName = user.PrincipalIDs[0]

	createdProjectRoleTemplateBinding, err := rbacapi.CreateProjectRoleTemplateBinding(client, &prtb)
	if err != nil {
		return nil, err
	}

	return createdProjectRoleTemplateBinding, nil
}

func checkAnnotationExistsInNamespace(client *rancher.Client, clusterID string, namespaceName string, annotationKey string, expectedExistence bool) error {
	namespace, err := namespaces.GetNamespaceByName(client, clusterID, namespaceName)
	if err != nil {
		return err
	}

	_, exists := namespace.Annotations[annotationKey]
	if (expectedExistence && !exists) || (!expectedExistence && exists) {
		errorMessage := fmt.Sprintf("Annotation '%s' should%s exist", annotationKey, map[bool]string{true: "", false: " not"}[expectedExistence])
		return errors.New(errorMessage)
	}

	return nil
}

func getLatestStatusMessageFromDeployment(deployment *appv1.Deployment, messageType string) (string, string, error) {
	latestTime := time.Time{}
	latestMessage := ""
	latestReason := ""

	targetMessageType := appv1.DeploymentConditionType(messageType)

	for _, condition := range deployment.Status.Conditions {
		if condition.Type == targetMessageType && condition.LastUpdateTime.After(latestTime) {
			latestMessage = condition.Message
			latestReason = condition.Reason
			latestTime = condition.LastUpdateTime.Time
		}
	}

	return latestMessage, latestReason, nil
}

func checkDeploymentStatus(client *rancher.Client, clusterID, namespaceName, deploymentName, statusType, expectedStatusReason, expectedStatusMessage string, expectedReplicaCount int32) error {
	updatedDeploymentList, err := deployments.ListDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	if len(updatedDeploymentList.Items) == 0 {
		return fmt.Errorf("deployment %s not found", deploymentName)
	}

	updatedDeployment := updatedDeploymentList.Items[0]

	statusMsg, statusReason, err := getLatestStatusMessageFromDeployment(&updatedDeployment, statusType)
	if err != nil {
		return err
	}

	if !strings.Contains(statusMsg, expectedStatusMessage) {
		return fmt.Errorf("expected status message: %s, actual status message: %s", expectedStatusMessage, statusMsg)
	}

	if !strings.Contains(statusReason, expectedStatusReason) {
		return fmt.Errorf("expected status reason: %s, actual status reason: %s", expectedStatusReason, statusReason)
	}

	if updatedDeployment.Status.ReadyReplicas != expectedReplicaCount {
		return fmt.Errorf("unexpected number of ready replicas: expected %d, got %d", expectedReplicaCount, updatedDeployment.Status.ReadyReplicas)
	}

	return nil
}

func getStatusAndMessageFromAnnotation(annotation string, conditionType string) (string, string, error) {
	var annotationData map[string][]map[string]string
	if err := json.Unmarshal([]byte(annotation), &annotationData); err != nil {
		return "", "", fmt.Errorf("error parsing JSON: %v", err)
	}

	conditions, ok := annotationData["Conditions"]
	if !ok {
		return "", "", fmt.Errorf("no 'Conditions' found in annotation")
	}

	for _, condition := range conditions {
		if condition["Type"] == conditionType {
			status := condition["Status"]
			message := condition["Message"]

			return status, message, nil
		}
	}

	return "", "", fmt.Errorf("no condition of type '%s' found", conditionType)
}

func getNamespaceLimit(client *rancher.Client, clusterID string, namespaceName, annotation string) (map[string]interface{}, error) {
	namespace, err := namespaces.GetNamespaceByName(client, clusterID, namespaceName)
	if err != nil {
		return nil, err
	}

	limitAnnotation := namespace.Annotations[annotation]
	if limitAnnotation == "" {
		return nil, errors.New("annotation not found")
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(limitAnnotation), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func checkNamespaceResourceQuota(client *rancher.Client, clusterID, namespaceName string, expectedPodLimit int) error {
	quotas, err := quotas.ListResourceQuotas(client, clusterID, namespaceName, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(quotas.Items) != 1 {
		return fmt.Errorf("expected resource quota count is 1, but got %d", len(quotas.Items))
	}

	resourceList := quotas.Items[0].Spec.Hard
	actualPodLimit, ok := resourceList[corev1.ResourcePods]
	if !ok {
		return fmt.Errorf("pod limit not found in the resource quota")
	}
	podLimit := int(actualPodLimit.Value())
	if podLimit != expectedPodLimit {
		return fmt.Errorf("pod limit in the resource quota: %d does not match the expected value: %d", podLimit, expectedPodLimit)
	}

	return nil
}

func checkNamespaceResourceQuotaValidationStatus(client *rancher.Client, clusterID, namespaceName, namespacePodLimit string, expectedStatus bool, expectedErrorMessage string) error {
	namespace, err := namespaces.GetNamespaceByName(client, clusterID, namespaceName)
	if err != nil {
		return err
	}

	limitData, err := getNamespaceLimit(client, clusterID, namespace.Name, resourceQuotaAnnotation)
	if err != nil {
		return err
	}
	actualNamespacePodLimit := limitData["limit"].(map[string]interface{})["pods"]

	if actualNamespacePodLimit != namespacePodLimit {
		return fmt.Errorf("namespace pod limit mismatch in the namespace spec. expected: %s, actual: %s", namespacePodLimit, actualNamespacePodLimit)
	}

	status, message, err := getStatusAndMessageFromAnnotation(namespace.Annotations[resourceQuotaStatusAnnotation], "ResourceQuotaValidated")
	if err != nil {
		return err
	}

	if (status == "True") != expectedStatus {
		return fmt.Errorf("resource quota validation status mismatch. expected: %t, actual: %s", expectedStatus, status)
	}

	if !strings.Contains(message, expectedErrorMessage) {
		return fmt.Errorf("Error message does not contain expected substring: %s", expectedErrorMessage)
	}

	return nil
}

func getAndConvertDeployment(client *rancher.Client, clusterID string, deployment *appv1.Deployment) (*appv1.Deployment, error) {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	deploymentID := deployment.Namespace + "/" + deployment.Name
	deploymentResp, err := steveClient.SteveType(workloads.DeploymentSteveType).ByID(deploymentID)
	if err != nil {
		return nil, err
	}

	deploymentObj := &appv1.Deployment{}
	err = v1.ConvertToK8sType(deploymentResp.JSONResp, deploymentObj)
	if err != nil {
		return nil, err
	}
	return deploymentObj, nil
}

func updateProjectContainerResourceLimit(client *rancher.Client, existingProject *v3.Project, cpuLimit, cpuReservation, memoryLimit, memoryReservation string) (*v3.Project, error) {
	updatedProject := existingProject.DeepCopy()
	updatedProject.Spec.ContainerDefaultResourceLimit.LimitsCPU = cpuLimit
	updatedProject.Spec.ContainerDefaultResourceLimit.RequestsCPU = cpuReservation
	updatedProject.Spec.ContainerDefaultResourceLimit.LimitsMemory = memoryLimit
	updatedProject.Spec.ContainerDefaultResourceLimit.RequestsMemory = memoryReservation

	updatedProject, err := projects.UpdateProject(client, existingProject, updatedProject)
	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}

func checkContainerResources(client *rancher.Client, clusterID, namespaceName, deploymentName, cpuLimit, cpuReservation, memoryLimit, memoryReservation string) error {
	var errs []string

	podNames, err := pod.GetPodNamesFromDeployment(client, clusterID, namespaceName, deploymentName)
	if err != nil {
		return fmt.Errorf("error fetching pod by deployment name: %w", err)
	}
	if len(podNames) < 1 {
		return errors.New("expected at least one pod, but got " + strconv.Itoa(len(podNames)))
	}

	pod, err := pod.GetPodByName(client, clusterID, namespaceName, podNames[0])
	if err != nil {
		return err
	}
	if len(pod.Spec.Containers) == 0 {
		return errors.New("no containers found in the pod")
	}

	normalizeString := func(s string) string {
		if s == "" {
			return "0"
		}
		return s
	}

	cpuLimit = normalizeString(cpuLimit)
	cpuReservation = normalizeString(cpuReservation)
	memoryLimit = normalizeString(memoryLimit)
	memoryReservation = normalizeString(memoryReservation)

	containerResources := pod.Spec.Containers[0].Resources
	containerCPULimit := containerResources.Limits[corev1.ResourceCPU]
	containerCPURequest := containerResources.Requests[corev1.ResourceCPU]
	containerMemoryLimit := containerResources.Limits[corev1.ResourceMemory]
	containerMemoryRequest := containerResources.Requests[corev1.ResourceMemory]

	if cpuLimit != containerCPULimit.String() {
		errs = append(errs, "CPU limit mismatch")
	}
	if cpuReservation != containerCPURequest.String() {
		errs = append(errs, "CPU reservation mismatch")
	}
	if memoryLimit != containerMemoryLimit.String() {
		errs = append(errs, "Memory limit mismatch")
	}
	if memoryReservation != containerMemoryRequest.String() {
		errs = append(errs, "Memory reservation mismatch")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func checkLimitRange(client *rancher.Client, clusterID, namespaceName string, expectedCPULimit, expectedCPURequest, expectedMemoryLimit, expectedMemoryRequest string) error {
	var ctx *wrangler.Context
	var err error

	if clusterID != "local" {
		ctx, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get downstream context: %w", err)
		}
	} else {
		ctx = client.WranglerContext
	}

	limitRanges, err := ctx.Core.LimitRange().List(namespaceName, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(limitRanges.Items) != 1 {
		return fmt.Errorf("expected limit range count is 1, but got %d", len(limitRanges.Items))
	}
	limitRangeList := limitRanges.Items[0].Spec

	actualCPULimit, ok := limitRangeList.Limits[0].Default["cpu"]
	if !ok {
		return fmt.Errorf("cpu limit not found in the limit range")
	}
	cpuLimit := actualCPULimit.String()
	if cpuLimit != expectedCPULimit {
		return fmt.Errorf("cpu limit in the limit range: %s does not match the expected value: %s", cpuLimit, expectedCPULimit)
	}

	actualMemoryLimit, ok := limitRangeList.Limits[0].Default["memory"]
	if !ok {
		return fmt.Errorf("memory limit not found in the limit range")
	}
	memoryLimit := actualMemoryLimit.String()
	if memoryLimit != expectedMemoryLimit {
		return fmt.Errorf("memory limit in the limit range: %s does not match the expected value: %s", memoryLimit, expectedMemoryLimit)
	}

	actualCPURequest, ok := limitRangeList.Limits[0].DefaultRequest["cpu"]
	if !ok {
		return fmt.Errorf("cpu request not found in the limit range")
	}
	cpuRequest := actualCPURequest.String()
	if cpuRequest != expectedCPURequest {
		return fmt.Errorf("cpu request in the limit range: %s does not match the expected value: %s", cpuRequest, expectedCPURequest)
	}

	actualMemoryRequest, ok := limitRangeList.Limits[0].DefaultRequest["memory"]
	if !ok {
		return fmt.Errorf("memory request not found in the limit range")
	}
	memoryRequest := actualMemoryRequest.String()
	if memoryRequest != expectedMemoryRequest {
		return fmt.Errorf("memory request in the limit range: %s does not match the expected value: %s", memoryRequest, expectedMemoryRequest)
	}

	return nil
}
