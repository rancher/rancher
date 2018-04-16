package rbac

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	typescorev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	rtbOwnerLabel                    = "authz.cluster.cattle.io/rtb-owner"
	projectIDAnnotation              = "field.cattle.io/projectId"
	prtbByProjectIndex               = "authz.cluster.cattle.io/prtb-by-project"
	prtbByProjecSubjectIndex         = "authz.cluster.cattle.io/prtb-by-project-subject"
	rtbByClusterAndRoleTemplateIndex = "authz.cluster.cattle.io/rtb-by-cluster-rt"
	nsByProjectIndex                 = "authz.cluster.cattle.io/ns-by-project"
	crByNSIndex                      = "authz.cluster.cattle.io/cr-by-ns"
	crbByRoleAndSubjectIndex         = "authz.cluster.cattle.io/crb-by-role-and-subject"
)

func Register(workload *config.UserContext) {
	// Add cache informer to project role template bindings
	prtbInformer := workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbByProjectIndex:               prtbByProjectName,
		prtbByProjecSubjectIndex:         prtbByProjectAndSubject,
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
	}
	prtbInformer.AddIndexers(prtbIndexers)

	crtbInformer := workload.Management.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
	}
	crtbInformer.AddIndexers(crtbIndexers)

	// Index for looking up namespaces by projectID annotation
	nsInformer := workload.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsByProjectIndex: nsByProjectID,
	}
	nsInformer.AddIndexers(nsIndexers)

	// Get ClusterRoles by the namespaces the authorizes because they are in a project
	crInformer := workload.RBAC.ClusterRoles("").Controller().Informer()
	crIndexers := map[string]cache.IndexFunc{
		crByNSIndex: crByNS,
	}
	crInformer.AddIndexers(crIndexers)

	// Get ClusterRoleBindings by subject name and kind
	crbInformer := workload.RBAC.ClusterRoleBindings("").Controller().Informer()
	crbIndexers := map[string]cache.IndexFunc{
		crbByRoleAndSubjectIndex: crbByRoleAndSubject,
	}
	crbInformer.AddIndexers(crbIndexers)

	r := &manager{
		workload:      workload,
		prtbIndexer:   prtbInformer.GetIndexer(),
		crtbIndexer:   crtbInformer.GetIndexer(),
		nsIndexer:     nsInformer.GetIndexer(),
		crIndexer:     crInformer.GetIndexer(),
		crbIndexer:    crbInformer.GetIndexer(),
		rtLister:      workload.Management.Management.RoleTemplates("").Controller().Lister(),
		rbLister:      workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:     workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:      workload.RBAC.ClusterRoles("").Controller().Lister(),
		nsLister:      workload.Core.Namespaces("").Controller().Lister(),
		clusterLister: workload.Management.Management.Clusters("").Controller().Lister(),
		projectLister: workload.Management.Management.Projects("").Controller().Lister(),
		clusterName:   workload.ClusterName,
	}
	workload.Management.Management.Projects("").AddClusterScopedLifecycle("project-namespace-auth", workload.ClusterName, newProjectLifecycle(r))
	workload.Management.Management.ProjectRoleTemplateBindings("").AddClusterScopedLifecycle("cluster-prtb-sync", workload.ClusterName, newPRTBLifecycle(r))
	workload.Management.Management.ClusterRoleTemplateBindings("").AddClusterScopedLifecycle("cluster-crtb-sync", workload.ClusterName, newCRTBLifecycle(r))
	workload.Management.Management.Clusters("").AddHandler("global-admin-cluster-sync", newClusterHandler(workload))
	workload.Core.Namespaces("").AddLifecycle("namespace-auth", newNamespaceLifecycle(r))

	// This method for creating a lifecycle creates a cluster scoped handler for a non-cluster scoped resource.
	// This means that when the cluster is deleted, cleanup logic in the mgmt cluster will remove the finalizer added by this
	// lifecycle, but during normal operation the events received by the handler will not be filtered to match the cluster
	// (because the resource -roleTemplate and globalRoleBinding- are global resources)
	rti := workload.Management.Management.RoleTemplates("")
	rtSync := v3.NewRoleTemplateLifecycleAdapter("cluster-roletemplate-sync_"+workload.ClusterName, true, rti, newRTLifecycle(r))
	workload.Management.Management.RoleTemplates("").AddHandler("cluster-roletemplate-sync", rtSync)

	grbi := workload.Management.Management.GlobalRoleBindings("")
	grbSync := v3.NewGlobalRoleBindingLifecycleAdapter("grb-sync_"+workload.ClusterName, true, grbi, newGlobalRoleBindingHandler(workload))
	workload.Management.Management.GlobalRoleBindings("").AddHandler("grb-sync", grbSync)
}

