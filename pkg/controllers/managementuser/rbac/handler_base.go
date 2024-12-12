package rbac

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types/convert"
	wranglerv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	typescorev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	nsutils "github.com/rancher/rancher/pkg/namespace"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	rtbOwnerLabel                    = "authz.cluster.cattle.io/rtb-owner-updated"
	rtbOwnerLabelLegacy              = "authz.cluster.cattle.io/rtb-owner"
	clusterRoleOwner                 = "authz.cluster.cattle.io/clusterrole-owner"
	projectIDAnnotation              = "field.cattle.io/projectId"
	prtbByProjectIndex               = "authz.cluster.cattle.io/prtb-by-project"
	prtbByProjecSubjectIndex         = "authz.cluster.cattle.io/prtb-by-project-subject"
	prtbByUIDIndex                   = "authz.cluster.cattle.io/rtb-owner"
	prtbByNsAndNameIndex             = "authz.cluster.cattle.io/rtb-owner-updated"
	rtbByClusterAndRoleTemplateIndex = "authz.cluster.cattle.io/rtb-by-cluster-rt"
	rtbByClusterAndUserIndex         = "authz.cluster.cattle.io/rtb-by-cluster-user"
	crByNSIndex                      = "authz.cluster.cattle.io/cr-by-ns"
	crbByRoleAndSubjectIndex         = "authz.cluster.cattle.io/crb-by-role-and-subject"
	rtbLabelUpdated                  = "authz.cluster.cattle.io/rtb-label-updated"
	rtbCrbRbLabelsUpdated            = "authz.cluster.cattle.io/crb-rb-labels-updated"
	rtByInheritedRTsIndex            = "authz.cluster.cattle.io/rts-by-inherited-rts"
	impersonationLabel               = "authz.cluster.cattle.io/impersonator"

	rolesCircularSoftLimit = 100
	rolesCircularHardLimit = 500
)

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-handler-base")

	// Add cache informer to project role template bindings
	prtbInformer := workload.Management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		nsutils.PrtbByRoleTemplateIndex: nsutils.PrtbByRoleTemplateName,
	}
	prtbInformer.AddIndexers(prtbIndexers)

	// Add cache informer to cluster role template bindings
	crtbInformer := workload.Management.Management.ClusterRoleTemplateBindings("").Controller().Informer()

	// Index for looking up namespaces by projectID annotation
	nsInformer := workload.Core.Namespaces("").Controller().Informer()
	nsIndexers := map[string]cache.IndexFunc{
		nsutils.NsByProjectIndex: nsutils.NsByProjectID,
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

	// Get RoleTemplates by RoleTemplate they inherit from
	rtInformer := workload.Management.Wrangler.Mgmt.RoleTemplate().Informer()
	rtIndexers := map[string]cache.IndexFunc{
		rtByInheritedRTsIndex: rtByInterhitedRTs,
	}
	rtInformer.AddIndexers(rtIndexers)

	r := &manager{
		workload:            workload,
		prtbIndexer:         prtbInformer.GetIndexer(),
		crtbIndexer:         crtbInformer.GetIndexer(),
		nsIndexer:           nsInformer.GetIndexer(),
		crIndexer:           crInformer.GetIndexer(),
		crbIndexer:          crbInformer.GetIndexer(),
		rtLister:            management.Management.RoleTemplates("").Controller().Lister(),
		rbLister:            workload.RBAC.RoleBindings("").Controller().Lister(),
		crbLister:           workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:            workload.RBAC.ClusterRoles("").Controller().Lister(),
		clusterRoles:        workload.RBAC.ClusterRoles(""),
		clusterRoleBindings: workload.RBAC.ClusterRoleBindings(""),
		nsLister:            workload.Core.Namespaces("").Controller().Lister(),
		nsController:        workload.Core.Namespaces("").Controller(),
		clusterLister:       management.Management.Clusters("").Controller().Lister(),
		projectLister:       management.Management.Projects(workload.ClusterName).Controller().Lister(),
		userLister:          management.Management.Users("").Controller().Lister(),
		userAttributeLister: management.Management.UserAttributes("").Controller().Lister(),
		clusterName:         workload.ClusterName,
	}
	management.Management.Projects(workload.ClusterName).AddClusterScopedLifecycle(ctx, "project-namespace-auth", workload.ClusterName, newProjectLifecycle(r))
	management.Management.ProjectRoleTemplateBindings("").AddClusterScopedLifecycle(ctx, "cluster-prtb-sync", workload.ClusterName, newPRTBLifecycle(r, management, nsInformer))
	workload.RBAC.ClusterRoles("").AddHandler(ctx, "cluster-clusterrole-sync", newClusterRoleHandler(r).sync)
	workload.RBAC.ClusterRoleBindings("").AddHandler(ctx, "legacy-crb-cleaner-sync", newLegacyCRBCleaner(r).sync)
	management.Management.ClusterRoleTemplateBindings("").AddClusterScopedLifecycle(ctx, "cluster-crtb-sync", workload.ClusterName, newCRTBLifecycle(r, management))
	management.Management.Clusters("").AddHandler(ctx, "global-admin-cluster-sync", newClusterHandler(workload))
	management.Management.GlobalRoleBindings("").AddHandler(ctx, grbHandlerName, newGlobalRoleBindingHandler(workload))

	sync := &resourcequota.SyncController{
		Namespaces:          workload.Core.Namespaces(""),
		NsIndexer:           nsInformer.GetIndexer(),
		ResourceQuotas:      workload.Core.ResourceQuotas(""),
		ResourceQuotaLister: workload.Core.ResourceQuotas("").Controller().Lister(),
		LimitRange:          workload.Core.LimitRanges(""),
		LimitRangeLister:    workload.Core.LimitRanges("").Controller().Lister(),
		ProjectLister:       management.Management.Projects(workload.ClusterName).Controller().Lister(),
	}

	workload.Core.Namespaces("").AddLifecycle(ctx, "namespace-auth", newNamespaceLifecycle(r, sync))
	management.Management.RoleTemplates("").AddHandler(ctx, "cluster-roletemplate-sync", newRTLifecycle(r))
	relatedresource.WatchClusterScoped(ctx, "enqueue-beneficiary-roletemplates", newRTEnqueueFunc(rtInformer.GetIndexer()),
		management.Wrangler.Mgmt.RoleTemplate(), management.Wrangler.Mgmt.RoleTemplate())

	nsEnqueuer := nsutils.NsEnqueuer{
		PrtbCache: prtbInformer.GetIndexer(),
		NsIndexer: nsInformer.GetIndexer(),
	}
	relatedresource.WatchClusterScoped(ctx, "enqueue-namespaces-by-roletemplate", nsEnqueuer.RoleTemplateEnqueueNamespace, workload.Corew.Namespace(), management.Wrangler.Mgmt.RoleTemplate())
}

