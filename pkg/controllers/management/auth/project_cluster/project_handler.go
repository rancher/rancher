package project_cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// The name of the project create controller
	ProjectCreateController = "mgmt-project-rbac-create"
	// The name of the project remove controller
	ProjectRemoveController = "mgmt-project-rbac-remove"
)

type projectLifecycle struct {
	mgr                  *config.ManagementContext
	crtbClient           v3.ClusterRoleTemplateBindingInterface
	crtbLister           v3.ClusterRoleTemplateBindingLister
	nsLister             corev1.NamespaceLister
	projects             v3.ProjectInterface
	prtbLister           v3.ProjectRoleTemplateBindingLister
	rbLister             rbacv1.RoleBindingLister
	roleBindings         rbacv1.RoleBindingInterface
	systemAccountManager *systemaccount.Manager
}

// NewProjectLifecycle creates and returns a projectLifecycle from a given ManagementContext
func NewProjectLifecycle(management *config.ManagementContext) *projectLifecycle {
	return &projectLifecycle{
		mgr:                  management,
		crtbClient:           management.Management.ClusterRoleTemplateBindings(""),
		crtbLister:           management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		nsLister:             management.Core.Namespaces("").Controller().Lister(),
		projects:             management.Management.Projects(""),
		prtbLister:           management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		rbLister:             management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:         management.RBAC.RoleBindings(""),
		systemAccountManager: systemaccount.NewManager(management),
	}
}

// Sync gets called whenever a project is created or updated and ensures the project
// has all the necessary backing resources
func (l *projectLifecycle) Sync(key string, orig *apisv3.Project) (runtime.Object, error) {
	if orig == nil || orig.DeletionTimestamp != nil {
		projectID := ""
		splits := strings.Split(key, "/")
		if len(splits) == 2 {
			projectID = splits[1]
		}
		// remove the system account created for this project
		logrus.Debugf("Deleting system user for project %v", projectID)
		if err := l.systemAccountManager.RemoveSystemAccount(projectID); err != nil {
			return nil, err
		}
		return nil, nil
	}

	obj := orig.DeepCopyObject()

	obj, err := reconcileResourceToNamespace(obj, ProjectCreateController, l.nsLister, l.mgr.K8sClient.CoreV1().Namespaces())
	if err != nil {
		return nil, err
	}

	obj, err = l.reconcileProjectCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating project %v", ProjectCreateController, orig.Name)
		obj, err = l.projects.ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}

	if err := l.enqueueCrtbs(orig); err != nil {
		return obj, err
	}

	return obj, nil
}

func (l *projectLifecycle) enqueueCrtbs(project *apisv3.Project) error {
	// get all crtbs in current project's cluster
	clusterID := project.Namespace
	crtbs, err := l.crtbLister.List(clusterID, labels.Everything())
	if err != nil {
		return err
	}
	// enqueue them so crtb controller picks them up and lists all projects and generates rolebindings for each crtb in the projects
	for _, crtb := range crtbs {
		l.crtbClient.Controller().Enqueue(clusterID, crtb.Name)
	}
	return nil
}

// Create is a no-op because the Sync function takes care of resource orchestration
func (l *projectLifecycle) Create(obj *apisv3.Project) (runtime.Object, error) {
	return obj, nil
}

// Updated is a no-op because the Sync function takes care of resource orchestration
func (l *projectLifecycle) Updated(obj *apisv3.Project) (runtime.Object, error) {
	return obj, nil
}

// Remove deletes all backing resources created by the project
func (l *projectLifecycle) Remove(obj *apisv3.Project) (runtime.Object, error) {
	var returnErr error
	set := labels.Set{rbac.RestrictedAdminProjectRoleBinding: "true"}
	rbs, err := l.rbLister.List(obj.Name, labels.SelectorFromSet(set))
	returnErr = errors.Join(returnErr, err)

	for _, rb := range rbs {
		err := l.roleBindings.DeleteNamespaced(obj.Name, rb.Name, &metav1.DeleteOptions{})
		returnErr = errors.Join(returnErr, err)
	}
	err = deleteNamespace(obj, ProjectRemoveController, l.mgr.K8sClient.CoreV1().Namespaces())
	returnErr = errors.Join(returnErr, err)
	return obj, returnErr
}

func (l *projectLifecycle) reconcileProjectCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		project, ok := obj.(*apisv3.Project)
		if !ok {
			return obj, fmt.Errorf("expected project, got %T", obj)
		}

		creatorID := project.Annotations[CreatorIDAnnotation]
		if creatorID == "" {
			logrus.Warnf("[%s] project %s has no creatorId annotation. Cannot add creator as owner", ProjectCreateController, project.Name)
			return obj, nil
		}

		if apisv3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
			// The projectRoleBindings are already completed, no need to check
			return obj, nil
		}

		// If the project does not have the annotation it indicates the
		// project is from a previous rancher version so don't add the
		// default bindings.
		creatorRoleBindings := project.Annotations[roleTemplatesRequiredAnnotation]
		if creatorRoleBindings == "" {
			return project, nil
		}

		roleMap := make(map[string][]string)
		if err := json.Unmarshal([]byte(creatorRoleBindings), &roleMap); err != nil {
			return obj, err
		}

		var createdRoles []string

		for _, role := range roleMap["required"] {
			rtbName := "creator-" + role
			if rtb, _ := l.prtbLister.Get(project.Name, rtbName); rtb != nil {
				createdRoles = append(createdRoles, role)
				// This projectRoleBinding exists, need to check all of them so keep going
				continue
			}

			// The projectRoleBinding doesn't exist yet so create it
			prtb := &apisv3.ProjectRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rtbName,
					Namespace: project.Name,
				},
				ProjectName:      project.Namespace + ":" + project.Name,
				RoleTemplateName: role,
				UserName:         creatorID,
			}

			if principalName := project.Annotations[creatorPrincipleNameAnnotation]; principalName != "" {
				if !strings.HasPrefix(principalName, "local") {
					// Setting UserPrincipalName only makes sense for non-local users.
					prtb.UserPrincipalName = principalName
					prtb.UserName = ""
				}
			}

			logrus.Infof("[%v] Creating creator projectRoleTemplateBinding for user %s for project %s", ProjectCreateController, creatorID, project.Name)
			_, err := l.mgr.Management.ProjectRoleTemplateBindings(project.Name).Create(prtb)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return obj, err
			}
			createdRoles = append(createdRoles, role)
		}

		project = project.DeepCopy()

		roleMap["created"] = createdRoles
		d, err := json.Marshal(roleMap)
		if err != nil {
			return obj, err
		}

		project.Annotations[roleTemplatesRequiredAnnotation] = string(d)

		if reflect.DeepEqual(roleMap["required"], createdRoles) {
			apisv3.ProjectConditionInitialRolesPopulated.True(project)
			logrus.Infof("[%s] Setting InitialRolesPopulated condition on project %s", ProjectCreateController, project.Name)
		}
		if _, err := l.projects.Update(project); err != nil {
			return obj, err
		}
		return obj, nil
	})
}