type manager struct {
	workload      *config.UserContext
	rtLister      v3.RoleTemplateLister
	prtbIndexer   cache.Indexer
	crtbIndexer   cache.Indexer
	nsIndexer     cache.Indexer
	crIndexer     cache.Indexer
	crbIndexer    cache.Indexer
	crLister      typesrbacv1.ClusterRoleLister
	crbLister     typesrbacv1.ClusterRoleBindingLister
	rbLister      typesrbacv1.RoleBindingLister
	nsLister      typescorev1.NamespaceLister
	clusterLister v3.ClusterLister
	projectLister v3.ProjectLister
	clusterName   string
}

func (m *manager) ensureRoles(rts map[string]*v3.RoleTemplate) error {
	roleCli := m.workload.K8sClient.RbacV1().ClusterRoles()
	for _, rt := range rts {
		if rt.External {
			continue
		}

		if role, err := m.crLister.Get("", rt.Name); err == nil && role != nil {
			if reflect.DeepEqual(role.Rules, rt.Rules) {
				continue
			}
			role = role.DeepCopy()
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

func (m *manager) gatherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	err := m.gatherRolesRecurse(rt, roleTemplates)
	if err != nil {
		return err
	}

	// clean the roles for kubeneretes: lowercase resources and verbs
	for key, rt := range roleTemplates {
		if rt.External {
			continue
		}
		rt = rt.DeepCopy()

		var toLowerRules []rbacv1.PolicyRule
		for _, r := range rt.Rules {
			rule := r.DeepCopy()

			var resources []string
			for _, re := range r.Resources {
				resources = append(resources, strings.ToLower(re))
			}
			rule.Resources = resources

			var verbs []string
			for _, v := range r.Verbs {
				verbs = append(verbs, strings.ToLower(v))
			}
			rule.Verbs = verbs
			toLowerRules = append(toLowerRules, *rule)
		}
		rt.Rules = toLowerRules
		roleTemplates[key] = rt
	}

	return nil
}

func (m *manager) gatherRolesRecurse(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	roleTemplates[rt.Name] = rt

	for _, rtName := range rt.RoleTemplateNames {
		subRT, err := m.rtLister.Get("", rtName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get RoleTemplate %s", rtName)
		}
		if err := m.gatherRoles(subRT, roleTemplates); err != nil {
			return errors.Wrapf(err, "couldn't gather RoleTemplate %s", rtName)
		}
	}

	return nil
}

func (m *manager) ensureClusterBindings(roles map[string]*v3.RoleTemplate, binding *v3.ClusterRoleTemplateBinding) error {
	create := func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object {
		return &rbacv1.ClusterRoleBinding{
			ObjectMeta: objectMeta,
			Subjects:   subjects,
			RoleRef:    roleRef,
		}
	}

	list := func(ns string, selector labels.Selector) ([]interface{}, error) {
		currentRBs, err := m.crbLister.List(ns, selector)
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
		crb, _ := i.(*rbacv1.ClusterRoleBinding)
		return crb.Name, crb.RoleRef.Name, crb.Subjects
	}

	return m.ensureBindings("", roles, binding, m.workload.RBAC.ClusterRoleBindings("").ObjectClient(), create, list, convert)
}

func (m *manager) ensureProjectRoleBindings(ns string, roles map[string]*v3.RoleTemplate, binding *v3.ProjectRoleTemplateBinding) error {
	create := func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object {
		return &rbacv1.RoleBinding{
			ObjectMeta: objectMeta,
			Subjects:   subjects,
			RoleRef:    roleRef,
		}
	}

	list := func(ns string, selector labels.Selector) ([]interface{}, error) {
		currentRBs, err := m.rbLister.List(ns, selector)
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

	return m.ensureBindings(ns, roles, binding, m.workload.RBAC.RoleBindings(ns).ObjectClient(), create, list, convert)
}

type createFn func(objectMeta metav1.ObjectMeta, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) runtime.Object
type listFn func(ns string, selector labels.Selector) ([]interface{}, error)
type convertFn func(i interface{}) (string, string, []rbacv1.Subject)

func (m *manager) ensureBindings(ns string, roles map[string]*v3.RoleTemplate, binding interface{}, client *objectclient.ObjectClient,
	create createFn, list listFn, convert convertFn) error {
	meta, err := meta.Accessor(binding)
	if err != nil {
		return err
	}

	desiredRBs := map[string]runtime.Object{}
	subject, err := buildSubjectFromRTB(binding)
	if err != nil {
		return err
	}
	for roleName := range roles {
		rbKey, objectMeta, subjects, roleRef := bindingParts(roleName, string(meta.GetUID()), subject)
		desiredRBs[rbKey] = create(objectMeta, subjects, roleRef)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: string(meta.GetUID())})
	currentRBs, err := list(ns, set.AsSelector())
	if err != nil {
		return err
	}
	rbsToDelete := map[string]bool{}
	processed := map[string]bool{}
	for _, rb := range currentRBs {
		rbName, roleName, subjects := convert(rb)
		// protect against an rb being in the list more than once (shouldn't happen, but just to be safe)
		if ok := processed[rbName]; ok {
			continue
		}
		processed[rbName] = true

		if len(subjects) != 1 {
			rbsToDelete[rbName] = true
			continue
		}

		crbKey := rbRoleSubjectKey(roleName, subjects[0])
		if _, ok := desiredRBs[crbKey]; ok {
			delete(desiredRBs, crbKey)
		} else {
			rbsToDelete[rbName] = true
		}
	}

	for _, rb := range desiredRBs {
		_, err := client.Create(rb)
		if err != nil {
			return err
		}
	}

	for name := range rbsToDelete {
		if err := client.Delete(name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func buildSubjectFromRTB(binding interface{}) (rbacv1.Subject, error) {
	// TODO This is a duplicate of the same method that lives in the management context. When a place for common
	// code exists, move it there and reuse it
	var userName, groupPrincipalName, groupName, name, kind string
	if rtb, ok := binding.(*v3.ProjectRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else if rtb, ok := binding.(*v3.ClusterRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else {
		return rbacv1.Subject{}, errors.Errorf("unrecognized roleTemplateBinding type: %v", binding)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupName
		kind = "Group"
	}

	if name == "" {
		return rbacv1.Subject{}, errors.Errorf("roletemplatebinding doesn't have any subject fields set: %v", binding)
	}

	return rbacv1.Subject{
		Kind: kind,
		Name: name,
	}, nil
}

func bindingParts(roleName, parentUID string, subject rbacv1.Subject) (string, metav1.ObjectMeta, []rbacv1.Subject, rbacv1.RoleRef) {
	crbKey := rbRoleSubjectKey(roleName, subject)
	return crbKey,
		metav1.ObjectMeta{
			GenerateName: "clusterrolebinding-",
			Labels:       map[string]string{rtbOwnerLabel: parentUID},
		},
		[]rbacv1.Subject{subject},
		rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: roleName,
		}
}

func prtbByProjectName(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{prtb.ProjectName}, nil
}

func getPRTBProjectAndSubjectKey(prtb *v3.ProjectRoleTemplateBinding) string {
	var name string
	if prtb.UserName != "" {
		name = prtb.UserName
	} else if prtb.UserPrincipalName != "" {
		name = prtb.UserPrincipalName
	} else if prtb.GroupName != "" {
		name = prtb.GroupName
	} else {
		name = prtb.GroupPrincipalName
	}
	return prtb.ProjectName + "." + name
}

func prtbByProjectAndSubject(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{getPRTBProjectAndSubjectKey(prtb)}, nil
}

func nsByProjectID(obj interface{}) ([]string, error) {
	ns, ok := obj.(*v1.Namespace)
	if !ok {
		return []string{}, nil
	}

	if id, ok := ns.Annotations[projectIDAnnotation]; ok {
		return []string{id}, nil
	}

	return []string{}, nil
}

func crbRoleSubjectKeys(roleName string, subjects []rbacv1.Subject) []string {
	var keys []string
	for _, s := range subjects {
		keys = append(keys, rbRoleSubjectKey(roleName, s))
	}
	return keys
}

func rbRoleSubjectKey(roleName string, subject rbacv1.Subject) string {
	return roleName + "." + subject.Kind + "." + subject.Name

}

func crbByRoleAndSubject(obj interface{}) ([]string, error) {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return crbRoleSubjectKeys(crb.RoleRef.Name, crb.Subjects), nil
}

func rtbByClusterAndRoleTemplateName(obj interface{}) ([]string, error) {
	var idx string
	switch rtb := obj.(type) {
	case *v3.ProjectRoleTemplateBinding:
		if rtb.RoleTemplateName != "" && rtb.ProjectName != "" {
			parts := strings.SplitN(rtb.ProjectName, ":", 2)
			if len(parts) == 2 {
				idx = parts[0] + "-" + rtb.RoleTemplateName
			}
		}
	case *v3.ClusterRoleTemplateBinding:
		if rtb.RoleTemplateName != "" && rtb.ClusterName != "" {
			idx = rtb.ClusterName + "-" + rtb.RoleTemplateName
		}
	}

	if idx == "" {
		return []string{}, nil
	}
	return []string{idx}, nil
}