type managerInterface interface {
	gatherRoles(*v3.RoleTemplate, map[string]*v3.RoleTemplate, int) error
	ensureRoles(map[string]*v3.RoleTemplate) error
	ensureClusterBindings(map[string]*v3.RoleTemplate, *v3.ClusterRoleTemplateBinding) error
	ensureProjectRoleBindings(string, map[string]*v3.RoleTemplate, *v3.ProjectRoleTemplateBinding) error
	ensureServiceAccountImpersonator(string) error
	deleteServiceAccountImpersonator(string) error
	ensureGlobalResourcesRolesForPRTB(string, map[string]*v3.RoleTemplate) ([]string, error)
	reconcileProjectAccessToGlobalResources(*v3.ProjectRoleTemplateBinding, []string) (map[string]bool, error)
	noRemainingOwnerLabels(*rbacv1.ClusterRoleBinding) (bool, error)
}

type manager struct {
	workload            *config.UserContext
	rtLister            v3.RoleTemplateLister
	prtbIndexer         cache.Indexer
	crtbIndexer         cache.Indexer
	nsIndexer           cache.Indexer
	crIndexer           cache.Indexer
	crbIndexer          cache.Indexer
	crLister            typesrbacv1.ClusterRoleLister
	clusterRoles        typesrbacv1.ClusterRoleInterface
	crbLister           typesrbacv1.ClusterRoleBindingLister
	clusterRoleBindings typesrbacv1.ClusterRoleBindingInterface
	rbLister            typesrbacv1.RoleBindingLister
	nsLister            typescorev1.NamespaceLister
	nsController        typescorev1.NamespaceController
	clusterLister       v3.ClusterLister
	projectLister       v3.ProjectLister
	userLister          v3.UserLister
	userAttributeLister v3.UserAttributeLister
	clusterName         string
}

