package utils

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/controllers/user/nslabels"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	creatorIDAnno = "field.cattle.io/creatorId"
)

func EnsureAppProjectName(userNSClient v1.NamespaceInterface, ownedProjectID, clusterName, appTargetNamespace, creatorID string) (string, error) {
	// detect Namespace
	deployNamespace, err := userNSClient.Get(appTargetNamespace, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return "", errors.Wrapf(err, "failed to find %q Namespace", appTargetNamespace)
	}
	deployNamespace = deployNamespace.DeepCopy()

	if deployNamespace.Name == appTargetNamespace {
		if deployNamespace.DeletionTimestamp != nil {
			return "", fmt.Errorf("stale %q Namespace is still on terminating", appTargetNamespace)
		}
	} else {
		deployNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        appTargetNamespace,
				Annotations: map[string]string{creatorIDAnno: creatorID},
			},
		}

		if deployNamespace, err = userNSClient.Create(deployNamespace); err != nil && !kerrors.IsAlreadyExists(err) {
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

		_, err := userNSClient.Update(deployNamespace)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move Namespace %s into a Project", appTargetNamespace)
		}
	}

	return appProjectName, nil
}

func GetSystemProjectID(clusterName string, cattleProjectsLister v3.ProjectLister) (string, error) {
	// fetch all system Projects
	cattleSystemProjects, _ := cattleProjectsLister.List(clusterName, labels.Set(project.SystemProjectLabel).AsSelector())

	var systemProject *v3.Project
	for _, p := range cattleSystemProjects {
		if p.Spec.DisplayName == project.System {
			systemProject = p
			break
		}
	}
	if systemProject == nil {
		return "", fmt.Errorf("failed to find any cattle system project")
	}

	return systemProject.Name, nil
}

func DeployApp(mgmtAppClient projv3.AppInterface, projectID string, createOrUpdateApp *projv3.App, forceRedeploy bool) (*projv3.App, error) {
	if createOrUpdateApp == nil {
		return nil, errors.New("cannot deploy a nil App")
	}
	var rtn *projv3.App

	appName := createOrUpdateApp.Name
	app, err := mgmtAppClient.GetNamespaced(projectID, appName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "failed to query %q App in %s Project", appName, projectID)
	}

	if app.DeletionTimestamp != nil {
		return nil, fmt.Errorf("stale %q App in %s Project is still on terminating", appName, projectID)
	}

	if app.Name == "" {
		logrus.Infof("Create app %s/%s", app.Spec.TargetNamespace, app.Name)
		if rtn, err = mgmtAppClient.Create(createOrUpdateApp); err != nil {
			return nil, errors.Wrapf(err, "failed to create %q App", appName)
		}
	} else {
		app = app.DeepCopy()
		app.Spec.Answers = createOrUpdateApp.Spec.Answers

		// clean up status
		if forceRedeploy {
			if app.Spec.Answers == nil {
				app.Spec.Answers = make(map[string]string, 1)
			}
			app.Spec.Answers["redeployTs"] = fmt.Sprintf("%d", time.Now().Unix())
		}

		if rtn, err = mgmtAppClient.Update(app); err != nil {
			return nil, errors.Wrapf(err, "failed to update %q App", appName)
		}
	}

	return rtn, nil
}

func DetectAppCatalogExistence(appCatalogID string, cattleTemplateVersionsLister v3.CatalogTemplateVersionLister) error {
	templateVersionID, templateVersionNamespace, err := common.ParseExternalID(appCatalogID)
	if err != nil {
		return errors.Wrapf(err, "failed to parse catalog ID %q", appCatalogID)
	}

	_, err = cattleTemplateVersionsLister.Get(templateVersionNamespace, templateVersionID)
	if err != nil {
		return errors.Wrapf(err, "failed to find catalog by ID %q", appCatalogID)
	}

	return nil
}

func DeleteApp(mgmtAppClient projv3.AppInterface, projectID, appName string) error {
	app, err := mgmtAppClient.GetNamespaced(projectID, appName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to find app %s in project %v", appName, projectID)
	}

	if app.DeletionTimestamp == nil {
		err := mgmtAppClient.DeleteNamespaced(projectID, appName, &metav1.DeleteOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to delete app %v", appName)
		}
	}

	return nil
}
