package projects

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/charts"
	"github.com/rancher/rancher/tests/framework/extensions/constants"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	projectsApi "github.com/rancher/rancher/tests/framework/extensions/kubeapi/projects"
	rbacApi "github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	secretsApi "github.com/rancher/rancher/tests/framework/extensions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	dummyFinalizer = "controller.cattle.io/dummy"
	timeFormat     = "2006/01/02 15:04:05"
)

var project = v3.Project{
	ObjectMeta: metav1.ObjectMeta{
		Name:       "",
		Namespace:  "",
		Finalizers: []string{},
	},
	Spec: v3.ProjectSpec{
		ClusterName: "",
	},
}

var prtb = v3.ProjectRoleTemplateBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "",
		Namespace: "",
	},
	ProjectName:       "",
	RoleTemplateName:  "",
	UserPrincipalName: "",
}

var projectscopedsecret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:        "",
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	},
	Data: map[string][]byte{
		"hello": []byte("world"),
	},
}

func createProject(client *rancher.Client, clusterName string) (*v3.Project, error) {
	project.Name = namegen.AppendRandomString("testproject")
	project.Namespace = clusterName
	project.Spec.ClusterName = clusterName
	createdProject, err := projectsApi.CreateProject(client, &project)
	if err != nil {
		return nil, err
	}

	return createdProject, nil
}

func createProjectRoleTemplateBinding(client *rancher.Client, user *management.User, project *v3.Project, role string) (*v3.ProjectRoleTemplateBinding, error) {
	projectName := fmt.Sprintf("%s:%s", project.Namespace, project.Name)
	prtb.Name = namegen.AppendRandomString("prtb-")
	prtb.Namespace = project.Name
	prtb.ProjectName = projectName
	prtb.RoleTemplateName = role
	prtb.UserPrincipalName = user.PrincipalIDs[0]
	createdProjectRoleTemplateBinding, err := rbacApi.CreateProjectRoleTemplateBinding(client, &prtb)
	if err != nil {
		return nil, err
	}

	return createdProjectRoleTemplateBinding, nil
}

func waitForFinalizerToUpdate(client *rancher.Client, projectName string) error {
	err := kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		project, pollErr := projectsApi.ListProjects(client, project.Namespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + projectName,
		})
		if pollErr != nil {
			return false, pollErr
		}

		if len(project.Items[0].Finalizers) >= 2 {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return err
	}

	return nil
}

func checkPodLogsForErrors(client *rancher.Client, cluster string, podName string, namespace string, errorPattern string, startTime time.Time) error {
	startTimeUTC := startTime.UTC()

	errorRegex := regexp.MustCompile(errorPattern)
	timeRegex := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	var errorMessage string

	kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TwoMinuteTimeout, func() (bool, error) {
		podLogs, err := kubeconfig.GetPodLogs(client, cluster, podName, namespace)
		if err != nil {
			return false, err
		}

		segments := strings.Split(podLogs, "\n")
		for _, segment := range segments {
			timeMatches := timeRegex.FindStringSubmatch(segment)
			if len(timeMatches) > 0 {
				segmentTime, err := time.Parse(timeFormat, timeMatches[0])
				if err != nil {
					continue
				}

				segmentTimeUTC := segmentTime.UTC()
				if segmentTimeUTC.After(startTimeUTC) {
					if matches := errorRegex.FindStringSubmatch(segment); len(matches) > 0 {
						errorMessage = "error logs found in rancher: " + segment
						return true, nil
					}
				}
			}
		}
		return false, nil
	})

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}

func updateProjectNamespaceFinalizer(client *rancher.Client, Project *v3.Project, finalizer []string) (*v3.Project, error) {
	Project.ObjectMeta.Finalizers = finalizer

	updatedProject, err := projectsApi.UpdateProject(client, Project)
	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}

func createProjectScopedSecret(client *rancher.Client, project *v3.Project) (*corev1.Secret, error) {
	projectscopedsecret.Name = namegen.AppendRandomString("testprojscopedsecret")
	annotationValue := project.Namespace + ":" + project.Name
	projectscopedsecret.Annotations[constants.ProjectIDAnnotation] = annotationValue
	projectscopedsecret.Labels[constants.ProjectScopedLabel] = constants.ProjectScopedLabelValue

	createdProjectScopedSecret, err := secretsApi.CreateSecretForCluster(client, projectscopedsecret, constants.LocalCluster, project.Name)

	if err != nil {
		return nil, err
	}

	return createdProjectScopedSecret, nil
}

