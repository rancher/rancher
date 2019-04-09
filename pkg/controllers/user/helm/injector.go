package helm

import (
	pkgerrors "github.com/pkg/errors"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	saInjectionAnno         = "field.cattle.io/injectAccount"
	ServiceAccountAnswerKey = "global.rancherInjectedServiceAccountName"
	rtbOwnerLabel           = "authz.cluster.cattle.io/rtb-owner"
	nsByProjectIndex        = "authz.cluster.cattle.io/ns-by-project"
)

func (l *Lifecycle) InjectServiceAccount(app *v3.App) map[string]string {
	if getRoleTemplateName(app) == "" {
		return nil
	}
	return map[string]string{ServiceAccountAnswerKey: app.Name}
}

func (l *Lifecycle) EnsureAppServiceAccount(app *v3.App) error {
	if getRoleTemplateName(app) == "" {
		return nil
	}
	saName := app.Name
	ns := app.Spec.TargetNamespace
	_, err := l.getOrCreateServiceAccount(ns, saName, string(app.UID))
	if err != nil {
		return err
	}

	role, err := l.rbacManager.RoleTemplateLister().Get("", getRoleTemplateName(app))
	if err != nil {
		return err
	}

	namespaces, err := l.rbacManager.NamespaceIndexer().ByIndex("authz.cluster.cattle.io/ns-by-project", l.ClusterName+":"+app.Namespace)
	if err != nil {
		return pkgerrors.Wrapf(err, "couldn't list namespaces with project ID %v", l.ClusterName+":"+app.Namespace)
	}

	roles := map[string]*mgmtv3.RoleTemplate{}
	if err := l.rbacManager.GatherRoles(role, roles); err != nil {
		return err
	}

	if err := l.rbacManager.EnsureRoles(roles); err != nil {
		return err
	}

	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		if err := l.ensureServiceAccountBindings(ns.Name, roles, app); err != nil {
			return err
		}
	}

	return l.rbacManager.ReconcileProjectAccessToGlobalResources(app, roles)
}

func (l *Lifecycle) ensureServiceAccountBindings(ns string, roles map[string]*mgmtv3.RoleTemplate, app *v3.App) error {
	create := func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object {
		return &rbacv1.RoleBinding{
			ObjectMeta: objectMeta,
			Subjects:   subjects,
			RoleRef:    roleRef,
		}
	}

	list := func(ns string, selector labels.Selector) ([]interface{}, error) {
		currentRBs, err := l.rbacManager.RoleBindingLister().List(ns, selector)
		if err != nil {
			return nil, err
		}
		var items []interface{}
		for _, c := range currentRBs {
			items = append(items, c)
		}
		return items, nil
	}

	convert := func(i interface{}) (string, string, []rbacv1.Subject) {
		rb, _ := i.(*rbacv1.RoleBinding)
		return rb.Name, rb.RoleRef.Name, rb.Subjects
	}
	return l.rbacManager.EnsureBindings(ns, roles, app, l.rbacManager.UserContext().RBAC.RoleBindings(ns).ObjectClient(), create, list, convert)
}

func (l *Lifecycle) getOrCreateServiceAccount(ns, saName, creatorUUID string) (*v1.ServiceAccount, error) {
	sa, err := l.ServiceAccountLister.Get(ns, saName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if errors.IsNotFound(err) {
		return l.ServiceAccountClient.Create(&v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saName,
				Namespace: ns,
				Labels: map[string]string{
					creatorUUID: "created-by-app",
				},
			},
		})
	}
	return sa, nil
}

func getRoleTemplateName(app *v3.App) string {
	value, ok := app.Annotations[saInjectionAnno]
	if !ok {
		return ""
	}

	if value != "project-monitoring-view" {
		return ""
	}

	return value
}

func (l *Lifecycle) ensureServiceAccountDeleted(app *v3.App) error {
	namespaces, err := l.rbacManager.NamespaceIndexer().ByIndex(nsByProjectIndex, l.ClusterName+":"+app.Namespace)
	if err != nil {
		return pkgerrors.Wrapf(err, "couldn't list namespaces with project ID %v", app.Namespace)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: string(app.UID)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := l.rbacManager.UserContext().RBAC.RoleBindings(ns.Name)
		rbs, err := l.rbacManager.RoleBindingLister().List(ns.Name, set.AsSelector())
		if err != nil {
			return pkgerrors.Wrapf(err, "couldn't list rolebindings with selector %s", set.AsSelector())
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if !errors.IsNotFound(err) {
					return pkgerrors.Wrapf(err, "error deleting rolebinding %v", rb.Name)
				}
			}
		}
	}

	if err := l.rbacManager.ReconcileProjectAccessToGlobalResourcesForDelete(app); err != nil {
		return err
	}

	sa, err := l.ServiceAccountLister.Get(app.Spec.TargetNamespace, app.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		return nil
	}

	return l.ServiceAccountClient.DeleteNamespaced(sa.Namespace, sa.Name, &metav1.DeleteOptions{})
}
