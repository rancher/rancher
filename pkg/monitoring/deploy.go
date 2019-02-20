package monitoring

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	"github.com/rancher/rancher/pkg/project"
	corev1 "github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnsureAppProjectName(agentNamespacesClient corev1.NamespaceInterface, ownedProjectID, clusterName, appTargetNamespace string) (string, error) {
	// detect Namespace
	deployNamespace, err := agentNamespacesClient.Get(appTargetNamespace, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return "", errors.Wrapf(err, "failed to find %q Namespace", appTargetNamespace)
	}
	deployNamespace = deployNamespace.DeepCopy()

	if deployNamespace.Name == appTargetNamespace {
		if deployNamespace.DeletionTimestamp != nil {
			return "", fmt.Errorf("stale %q Namespace is still on terminating", appTargetNamespace)
		}
	} else {
		deployNamespace = &k8scorev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: appTargetNamespace,
			},
		}

		if deployNamespace, err = agentNamespacesClient.Create(deployNamespace); err != nil && !k8serrors.IsAlreadyExists(err) {
			return "", errors.Wrapf(err, "failed to create %q Namespace", appTargetNamespace)
		}
	}

	// move Namespace into a project
	expectedAppProjectName := fmt.Sprintf("%s:%s", clusterName, ownedProjectID)
	appProjectName := ""
	if projectName, ok := deployNamespace.Annotations[nslabels.ProjectIDFieldLabel]; ok {
		appProjectName = projectName
	}
	if appProjectName != expectedAppProjectName {
		appProjectName = expectedAppProjectName
		if deployNamespace.Annotations == nil {
			deployNamespace.Annotations = make(map[string]string, 2)
		}

		deployNamespace.Annotations[nslabels.ProjectIDFieldLabel] = appProjectName

		_, err := agentNamespacesClient.Update(deployNamespace)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move Namespace %s into a Project", appTargetNamespace)
		}
	}

	return appProjectName, nil
}

func DetectAppCatalogExistence(appCatalogID string, cattleTemplateVersionsClient mgmtv3.CatalogTemplateVersionInterface) error {
	templateVersionID, templateVersionNamespace, err := common.ParseExternalID(appCatalogID)
	if err != nil {
		return errors.Wrapf(err, "failed to parse catalog ID %q", appCatalogID)
	}

	_, err = cattleTemplateVersionsClient.GetNamespaced(templateVersionNamespace, templateVersionID, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to find catalog by ID %q", appCatalogID)
	}

	return nil
}

func GetSystemProjectID(cattleProjectsClient mgmtv3.ProjectInterface) (string, error) {
	// fetch all system Projects
	cattletSystemProjects, _ := cattleProjectsClient.List(metav1.ListOptions{
		LabelSelector: "authz.management.cattle.io/system-project=true",
	})

	var systemProject *mgmtv3.Project
	cattletSystemProjects = cattletSystemProjects.DeepCopy()
	for _, defaultProject := range cattletSystemProjects.Items {
		systemProject = &defaultProject

		if defaultProject.Spec.DisplayName == project.System {
			break
		}
	}
	if systemProject == nil {
		return "", fmt.Errorf("failed to find any cattle system project")
	}

	return systemProject.Name, nil
}

func DeployApp(cattleAppClient projectv3.AppInterface, projectID string, createOrUpdateApp *projectv3.App) error {
	if createOrUpdateApp == nil {
		return errors.New("cannot deploy a nil App")
	}

	appName := createOrUpdateApp.Name
	app, err := cattleAppClient.GetNamespaced(projectID, appName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query %q App in %s Project", appName, projectID)
	}

	if app.DeletionTimestamp != nil {
		return fmt.Errorf("stale %q App in %s Project is still on terminating", appName, projectID)
	}

	if app.Name == "" {
		if _, err = cattleAppClient.Create(createOrUpdateApp); err != nil {
			return errors.Wrapf(err, "failed to create %q App", appName)
		}
	} else {
		app = app.DeepCopy()
		app.Spec.Answers = createOrUpdateApp.Spec.Answers
		if _, err = cattleAppClient.Update(app); err != nil {
			return errors.Wrapf(err, "failed to update %q App", appName)
		}
	}

	return nil
}

func WithdrawApp(cattleAppClient projectv3.AppInterface, appLabels metav1.ListOptions) error {
	monitoringApps, err := cattleAppClient.List(appLabels)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrapf(err, "failed to find App with %s", appLabels.String())
	}

	for _, app := range monitoringApps.Items {
		if app.DeletionTimestamp == nil {
			if err := cattleAppClient.DeleteNamespaced(app.Namespace, app.Name, &metav1.DeleteOptions{}); err != nil {
				return errors.Wrapf(err, "failed to remove App with %s", appLabels.String())
			}
		}
	}

	return nil
}
