package projects

import (
	"errors"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/kubeapi/namespaces"
	"github.com/rancher/shepherd/extensions/kubeapi/projects"
	rbacapi "github.com/rancher/shepherd/extensions/kubeapi/rbac"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dummyFinalizer          = "example.com/dummy"
	systemProjectLabel      = "authz.management.cattle.io/system-project"
	resourceQuotaAnnotation = "field.cattle.io/resourceQuota"
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

func checkAnnotationExistsInNamespace(client *rancher.Client, clusterID string, namespaceName string, annotationKey string, expectedExistence bool) error {
	updatedNamespace, err := namespaces.GetNamespaceByName(client, clusterID, namespaceName)
	if err != nil {
		return err
	}

	_, exists := updatedNamespace.Annotations[annotationKey]
	if (expectedExistence && !exists) || (!expectedExistence && exists) {
		errorMessage := fmt.Sprintf("Annotation '%s' should%s exist", annotationKey, map[bool]string{true: "", false: " not"}[expectedExistence])
		return errors.New(errorMessage)
	}

	return nil
}

func checkNamespaceLabelsAndAnnotations(clusterID string, projectName string, namespace *corev1.Namespace) error {
	var errorMessages []string
	expectedLabels := map[string]string{
		projects.ProjectIDAnnotation: projectName,
	}

	expectedAnnotations := map[string]string{
		projects.ProjectIDAnnotation: clusterID + ":" + projectName,
	}

	for key, value := range expectedLabels {
		if _, ok := namespace.Labels[key]; !ok {
			errorMessages = append(errorMessages, fmt.Sprintf("expected label %s not present in namespace labels", key))
		} else if namespace.Labels[key] != value {
			errorMessages = append(errorMessages, fmt.Sprintf("label value mismatch for %s: expected %s, got %s", key, value, namespace.Labels[key]))
		}
	}

	for key, value := range expectedAnnotations {
		if _, ok := namespace.Annotations[key]; !ok {
			errorMessages = append(errorMessages, fmt.Sprintf("expected annotation %s not present in namespace annotations", key))
		} else if namespace.Annotations[key] != value {
			errorMessages = append(errorMessages, fmt.Sprintf("annotation value mismatch for %s: expected %s, got %s", key, value, namespace.Annotations[key]))
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf(strings.Join(errorMessages, "\n"))
	}

	return nil
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
