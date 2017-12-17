package authz

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	typesextv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	finalizerName  = "rtbFinalizer"
	rtbOwnerLabel  = "io.cattle.rtb.owner"
	projectIDLabel = "io.cattle.field.projectId"
	prtbIndex      = "io.cattle.authz.prtb.projectname"
)

func Register(workload *config.ClusterContext) {
	// Add cache informer to project role template bindings
	informer := workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		prtbIndex: prtbIndexer,
	}
	informer.AddIndexers(indexers)

	r := &roleHandler{
		workload:   workload,
		indexer:    informer.GetIndexer(),
		rtLister:   workload.Management.Management.RoleTemplates("").Controller().Lister(),
		psptLister: workload.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		nsLister:   workload.Core.Namespaces("").Controller().Lister(),
		rbLister:   workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:  workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:   workload.RBAC.ClusterRoles("").Controller().Lister(),
		pspLister:  workload.Extensions.PodSecurityPolicies("").Controller().Lister(),
	}
	workload.Management.Management.ProjectRoleTemplateBindings("").Controller().AddHandler(r.syncPRTB)
	workload.Management.Management.ClusterRoleTemplateBindings("").Controller().AddHandler(r.syncCRTB)
	workload.Core.Namespaces("").Controller().AddHandler(r.syncNS)
}

type roleHandler struct {
	workload   *config.ClusterContext
	rtLister   v3.RoleTemplateLister
	indexer    cache.Indexer
	psptLister v3.PodSecurityPolicyTemplateLister
	nsLister   typescorev1.NamespaceLister
	crLister   typesrbacv1.ClusterRoleLister
	crbLister  typesrbacv1.ClusterRoleBindingLister
	rbLister   typesrbacv1.RoleBindingLister
	pspLister  typesextv1beta1.PodSecurityPolicyLister
}

func (r *roleHandler) syncCRTB(key string, binding *v3.ClusterRoleTemplateBinding) error {
	if binding == nil {
		return nil
	}

	if binding.DeletionTimestamp != nil {
		return r.ensureCRTBDelete(key, binding)
	}

	return r.ensureCRTB(key, binding)
}

func (r *roleHandler) ensureCRTBDelete(key string, binding *v3.ClusterRoleTemplateBinding) error {
	if len(binding.ObjectMeta.Finalizers) <= 0 || binding.ObjectMeta.Finalizers[0] != finalizerName {
		return nil
	}

	binding = binding.DeepCopy()

	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	bindingCli := r.workload.K8sClient.RbacV1().ClusterRoleBindings()
	rbs, err := r.crbLister.List("", set.AsSelector())
	if err != nil {
		return errors.Wrapf(err, "couldn't list clusterrolebindings with selector %s", set.AsSelector())
	}

	for _, rb := range rbs {
		if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			return errors.Wrapf(err, "error deleting clusterrolebinding %v", rb.Name)
		}
	}

	if r.removeFinalizer(binding) {
		_, err := r.workload.Management.Management.ClusterRoleTemplateBindings("").Update(binding)
		return err
	}
	return nil
}

func (r *roleHandler) ensureCRTB(key string, binding *v3.ClusterRoleTemplateBinding) error {
	binding = binding.DeepCopy()
	if r.addFinalizer(binding) {
		if _, err := r.workload.Management.Management.ClusterRoleTemplateBindings("").Update(binding); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer set on %v", key)
		}
	}

	rt, err := r.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(rt, roles); err != nil {
		return err
	}

	if err := r.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	for _, role := range roles {
		if err := r.ensureClusterBinding(role.Name, binding); err != nil {
			return errors.Wrapf(err, "couldn't ensure cluster binding %v %v", role.Name, binding.Subject.Name)
		}
	}

	return nil
}

func (r *roleHandler) syncPRTB(key string, binding *v3.ProjectRoleTemplateBinding) error {
	if binding == nil {
		return nil
	}

	if binding.DeletionTimestamp != nil {
		return r.ensurePRTBDelete(key, binding)
	}

	return r.ensurePRTB(key, binding)
}

func (r *roleHandler) ensurePRTB(key string, binding *v3.ProjectRoleTemplateBinding) error {
	binding = binding.DeepCopy()
	added := r.addFinalizer(binding)
	if added {
		if _, err := r.workload.Management.Management.ProjectRoleTemplateBindings("").Update(binding); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer set on %v", key)
		}
	}

	rt, err := r.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	// Get namespaces belonging to project
	set := labels.Set(map[string]string{projectIDLabel: binding.ProjectName})
	namespaces, err := r.nsLister.List("", set.AsSelector())
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with selector %s", set.AsSelector())
	}
	if len(namespaces) == 0 {
		return nil
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(rt, roles); err != nil {
		return err
	}

	if err := r.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	// TODO is .Items the complete list or is there potential pagination to deal with?
	for _, ns := range namespaces {
		for _, role := range roles {
			if err := r.ensureBinding(ns.Name, role.Name, binding); err != nil {
				return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", role.Name, binding.Subject.Name, ns.Name)
			}
		}
	}

	return nil
}

