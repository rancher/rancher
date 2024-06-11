package auth

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectCreateController = "mgmt-project-rbac-create"
	projectRemoveController = "mgmt-project-rbac-remove"
)

type projectLifecycle struct {
	mgr                  *mgr
	crtbClient           v3.ClusterRoleTemplateBindingInterface
	crtbLister           v3.ClusterRoleTemplateBindingLister
	prtbLister           v3.ProjectRoleTemplateBindingLister
	rbLister             rbacv1.RoleBindingLister
	roleBindings         rbacv1.RoleBindingInterface
	systemAccountManager *systemaccount.Manager
}

func (l *projectLifecycle) sync(key string, orig *apisv3.Project) (runtime.Object, error) {
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

	obj, err := l.mgr.reconcileResourceToNamespace(obj, projectCreateController)
	if err != nil {
		return nil, err
	}

	obj, err = l.reconcileProjectCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating project %v", projectCreateController, orig.Name)
		obj, err = l.mgr.mgmt.Management.Projects("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}
	if err != nil && !kerrors.IsAlreadyExists(err) {
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

func (l *projectLifecycle) Create(obj *apisv3.Project) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *projectLifecycle) Updated(obj *apisv3.Project) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *projectLifecycle) Remove(obj *apisv3.Project) (runtime.Object, error) {
	var returnErr error
	set := labels.Set{rbac.RestrictedAdminProjectRoleBinding: "true"}
	rbs, err := l.rbLister.List(obj.Name, labels.SelectorFromSet(set))
	returnErr = errors.Join(returnErr, err)

	for _, rb := range rbs {
		err := l.roleBindings.DeleteNamespaced(obj.Name, rb.Name, &v1.DeleteOptions{})
		returnErr = errors.Join(returnErr, err)
	}
	err = l.mgr.deleteNamespace(obj, projectRemoveController)
	returnErr = errors.Join(returnErr, err)
	return obj, returnErr
}

func (l *projectLifecycle) reconcileProjectCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok || creatorID == "" {
			logrus.Warnf("%v %v has no creatorId annotation. Cannot add creator as owner", typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}
		project := obj.(*apisv3.Project)

		if apisv3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
			// The projectRoleBindings are already completed, no need to check
			return obj, nil
		}

		// If the project does not have the annotation it indicates the
		// project is from a previous rancher version so don't add the
		// default bindings.
		roleJSON, ok := project.Annotations[roleTemplatesRequired]
		if !ok {
			return project, nil
		}

		roleMap := make(map[string][]string)
		err = json.Unmarshal([]byte(roleJSON), &roleMap)
		if err != nil {
			return obj, err
		}

		var createdRoles []string

		for _, role := range roleMap["required"] {
			rtbName := "creator-" + role

			if rtb, _ := l.prtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
				createdRoles = append(createdRoles, role)
				// This projectRoleBinding exists, need to check all of them so keep going
				continue
			}

			// The projectRoleBinding doesn't exist yet so create it
			om := v1.ObjectMeta{
				Name:      rtbName,
				Namespace: metaAccessor.GetName(),
			}

			logrus.Infof("[%v] Creating creator projectRoleTemplateBinding for user %v for project %v", projectCreateController, creatorID, metaAccessor.GetName())
			if _, err := l.mgr.mgmt.Management.ProjectRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ProjectRoleTemplateBinding{
				ObjectMeta:       om,
				ProjectName:      metaAccessor.GetNamespace() + ":" + metaAccessor.GetName(),
				RoleTemplateName: role,
				UserName:         creatorID,
			}); err != nil && !apierrors.IsAlreadyExists(err) {
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

		project.Annotations[roleTemplatesRequired] = string(d)

		if reflect.DeepEqual(roleMap["required"], createdRoles) {
			apisv3.ProjectConditionInitialRolesPopulated.True(project)
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on project %v", ctrbMGMTController, project.Name)
		}
		if _, err := l.mgr.mgmt.Management.Projects("").Update(project); err != nil {
			return obj, err
		}
		return obj, nil
	})
}