func (m *manager) ensureRoles(rts map[string]*v3.RoleTemplate) error {
	for _, rt := range rts {
		if rt.External {
			continue
		}
		if err := m.ensureClusterRoles(rt); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) ensureClusterRoles(rt *v3.RoleTemplate) error {
	if clusterRole, err := m.crLister.Get("", rt.Name); err == nil && clusterRole != nil {
		err := m.compareAndUpdateClusterRole(clusterRole, rt)
		if err == nil {
			return nil
		}
		if apierrors.IsConflict(err) {
			// get object from etcd and retry
			clusterRole, err := m.clusterRoles.Get(rt.Name, metav1.GetOptions{})
			if err != nil {
				return errors.Wrapf(err, "error getting clusterRole %v", rt.Name)
			}
			return m.compareAndUpdateClusterRole(clusterRole, rt)
		}
		return errors.Wrapf(err, "couldn't update clusterRole %v", rt.Name)
	}

	return m.createClusterRole(rt)
}

func (m *manager) compareAndUpdateClusterRole(clusterRole *rbacv1.ClusterRole, rt *v3.RoleTemplate) error {
	if equality.Semantic.DeepEqual(clusterRole.Rules, rt.Rules) {
		return nil
	}
	clusterRole = clusterRole.DeepCopy()
	clusterRole.Rules = rt.Rules
	logrus.Infof("Updating clusterRole %v because of rules difference with roleTemplate %v (%v).", clusterRole.Name, rt.DisplayName, rt.Name)
	_, err := m.clusterRoles.Update(clusterRole)
	if err != nil {
		return errors.Wrapf(err, "couldn't update clusterRole %v", rt.Name)
	}
	return nil
}

func (m *manager) createClusterRole(rt *v3.RoleTemplate) error {
	logrus.Infof("Creating clusterRole for roleTemplate %v (%v).", rt.DisplayName, rt.Name)
	_, err := m.clusterRoles.Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rt.Name,
			Annotations: map[string]string{clusterRoleOwner: rt.Name},
		},
		Rules: rt.Rules,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "couldn't create clusterRole %v", rt.Name)
	}
	return nil
}

func ToLowerRoleTemplates(roleTemplates map[string]*v3.RoleTemplate) {
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
}

func (m *manager) gatherRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate, depthCounter int) error {
	if depthCounter == rolesCircularSoftLimit {
		logrus.Warnf("roletemplate has caused %v recursive function calls", rolesCircularSoftLimit)
	}
	if depthCounter >= rolesCircularHardLimit {
		return fmt.Errorf("roletemplate '%s' has caused %d recursive function calls, possible circular dependency", rt.Name, rolesCircularHardLimit)
	}
	err := m.gatherRolesRecurse(rt, roleTemplates, depthCounter)
	if err != nil {
		return err
	}
	ToLowerRoleTemplates(roleTemplates)
	return nil
}

