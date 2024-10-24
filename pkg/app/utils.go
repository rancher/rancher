package app

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/managementagent/nslabels"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubectl/pkg/util/slice"
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

	expectedAppProjectName := fmt.Sprintf("%s:%s", clusterName, ownedProjectID)
	appProjectID := deployNamespace.Annotations[nslabels.ProjectIDFieldLabel]
	if appProjectID != "" && isKnownSystemNamespace(deployNamespace) {
		// Don't reassign the system namespaces to another project.
		return expectedAppProjectName, nil
	}

	// move Namespace into a project
	if appProjectID != expectedAppProjectName {
		appProjectID = expectedAppProjectName
		if deployNamespace.Annotations == nil {
			deployNamespace.Annotations = make(map[string]string, 2)
		}

		deployNamespace.Annotations[nslabels.ProjectIDFieldLabel] = appProjectID

		_, err := userNSClient.Update(deployNamespace)
		if err != nil {
			return "", errors.Wrapf(err, "failed to move Namespace %s into a Project", appTargetNamespace)
		}
	}

	return appProjectID, nil
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

func isKnownSystemNamespace(ns *v1.Namespace) bool {
	return slice.ContainsString(strings.Split(settings.SystemNamespaces.Get(), ","), ns.Name, nil) ||
		rbac.IsFleetNamespace(ns)
}
