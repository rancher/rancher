package auth

import (
	"errors"
	"reflect"
	"strings"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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
	systemAccountManager *systemaccount.Manager
	rbLister             rbacv1.RoleBindingLister
	roleBindings         rbacv1.RoleBindingInterface
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

	obj, err = l.mgr.reconcileCreatorRTB(obj)
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
	crtbs, err := l.mgr.crtbLister.List(clusterID, labels.Everything())
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