func (m *manager) gatherRolesRecurse(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate, depthCounter int) error {
	roleTemplates[rt.Name] = rt
	depthCounter++

	for _, rtName := range rt.RoleTemplateNames {
		subRT, err := m.rtLister.Get("", rtName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get RoleTemplate %s", rtName)
		}
		if err := m.gatherRoles(subRT, roleTemplates, depthCounter); err != nil {
			return err
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

func (m *manager) ensureBindings(ns string, roles map[string]*v3.RoleTemplate, binding metav1.Object, client *objectclient.ObjectClient,
	create createFn, list listFn, convert convertFn) error {
	objMeta := meta.AsPartialObjectMetadata(binding).ObjectMeta

	desiredRBs := map[string]runtime.Object{}
	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		return err
	}
	for roleName := range roles {
		rbKey, objectMeta, subjects, roleRef := bindingParts(ns, roleName, objMeta, subject)
		desiredRBs[rbKey] = create(objectMeta, subjects, roleRef)
	}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(objMeta)})
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

	for key, rb := range desiredRBs {
		switch roleBinding := rb.(type) {
		case *rbacv1.RoleBinding:
			_, err := m.workload.RBAC.RoleBindings("").Controller().Lister().Get(ns, roleBinding.Name)
			if apierrors.IsNotFound(err) {
				logrus.Infof("Creating roleBinding %v in %s", key, ns)
				_, err := m.workload.RBAC.RoleBindings(ns).Create(roleBinding)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			} else if err != nil {
				return err
			}
		case *rbacv1.ClusterRoleBinding:
			logrus.Infof("Creating clusterRoleBinding %v", key)
			_, err := m.workload.RBAC.ClusterRoleBindings("").Create(roleBinding)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	for name := range rbsToDelete {
		logrus.Infof("Deleting roleBinding %v", name)
		if err := client.Delete(name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func bindingParts(namespace, roleName string, objMeta metav1.ObjectMeta, subject rbacv1.Subject) (string, metav1.ObjectMeta, []rbacv1.Subject, rbacv1.RoleRef) {
	key := rbRoleSubjectKey(roleName, subject)

	roleRef := rbacv1.RoleRef{
		Kind: "ClusterRole",
		Name: roleName,
	}

	var name string
	if namespace == "" { // if namespace is empty, binding will be ClusterRoleBinding, so name accordingly
		name = pkgrbac.NameForClusterRoleBinding(roleRef, subject)
	} else {
		name = pkgrbac.NameForRoleBinding(namespace, roleRef, subject)
	}

	return key,
		metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(objMeta)},
		},
		[]rbacv1.Subject{subject},
		roleRef
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
	} else if prtb.GroupPrincipalName != "" {
		name = prtb.GroupPrincipalName
	} else if prtb.ServiceAccount != "" {
		name = prtb.ServiceAccount
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

func prtbByUID(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{convert.ToString(prtb.UID)}, nil
}

func prtbByNsName(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{pkgrbac.GetRTBLabel(prtb.ObjectMeta)}, nil
}

func crbRoleSubjectKeys(roleName string, subjects []rbacv1.Subject) []string {
	var keys []string
	for _, s := range subjects {
		keys = append(keys, rbRoleSubjectKey(roleName, s))
	}
	return keys
}

func rbRoleSubjectKey(roleName string, subject rbacv1.Subject) string {
	return subject.Kind + " " + subject.Name + " Role " + roleName
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

func rtByInterhitedRTs(obj interface{}) ([]string, error) {
	rt, ok := obj.(*wranglerv3.RoleTemplate)
	if !ok {
		return nil, fmt.Errorf("failed to convert object to *RoleTemplate in indexer [%s]", rtByInheritedRTsIndex)
	}
	return rt.RoleTemplateNames, nil
}

func rtbByClusterAndUserNotDeleting(obj interface{}) ([]string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return []string{}, err
	}
	if meta.GetDeletionTimestamp() != nil {
		return []string{}, nil
	}
	var idx string
	switch rtb := obj.(type) {
	case *v3.ProjectRoleTemplateBinding:
		if rtb.UserName != "" && rtb.ProjectName != "" {
			parts := strings.SplitN(rtb.ProjectName, ":", 2)
			if len(parts) == 2 {
				idx = parts[0] + "-" + rtb.UserName
			}
		}
	case *v3.ClusterRoleTemplateBinding:
		if rtb.UserName != "" && rtb.ClusterName != "" {
			idx = rtb.ClusterName + "-" + rtb.UserName
		}
	}

	if idx == "" {
		return []string{}, nil
	}
	return []string{idx}, nil
}