func createNamespaces(client *rancher.Client, namespaceCount int, project *v3.Project) ([]*corev1.Namespace, error) {
	projectObj, err := projects.GetProjectByName(client, project.Namespace, project.Name)
	if err != nil {
		return nil, err
	}

	var namespaceList []*corev1.Namespace

	for i := 0; i < namespaceCount; i++ {
		namespaceName := namegen.AppendRandomString("testns")
		namespace, err := namespaces.CreateNamespace(client, namespaceName, "{}", map[string]string{}, map[string]string{}, projectObj)

		if err != nil {
			return nil, err
		}

		namespaceObj := &corev1.Namespace{}
		err = v1.ConvertToK8sType(namespace.JSONResp, namespaceObj)
		if err != nil {
			return nil, err
		}

		namespaceList = append(namespaceList, namespaceObj)
	}

	return namespaceList, nil
}

func validateProjectSecretLabelsAndAnnotations(projectScopedSecret *corev1.Secret, annotationValue string) error {

	expectedLabel := constants.ProjectScopedLabel + ": " + constants.ProjectScopedLabelValue
	actualLabel, labelExists := projectScopedSecret.Labels[constants.ProjectScopedLabel]
	if !(labelExists && actualLabel == expectedLabel) {
		return fmt.Errorf("project scoped secret does not have label '%s'", expectedLabel)
	}

	expectedAnnotation := constants.ProjectIDAnnotation + ": " + annotationValue
	actualAnnotation, annotationExists := projectScopedSecret.Annotations[constants.ProjectIDAnnotation]
	if !(annotationExists && actualAnnotation == expectedAnnotation) {
		return fmt.Errorf("project scoped secret does not have annotation '%s'", expectedAnnotation)
	}

	return nil
}

func validatePropagatedNamespaceSecret(client *rancher.Client, projectScopedSecret *corev1.Secret, namespaceList []*corev1.Namespace) error {
	for _, namespace := range namespaceList {
		annotationValue := projectScopedSecret.Annotations[constants.ProjectIDAnnotation]
		clusterID := strings.Split(annotationValue, ":")[0]
		secretName := projectScopedSecret.Name

		namespaceSecret, err := secretsApi.GetSecretByName(client, clusterID, namespace.Name, secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		hasProjectScopedLabel := namespaceSecret.Labels[constants.ProjectScopedLabel] == constants.ProjectScopedLabelValue
		hasProjectIDAnnotation := namespaceSecret.Annotations[constants.ProjectIDAnnotation] == projectScopedSecret.Namespace+":"+projectScopedSecret.Name
		_, hasCreatorLabel := namespaceSecret.Labels[constants.NormanCreatorLabel]

		if !hasProjectScopedLabel || !hasProjectIDAnnotation || hasCreatorLabel {
			errorMessage := "Validation failed for namespace: " + namespace.Name + "\n"
			if !hasProjectScopedLabel {
				errorMessage += "Missing or incorrect 'cattle.io/project-scoped' label\n"
			}
			if !hasProjectIDAnnotation {
				errorMessage += "Missing or incorrect 'field.cattle.io/projectId' annotation\n"
			}
			if hasCreatorLabel {
				errorMessage += "'cattle.io/creator' label should not exist\n"
			}
			return errors.New(errorMessage)
		}

		if !reflect.DeepEqual(projectScopedSecret.Data, namespaceSecret.Data) {
			return fmt.Errorf("secret data mismatch for secret '%s' in namespace '%s'", secretName, namespace.Name)
		}
	}

	return nil
}

func CreateDeploymentWithEnvSecret(client *rancher.Client, clusterID string, namespace *corev1.Namespace, secret *corev1.Secret) error {
	deploymentName := namegen.AppendRandomString("testdeployment")
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	podTemplateWithSecretEnvironmentVariable := workloads.NewPodTemplateWithSecretEnvironmentVariable(secret.Name)
	deploymentEnvironmentWithSecretTemplate := workloads.NewDeploymentTemplate(deploymentName, namespace.Name, podTemplateWithSecretEnvironmentVariable, true, nil)
	_, err = steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentEnvironmentWithSecretTemplate)
	if err != nil {
		return err
	}

	err = charts.WatchAndWaitDeployments(client, clusterID, namespace.Name, metav1.ListOptions{})
	if err != nil {
		return err
	}

	return nil
}
