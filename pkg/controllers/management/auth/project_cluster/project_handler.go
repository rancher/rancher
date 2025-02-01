package project_cluster

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// The name of the project create controller
	ProjectCreateController = "mgmt-project-rbac-create"
	// The name of the project remove controller
	ProjectRemoveController = "mgmt-project-rbac-remove"
)

type projectLifecycle struct {
	crtbClient           v3.ClusterRoleTemplateBindingController
	crtbLister           v3.ClusterRoleTemplateBindingCache
	nsLister             corev1.NamespaceCache
	nsClient             k8scorev1.NamespaceInterface
	projects             v3.ProjectController
	prtbLister           v3.ProjectRoleTemplateBindingCache
	prtbClient           v3.ProjectRoleTemplateBindingController
	rbLister             rbacv1.RoleBindingCache
	roleBindings         rbacv1.RoleBindingController
	systemAccountManager *systemaccount.Manager
	crClient             rbacv1.ClusterRoleController
}

// NewProjectLifecycle creates and returns a projectLifecycle from a given ManagementContext
func NewProjectLifecycle(management *config.ManagementContext) *projectLifecycle {
	return &projectLifecycle{
		crtbClient:           management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		crtbLister:           management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		nsLister:             management.Wrangler.Core.Namespace().Cache(),
		nsClient:             management.K8sClient.CoreV1().Namespaces(),
		projects:             management.Wrangler.Mgmt.Project(),
		prtbLister:           management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		prtbClient:           management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		rbLister:             management.Wrangler.RBAC.RoleBinding().Cache(),
		roleBindings:         management.Wrangler.RBAC.RoleBinding(),
		systemAccountManager: systemaccount.NewManager(management),
		crClient:             management.Wrangler.RBAC.ClusterRole(),
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
		logrus.Debugf("Deleting system user for project %s", projectID)
		if err := l.systemAccountManager.RemoveSystemAccount(projectID); err != nil {
			return nil, err
		}
		return nil, nil
	}

	obj := orig.DeepCopyObject()

	obj, err := reconcileResourceToNamespace(obj, ProjectCreateController, l.nsLister, l.nsClient)
	if err != nil {
		return nil, err
	}

	if features.AggregatedRoleTemplates.Enabled() {
		if err := createMembershipRoles(obj, l.crClient); err != nil {
			return nil, err
		}
	}

	obj, err = l.reconcileProjectCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%s] Updating project %s", ProjectCreateController, orig.Name)
		project := obj.(*apisv3.Project)
		obj, err = l.projects.Update(project)
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
		l.crtbClient.Enqueue(clusterID, crtb.Name)
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
	return obj, deleteNamespace(obj, ProjectRemoveController, l.nsClient)
}

func (l *projectLifecycle) reconcileProjectCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	project, ok := obj.(*apisv3.Project)
	if !ok {
		return obj, fmt.Errorf("expected project, got %T", obj)
	}

	// If we specify no creator owner RBAC, exit
	if _, ok := project.Annotations[NoCreatorRBACAnnotation]; ok {
		logrus.Infof("[%s] annotation %s found. Skipping adding creator as owner", ProjectCreateController, NoCreatorRBACAnnotation)
		return obj, nil
	}
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
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

			if principalName := project.Annotations[creatorPrincipalNameAnnotation]; principalName != "" {
				if !strings.HasPrefix(principalName, "local") {
					// Setting UserPrincipalName only makes sense for non-local users.
					prtb.UserPrincipalName = principalName
					prtb.UserName = ""
				}
			}

			logrus.Infof("[%s] Creating creator projectRoleTemplateBinding for user %s for project %s", ProjectCreateController, creatorID, project.Name)
			_, err := l.prtbClient.Create(prtb)
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

		_, err = l.projects.Update(project)

		return obj, err
	})
}