func (r *roleHandler) ensurePRTBDelete(key string, binding *v3.ProjectRoleTemplateBinding) error {
	if len(binding.ObjectMeta.Finalizers) <= 0 || binding.ObjectMeta.Finalizers[0] != finalizerName {
		return nil
	}

	binding = binding.DeepCopy()

	// Get namespaces belonging to project
	set := labels.Set(map[string]string{projectIDLabel: binding.ProjectName})
	namespaces, err := r.nsLister.List("", set.AsSelector())
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with selector %s", set.AsSelector())
	}

	set = labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	for _, ns := range namespaces {
		bindingCli := r.workload.K8sClient.RbacV1().RoleBindings(ns.Name)
		rbs, err := r.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return errors.Wrapf(err, "couldn't list rolebindings with selector %s", set.AsSelector())
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				return errors.Wrapf(err, "error deleting rolebinding %v", rb.Name)
			}
		}
	}

	if r.removeFinalizer(binding) {
		_, err := r.workload.Management.Management.ProjectRoleTemplateBindings("").Update(binding)
		return err
	}
	return nil
}

func (r *roleHandler) gatherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	roleTemplates[rt.Name] = rt

	for _, rtName := range rt.RoleTemplateNames {
		subRT, err := r.rtLister.Get("", rtName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get RoleTemplate %s", rtName)
		}
		if err := r.gatherRoles(subRT, roleTemplates); err != nil {
			return errors.Wrapf(err, "couldn't gather RoleTemplate %s", rtName)
		}
	}

	return nil
}

func (r *roleHandler) addFinalizer(objectMeta metav1.Object) bool {
	if slice.ContainsString(objectMeta.GetFinalizers(), finalizerName) {
		return false
	}

	objectMeta.SetFinalizers(append(objectMeta.GetFinalizers(), finalizerName))
	return true
}

func (r *roleHandler) removeFinalizer(objectMeta metav1.Object) bool {
	if !slice.ContainsString(objectMeta.GetFinalizers(), finalizerName) {
		return false
	}

	changed := false
	var finalizers []string
	for _, finalizer := range objectMeta.GetFinalizers() {
		if finalizer == finalizerName {
			changed = true
			continue
		}
		finalizers = append(finalizers, finalizer)
	}

	if changed {
		objectMeta.SetFinalizers(finalizers)
	}

	return changed

}

func (r *roleHandler) ensureRoles(rts map[string]*v3.RoleTemplate) error {
	roleCli := r.workload.K8sClient.RbacV1().ClusterRoles()
	for _, rt := range rts {
		if rt.Builtin {
			// TODO assert the role exists and log an error if it doesnt.
			continue
		}

		if role, err := r.crLister.Get("", rt.Name); err == nil && role != nil {
			role = role.DeepCopy()
			// TODO potentially check a version so that we don't do unnecessary updates
			role.Rules = rt.Rules
			_, err := roleCli.Update(role)
			if err != nil {
				return errors.Wrapf(err, "couldn't update role %v", rt.Name)
			}
			continue
		}

		_, err := roleCli.Create(&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: rt.Name,
			},
			Rules: rt.Rules,
		})
		if err != nil {
			return errors.Wrapf(err, "couldn't create role %v", rt.Name)
		}
	}

	return nil
}

func (r *roleHandler) ensureClusterBinding(roleName string, binding *v3.ClusterRoleTemplateBinding) error {
	bindingCli := r.workload.K8sClient.RbacV1().ClusterRoleBindings()
	bindingName, objectMeta, subjects, roleRef := bindingParts(roleName, string(binding.UID), binding.Subject)
	if c, _ := r.crbLister.Get("", bindingName); c != nil {
		return nil
	}

	_, err := bindingCli.Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: objectMeta,
		Subjects:   subjects,
		RoleRef:    roleRef,
	})

	return err
}

func (r *roleHandler) ensureBinding(ns, roleName string, binding *v3.ProjectRoleTemplateBinding) error {
	bindingCli := r.workload.K8sClient.RbacV1().RoleBindings(ns)
	bindingName, objectMeta, subjects, roleRef := bindingParts(roleName, string(binding.UID), binding.Subject)
	if b, _ := r.rbLister.Get("", bindingName); b != nil {
		return nil
	}
	_, err := bindingCli.Create(&rbacv1.RoleBinding{
		ObjectMeta: objectMeta,
		Subjects:   subjects,
		RoleRef:    roleRef,
	})
	return err
}

func bindingParts(roleName, parentUID string, subject rbacv1.Subject) (string, metav1.ObjectMeta, []rbacv1.Subject, rbacv1.RoleRef) {
	bindingName := strings.ToLower(fmt.Sprintf("%v-%v-%v", roleName, subject.Name, parentUID))
	return bindingName,
		metav1.ObjectMeta{
			Name:   bindingName,
			Labels: map[string]string{rtbOwnerLabel: parentUID},
		},
		[]rbacv1.Subject{subject},
		rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: roleName,
		}
}

func prtbIndexer(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		logrus.Infof("object %v is not Project Role Template Binding", obj)
		return []string{}, nil
	}

	return []string{prtb.ProjectName}, nil
}
