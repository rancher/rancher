package authz

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	typesextv1beta1 "github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	finalizerName       = "rtbFinalizer"
	rtbOwnerLabel       = "io.cattle.rtb.owner"
	projectIDAnnotation = "field.cattle.io/projectId"
	prtbIndex           = "authz.cluster.cattle.io/prtb-index"
	nsIndex             = "authz.cluster.cattle.io/ns-index"
)

func Register(workload *config.ClusterContext) {
	// Add cache informer to project role template bindings
	informer := workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		prtbIndex: prtbIndexer,
	}
	informer.AddIndexers(indexers)

	// Index for looking up namespaces by projectID annotation
	nsInformer := workload.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsIndex: nsIndexer,
	}
	nsInformer.AddIndexers(nsIndexers)

	r := &roleHandler{
		workload:    workload,
		prtbIndexer: informer.GetIndexer(),
		nsIndexer:   nsInformer.GetIndexer(),
		rtLister:    workload.Management.Management.RoleTemplates("").Controller().Lister(),
		psptLister:  workload.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		rbLister:    workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:   workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:    workload.RBAC.ClusterRoles("").Controller().Lister(),
		pspLister:   workload.Extensions.PodSecurityPolicies("").Controller().Lister(),
	}
	workload.Management.Management.ProjectRoleTemplateBindings("").Controller().AddHandler(r.syncPRTB)
	workload.Management.Management.ClusterRoleTemplateBindings("").Controller().AddHandler(r.syncCRTB)
	workload.Management.Management.RoleTemplates("").Controller().AddHandler(r.syncRT)
	workload.Core.Namespaces("").Controller().AddHandler(r.syncNS)

	namespaceLifecycle := newNSLifecycle(workload)
	workload.Core.Namespaces("").AddLifecycle("defaultNamespace", namespaceLifecycle)

}

type roleHandler struct {
	workload    *config.ClusterContext
	rtLister    v3.RoleTemplateLister
	prtbIndexer cache.Indexer
	nsIndexer   cache.Indexer
	psptLister  v3.PodSecurityPolicyTemplateLister
	crLister    typesrbacv1.ClusterRoleLister
	crbLister   typesrbacv1.ClusterRoleBindingLister
	rbLister    typesrbacv1.RoleBindingLister
	pspLister   typesextv1beta1.PodSecurityPolicyLister
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
			if err = checkDeleteErr(err); err != nil {
				return errors.Wrapf(err, "error deleting clusterrolebinding %v", rb.Name)
			}
		}
	}

	if r.removeFinalizer(binding) {
		_, err := r.workload.Management.Management.ClusterRoleTemplateBindings(binding.Namespace).Update(binding)
		return err
	}
	return nil
}

func (r *roleHandler) ensureCRTB(key string, binding *v3.ClusterRoleTemplateBinding) error {
	binding = binding.DeepCopy()
	if r.addFinalizer(binding) {
		if _, err := r.workload.Management.Management.ClusterRoleTemplateBindings(binding.Namespace).Update(binding); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer on %v", key)
		}
	}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.Subject.Name == "" {
		logrus.Warnf("Binding %v has no subject. Skipping", binding.Name)
		return nil
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
		if _, err := r.workload.Management.Management.ProjectRoleTemplateBindings(binding.Namespace).Update(binding); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer on %v", key)
		}
	}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}
	if binding.Subject.Name == "" {
		logrus.Warnf("Binding %v has no subject. Skipping", binding.Name)
		return nil
	}

	rt, err := r.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	// Get namespaces belonging to project
	namespaces, err := r.nsIndexer.ByIndex(nsIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
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

	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
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
	namespaces, err := r.nsIndexer.ByIndex(nsIndex, binding.ProjectName)
	if err != nil {
		return errors.Wrapf(err, "couldn't list namespaces with project ID %v", binding.ProjectName)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: string(binding.UID)})
	for _, n := range namespaces {
		ns := n.(*v1.Namespace)
		bindingCli := r.workload.K8sClient.RbacV1().RoleBindings(ns.Name)
		rbs, err := r.rbLister.List(ns.Name, set.AsSelector())
		if err != nil {
			return errors.Wrapf(err, "couldn't list rolebindings with selector %s", set.AsSelector())
		}

		for _, rb := range rbs {
			if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if err = checkDeleteErr(err); err != nil {
					return errors.Wrapf(err, "error deleting rolebinding %v", rb.Name)
				}
			}
		}
	}

	if r.removeFinalizer(binding) {
		_, err := r.workload.Management.Management.ProjectRoleTemplateBindings(binding.Namespace).Update(binding)
		return err
	}
	return nil
}

func (r *roleHandler) syncRT(key string, template *v3.RoleTemplate) error {
	if template == nil {
		return nil
	}

	if template.DeletionTimestamp != nil {
		return r.ensureRTDelete(key, template)
	}

	return r.ensureRT(key, template)
}

func (r *roleHandler) ensureRT(key string, template *v3.RoleTemplate) error {
	template = template.DeepCopy()
	if r.addFinalizer(template) {
		if _, err := r.workload.Management.Management.RoleTemplates("").Update(template); err != nil {
			return errors.Wrapf(err, "couldn't set finalizer on %v", key)
		}
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(template, roles); err != nil {
		return err
	}

	if err := r.ensureRoles(roles); err != nil {
		return errors.Wrapf(err, "couldn't ensure roles")
	}

	return nil
}

func (r *roleHandler) ensureRTDelete(key string, template *v3.RoleTemplate) error {
	if len(template.ObjectMeta.Finalizers) <= 0 || template.ObjectMeta.Finalizers[0] != finalizerName {
		return nil
	}

	template = template.DeepCopy()

	roles := map[string]*v3.RoleTemplate{}
	if err := r.gatherRoles(template, roles); err != nil {
		return err
	}

	roleCli := r.workload.K8sClient.RbacV1().ClusterRoles()
	for _, role := range roles {
		if err := roleCli.Delete(role.Name, &metav1.DeleteOptions{}); err != nil {
			if err = checkDeleteErr(err); err != nil {
				return errors.Wrapf(err, "error deleting clusterrole %v", role.Name)
			}
		}
	}

	if r.removeFinalizer(template) {
		_, err := r.workload.Management.Management.RoleTemplates("").Update(template)
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
	if b, _ := r.rbLister.Get(ns, bindingName); b != nil {
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

func nsIndexer(obj interface{}) ([]string, error) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		logrus.Infof("object %v is not a namespace", obj)
		return []string{}, nil
	}

	if id, ok := ns.Annotations[projectIDAnnotation]; ok {
		return []string{id}, nil
	}

	return []string{}, nil
}

func prtbIndexer(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		logrus.Infof("object %v is not Project Role Template Binding", obj)
		return []string{}, nil
	}

	return []string{prtb.ProjectName}, nil
}

func checkDeleteErr(e error) error {
	if err, ok := e.(*k8serr.StatusError); ok {
		if err.ErrStatus.Code == 404 {
			return nil
		}
	}
	return e
}
