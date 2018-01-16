package authz

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	finalizerName       = "rtbFinalizer-"
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
		workload:      workload,
		prtbIndexer:   informer.GetIndexer(),
		nsIndexer:     nsInformer.GetIndexer(),
		rtLister:      workload.Management.Management.RoleTemplates("").Controller().Lister(),
		rbLister:      workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:     workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:      workload.RBAC.ClusterRoles("").Controller().Lister(),
		clusterLister: workload.Management.Management.Clusters("").Controller().Lister(),
		clusterName:   workload.ClusterName,
		finalizerName: finalizerName + workload.ClusterName,
	}
	workload.Management.Management.ProjectRoleTemplateBindings("").AddHandler("auth", r.syncPRTB)
	workload.Management.Management.ClusterRoleTemplateBindings("").AddHandler("auth", r.syncCRTB)
	workload.Management.Management.RoleTemplates("").AddHandler("auth", r.syncRT)
	workload.Core.Namespaces("").AddHandler("auth", r.syncNS)
}

func prtbIndexer(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		logrus.Infof("object %v is not Project Role Template Binding", obj)
		return []string{}, nil
	}

	return []string{prtb.ProjectName}, nil
}

type roleHandler struct {
	workload      *config.ClusterContext
	rtLister      v3.RoleTemplateLister
	prtbIndexer   cache.Indexer
	nsIndexer     cache.Indexer
	crLister      typesrbacv1.ClusterRoleLister
	crbLister     typesrbacv1.ClusterRoleBindingLister
	rbLister      typesrbacv1.RoleBindingLister
	clusterLister v3.ClusterLister
	clusterName   string
	finalizerName string
}

func (r *roleHandler) addFinalizer(objectMeta metav1.Object) bool {
	if slice.ContainsString(objectMeta.GetFinalizers(), r.finalizerName) {
		return false
	}

	objectMeta.SetFinalizers(append(objectMeta.GetFinalizers(), r.finalizerName))
	return true
}

func (r *roleHandler) removeFinalizer(objectMeta metav1.Object) bool {
	if !slice.ContainsString(objectMeta.GetFinalizers(), r.finalizerName) {
		return false
	}

	changed := false
	var finalizers []string
	for _, finalizer := range objectMeta.GetFinalizers() {
		if finalizer == r.finalizerName {
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
