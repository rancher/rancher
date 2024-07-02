package projects

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeapi/namespaces"
	"github.com/rancher/shepherd/extensions/kubeapi/projects"
	"github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/rancher/shepherd/extensions/kubeapi/workloads/deployments"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	dummyFinalizer                  = "dummy"
	timeFormat                      = "2006/01/02 15:04:05"
	projectOwner                    = "project-owner"
	clusterOwner                    = "cluster-owner"
	clusterMember                   = "cluster-member"
	systemProjectName               = "System"
	systemProjectLabel              = "authz.management.cattle.io/system-project"
	namespaceSteveType              = "namespace"
	resourceQuotaAnnotation         = "field.cattle.io/resourceQuota"
	containerDefaultLimitAnnotation = "field.cattle.io/containerDefaultResourceLimit"
	resourceQuotaStatusAnnotation   = "cattle.io/status"
	containerName                   = "nginx"
	imageName                       = "nginx"
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

func NewProjectTemplate(clusterID string) *v3.Project {
	project := &v3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:       namegen.AppendRandomString("testproject"),
			Namespace:  clusterID,
			Finalizers: []string{},
		},
		Spec: v3.ProjectSpec{
			ClusterName: clusterID,
			ResourceQuota: &v3.ProjectResourceQuota{
				Limit: v3.ResourceQuotaLimit{
					Pods: "",
				},
			},
			NamespaceDefaultResourceQuota: &v3.NamespaceResourceQuota{
				Limit: v3.ResourceQuotaLimit{
					Pods: "",
				},
			},
			ContainerDefaultResourceLimit: &v3.ContainerResourceLimit{
				RequestsCPU:    "",
				RequestsMemory: "",
				LimitsCPU:      "",
				LimitsMemory:   "",
			},
		},
	}
	return project
}

func createProject(client *rancher.Client, project *v3.Project) (*v3.Project, error) {
	createdProject, err := client.WranglerContext.Mgmt.Project().Create(project)
	if err != nil {
		return nil, err
	}

	return createdProject, nil
}

func createProjectAndNamespace(client *rancher.Client, clusterID string, project *v3.Project) (*v3.Project, *corev1.Namespace, error) {
	createdProject, err := createProject(client, project)
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

func createProjectRoleTemplateBinding(client *rancher.Client, user *management.User, project *v3.Project, role string) (*v3.ProjectRoleTemplateBinding, error) {
	projectName := fmt.Sprintf("%s:%s", project.Namespace, project.Name)
	prtb.Name = namegen.AppendRandomString("prtb-")
	prtb.Namespace = project.Name
	prtb.ProjectName = projectName
	prtb.RoleTemplateName = role
	prtb.UserPrincipalName = user.PrincipalIDs[0]

	createdProjectRoleTemplateBinding, err := rbac.CreateProjectRoleTemplateBinding(client, &prtb)
	if err != nil {
		return nil, err
	}

	return createdProjectRoleTemplateBinding, nil
}

func createDeployment(client *rancher.Client, clusterID string, namespace string, replicaCount int) (*appv1.Deployment, error) {
	deploymentName := namegen.AppendRandomString("testdeployment")
	containerTemplate := workloads.NewContainer(containerName, imageName, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
	replicas := int32(replicaCount)

	deploymentObj, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespace, podTemplate, replicas)
	if err != nil {
		return nil, err
	}

	return deploymentObj, nil
}

func updateProjectNamespaceFinalizer(client *rancher.Client, existingProject *v3.Project, finalizer []string) (*v3.Project, error) {
	updatedProject := existingProject.DeepCopy()
	updatedProject.ObjectMeta.Finalizers = finalizer

	updatedProject, err := projects.UpdateProject(client, existingProject, updatedProject)
	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}

func waitForFinalizerToUpdate(client *rancher.Client, projectName string, projectNamespace string, finalizerCount int) error {
	err := kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		project, pollErr := projects.ListProjects(client, projectNamespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + projectName,
		})
		if pollErr != nil {
			return false, pollErr
		}

		if len(project.Items[0].Finalizers) == finalizerCount {
			return true, nil
		}
		return false, pollErr
	})

	if err != nil {
		return err
	}

	return nil
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

func checkPodLogsForErrors(client *rancher.Client, clusterID string, podName string, namespace string, errorPattern string, startTime time.Time) error {
	startTimeUTC := startTime.UTC()

	errorRegex := regexp.MustCompile(errorPattern)
	timeRegex := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	var errorMessage string

	kwait.Poll(defaults.TenSecondTimeout, defaults.TwoMinuteTimeout, func() (bool, error) {
		podLogs, err := kubeconfig.GetPodLogs(client, clusterID, podName, namespace, "")
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
