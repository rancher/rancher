package rbac

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/apis/management.cattle.io"
	apisV3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/resourcequota"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/rbac"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	projectNSGetClusterRoleNameFmt = "%v-namespaces-%v"
	projectNSAnn                   = "authz.cluster.auth.io/project-namespaces"
	initialRoleCondition           = "InitialRolesPopulated"
	manageNSVerb                   = "manage-namespaces"
	projectNSEditVerb              = "*"

	// compatibility with previous norman lifecycle implementation, now implemented inside OnChange's handler
	normanLifecycleAnnotation = "lifecycle.cattle.io/create.namespace-auth"
	normanLifecycleFinalizer  = "controller.cattle.io/namespace-auth"
)

var projectNSVerbToSuffix = map[string]string{
	"get":             "readonly",
	projectNSEditVerb: "edit",
}
var defaultProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/default-project": "true"})
var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})
var initialProjectToLabels = map[string]labels.Set{
	project.Default: defaultProjectLabels,
	project.System:  systemProjectLabels,
}

func newNamespaceLifecycle(m *manager, sync *resourcequota.SyncController) *nsLifecycle {
	return &nsLifecycle{m: m, rq: sync}
}

type nsLifecycle struct {
	m  *manager
	rq *resourcequota.SyncController
}

// onChange implements the same functionality as the previous norman-based nsLifecycle:
// - First ever reconciliation triggers onCreate, after which an annotation is added to mark this event.
// - Following reconciliations observe this annotation and run a regular update instead
// - A finalizer is also used to block deletion and trigger cleanup,
// The original annotation and finalizer keys from norman are used to preserve backwards compatibility
func (n *nsLifecycle) onChange(_ string, obj *v1.Namespace) (*v1.Namespace, error) {
	if obj == nil {
		return nil, nil
	}

	if obj.DeletionTimestamp != nil {
		if !slices.Contains(obj.GetFinalizers(), normanLifecycleFinalizer) {
			// already finalized
			return obj, nil
		}
		return n.onRemove(obj)
	}

	if obj.Annotations[normanLifecycleAnnotation] != "true" {
		return n.onCreate(obj)
	}

	_, err := n.syncNS(obj)
	return obj, err
}

func (n *nsLifecycle) removeFinalizer(obj *v1.Namespace) (*v1.Namespace, error) {
	if obj == nil {
		return nil, nil
	}
	if x := slices.Index(obj.GetFinalizers(), normanLifecycleFinalizer); x >= 0 {
		obj = obj.DeepCopy()
		obj.Finalizers = slices.Delete(obj.Finalizers, x, x+1)
		return n.m.namespaces.Update(obj)
	}
	return obj, nil
}

func (n *nsLifecycle) onCreate(obj *v1.Namespace) (*v1.Namespace, error) {
	obj, err := n.resourceQuotaInit(obj)
	if err != nil {
		return nil, err
	}

	hasPRTBs, err := n.syncNS(obj)
	if err != nil {
		return nil, err
	}

	obj = obj.DeepCopy()
	if err := n.assignToInitialProject(obj); err != nil {
		return nil, err
	}

	// mark as initialized on success
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[normanLifecycleAnnotation] = "true"
	obj, err = n.m.namespaces.Update(obj)
	if err != nil {
		return nil, err
	}

	go updateStatusAnnotation(hasPRTBs, obj.DeepCopy(), n.m)

	return obj, nil
}

func (n *nsLifecycle) resourceQuotaInit(obj *v1.Namespace) (*v1.Namespace, error) {
	return n.rq.CreateResourceQuota(obj)
}

func (n *nsLifecycle) onRemove(obj *v1.Namespace) (*v1.Namespace, error) {
	n.asyncCleanupRBAC(obj.Name)

	obj, err := n.removeFinalizer(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to remove finalizer: %v", err)
	}
	return obj, nil
}

func (n *nsLifecycle) syncNS(obj *v1.Namespace) (bool, error) {
	// add fleet namespace to system project
	if IsFleetNamespace(obj) &&
		// If this is the local cluster, then only move the namespace to ths system project if the projectIDAnnotation is
		// empty or beings with "local" (i.e. not c-). If the projectIDAnnotation begins with something other than "local"
		// then it is likely that local cluster is the tenant cluster in a hosted Rancher setup and the namespace belongs to
		// the system project for the cluster in the host cluster. Moving it here would only cause the namespace to be
		// continually moved between projects forever.
		(n.m.clusterName != "local" || obj.Annotations[projectIDAnnotation] == "" || strings.HasPrefix(obj.Annotations[projectIDAnnotation], "local")) {

		systemProjectName, err := n.GetSystemProjectName()
		if err != nil {
			return false, errors.Wrapf(err, "failed to add namespace %s to system project", obj.Name)
		}

		// When there is no system project, we should not set this annotation as a result because the project name
		// is empty. If the annotation already exists, and there is no system project, then we need to delete the
		// annotation.
		if systemProjectName != "" {
			obj.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:%v", n.m.clusterName, systemProjectName)
		} else {
			delete(obj.Annotations, projectIDAnnotation)
		}
	}

	hasPRTBs, err := n.ensurePRTBAddToNamespace(obj)
	if err != nil {
		return false, fmt.Errorf("ensuring PRTBs are added to namespace %s: %w", obj.Name, err)
	}

	if err := n.reconcileNamespaceProjectClusterRole(obj); err != nil {
		return false, fmt.Errorf("reconciling namespace %s project cluster roles: %w", obj.Name, err)
	}

	return hasPRTBs, nil
}

func (n *nsLifecycle) assignToInitialProject(ns *v1.Namespace) error {
	if ns.Annotations[projectIDAnnotation] != "" {
		return nil
	}

	initialProjectsToNamespaces, err := getDefaultAndSystemProjectsToNamespaces()
	if err != nil {
		return fmt.Errorf("assigning namespace %s to initial projects: %w", ns.Name, err)
	}
	for projectName, namespaces := range initialProjectsToNamespaces {
		for _, nsToCheck := range namespaces {
			if nsToCheck == ns.Name {
				projects, err := n.m.projectLister.List(n.m.clusterName, initialProjectToLabels[projectName].AsSelector())
				if err != nil {
					return fmt.Errorf("listing projects for cluster %s: %w", n.m.clusterName, err)
				}
				if len(projects) == 0 {
					continue
				}
				if len(projects) > 1 {
					return fmt.Errorf("cluster [%s] contains more than 1 [%s] project", n.m.clusterName, projectName)
				}
				if projects[0] == nil {
					continue
				}
				if ns.Annotations == nil {
					ns.Annotations = map[string]string{}
				}
				ns.Annotations[projectIDAnnotation] = fmt.Sprintf("%v:%v", n.m.clusterName, projects[0].Name)
			}
		}
	}

	return nil
}

func (n *nsLifecycle) GetSystemProjectName() (string, error) {
	projects, err := n.m.projectLister.List(n.m.clusterName, initialProjectToLabels[project.System].AsSelector())
	if err != nil {
		return "", fmt.Errorf("getting system project name for cluster %s: %w", n.m.clusterName, err)
	}
	if len(projects) == 0 {
		return "", nil
	}
	if len(projects) > 1 {
		return "", fmt.Errorf("cluster [%s] contains more than 1 [%s] project", n.m.clusterName, project.System)
	}
	if projects[0] == nil {
		return "", nil
	}
	return projects[0].Name, nil
}

func IsFleetNamespace(ns *v1.Namespace) bool {
	return ns.Name == fleetconst.ClustersLocalNamespace || ns.Name == fleetconst.ClustersDefaultNamespace || ns.Name == fleetconst.ReleaseClustersNamespace || ns.Labels["fleet.cattle.io/managed"] == "true"
}

// ensurePRTBAddToNamespace reconciles the per-namespace PRTB RoleBindings for the project the
// namespace currently belongs to. Under the legacy RBAC model it creates the bindings that should
// exist; in all cases it removes bindings that should not (e.g. left behind when the namespace was
// moved between projects, or when it belongs to no project at all). It returns whether the
// namespace's project has any PRTBs, which the caller uses to gate the initial-roles status.
func (n *nsLifecycle) ensurePRTBAddToNamespace(ns *v1.Namespace) (bool, error) {
	projectID := ns.Annotations[projectIDAnnotation]

	var prtbs []any
	if projectID != "" {
		var err error
		prtbs, err = n.m.prtbIndexer.ByIndex(prtbByProjectIndex, projectID)
		if err != nil {
			return false, errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
		}
	}
	hasPRTBs := len(prtbs) > 0

	// Under the legacy RBAC model the per-namespace PRTB RoleBindings are created here. Under
	// aggregation they are owned by the roletemplate-aggregation controllers, which create them with
	// aggregation labels and remove them when the PRTB is deleted; creating the legacy bindings here
	// would leak RoleBindings the aggregation removal handler does not clean up.
	if projectID != "" && !features.AggregatedRoleTemplates.Enabled() {
		if err := n.createLegacyProjectRoleBindings(ns.Name, prtbs); err != nil {
			return false, err
		}
	}

	// Remove any PRTB RoleBinding (legacy or aggregation) that doesn't belong to the namespace's
	// current project - e.g. bindings left behind when the namespace was moved between projects, or
	// when it belongs to no project at all.
	if err := n.removePRTBRoleBindingsNotInProject(ns.Name, projectID, prtbs); err != nil {
		return hasPRTBs, err
	}

	return hasPRTBs, nil
}

// createLegacyProjectRoleBindings ensures a RoleBinding exists in the namespace for each PRTB in the
// namespace's project. Only used under the legacy RBAC model; under aggregation these bindings are
// owned by the roletemplate-aggregation controllers.
func (n *nsLifecycle) createLegacyProjectRoleBindings(nsName string, prtbs []any) error {
	for _, obj := range prtbs {
		prtb, ok := obj.(*apisV3.ProjectRoleTemplateBinding)
		if !ok {
			return fmt.Errorf("expected *v3.ProjectRoleTemplateBinding, got %T", obj)
		}

		if prtb.UserName == "" && prtb.GroupPrincipalName == "" && prtb.GroupName == "" {
			continue
		}

		if prtb.RoleTemplateName == "" {
			logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", prtb.Name)
			continue
		}

		rt, err := n.m.rtLister.Get(prtb.RoleTemplateName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Warnf("ProjectRoleTemplateBinding %q sets a non-existing role template %q. Skipping.", prtb.Name, prtb.RoleTemplateName)
				continue
			}
			return err
		}

		roles := map[string]*apisV3.RoleTemplate{}
		if err := n.m.gatherRoles(rt, roles, 0); err != nil {
			return err
		}

		if err := n.m.ensureRoles(roles); err != nil {
			return errors.Wrap(err, "couldn't ensure roles")
		}

		if err := n.m.ensureProjectRoleBindings(nsName, roles, prtb); err != nil {
			return errors.Wrapf(err, "couldn't ensure binding %v in %v", prtb.Name, nsName)
		}
	}
	return nil
}

// legacyOwnerIndexes maps each legacy rtb-owner RoleBinding label to the PRTB indexer that resolves
// its value to the owning PRTB: the pre-2.5 label carries the PRTB UID, the 2.5+ label carries the
// PRTB's namespace_name key. Only the legacy cleanup branch consults these; the whole map and its
// use in prtbOwnerInCurrentProject can be removed with the legacy RBAC model.
var legacyOwnerIndexes = map[string]string{
	rtbOwnerLabelLegacy: prtbByUIDIndex,
	rtbOwnerLabel:       prtbByNsAndNameIndex,
}

// removePRTBRoleBindingsNotInProject removes every PRTB-owned RoleBinding in the namespace whose
// owning PRTB does not belong to the namespace's current project, handling legacy (rtb-owner
// labelled) and aggregation (prtb-owner labelled) bindings in a single pass. Bindings left behind
// when a namespace moves between projects are the main target; when the namespace belongs to no
// project, no owner is valid so every PRTB-owned binding is removed.
//
// This closes the gap where moving a namespace between projects leaves a user with access to it: the
// aggregation handler reconciles bindings by iterating a project's current namespaces, so once a
// namespace leaves the project the owning PRTB's reconcile can no longer reach it to clean up.
func (n *nsLifecycle) removePRTBRoleBindingsNotInProject(nsName, projectID string, prtbs []any) error {
	var backingNamespace string
	if projectID != "" {
		clusterID, projectID, found := strings.Cut(projectID, ":")
		if !found {
			return nil
		}
		project, err := n.rq.ProjectCache.Get(clusterID, projectID)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Warnf("Namespace %s references project %s in namespace %s which does not exist", nsName, projectID, clusterID)
				return nil
			}
			return err
		}
		backingNamespace = project.GetProjectBackingNamespace()
	}

	// allowedOwnerLabels is the set of prtb-owner label keys valid for the current project, against
	// which aggregation bindings are matched. Empty when the namespace belongs to no project.
	allowedOwnerLabels := make(map[string]bool, len(prtbs))
	for _, obj := range prtbs {
		if prtb, ok := obj.(*apisV3.ProjectRoleTemplateBinding); ok {
			allowedOwnerLabels[pkgrbac.GetPRTBOwnerLabel(prtb.Name)] = true
		}
	}

	rbs, err := n.m.rbLister.List(nsName, labels.Everything())
	if err != nil {
		return errors.Wrapf(err, "couldn't list role bindings in %s", nsName)
	}
	for _, rb := range rbs {
		owned, inProject, err := n.prtbOwnerInCurrentProject(rb, backingNamespace, allowedOwnerLabels)
		if err != nil {
			return err
		}
		// Only touch PRTB-owned bindings; leave CRTB-owned (or unrelated) bindings alone.
		if owned && !inProject {
			if err := rbac.DeleteNamespacedResource(nsName, rb.Name, n.m.roleBindings); err != nil {
				return err
			}
		}
	}
	return nil
}

// prtbOwnerInCurrentProject reports whether rb is owned by a PRTB and if so, whether that
// PRTB belongs to the namespace's current project.
//
// Aggregation bindings encode their owning PRTB in a prtb-owner-<name> label key, matched against
// allowedOwnerLabels (the owner labels of the current project's PRTBs). Legacy bindings reference
// their owning PRTB by the rtb-owner label value, which is resolved through the PRTB indexer and
// matched against the current project's backing namespace. The legacy branch can be dropped once the
// legacy RBAC model is removed.
func (n *nsLifecycle) prtbOwnerInCurrentProject(rb *rbacv1.RoleBinding, backingNamespace string, allowedOwnerLabels map[string]bool) (owned, inProject bool, err error) {
	for key := range rb.Labels {
		if strings.HasPrefix(key, pkgrbac.PrtbOwnerLabel+"-") {
			owned = true
			if allowedOwnerLabels[key] {
				inProject = true
			}
		}
	}

	for label, index := range legacyOwnerIndexes {
		value := convert.ToString(rb.Labels[label])
		if value == "" {
			continue
		}
		owned = true
		prtbs, lookupErr := n.m.prtbIndexer.ByIndex(index, value)
		if lookupErr != nil {
			return owned, inProject, errors.Wrapf(lookupErr, "couldn't find prtb for %s", rb.Name)
		}
		for _, obj := range prtbs {
			if prtb, ok := obj.(*apisV3.ProjectRoleTemplateBinding); ok && prtb.Namespace == backingNamespace {
				inProject = true
			}
		}
	}

	return owned, inProject, nil
}

// To ensure that all users in a project can do a GET on the namespaces in that project, this
// function ensures that a ClusterRole exists for the project that grants get access to the
// namespaces in the project. A corresponding PRTB handler will ensure that a binding to this
// ClusterRole exists for every project member
func (n *nsLifecycle) reconcileNamespaceProjectClusterRole(ns *v1.Namespace) error {
	for verb, name := range projectNSVerbToSuffix {
		var desiredRole string
		var projectName string
		if ns.DeletionTimestamp == nil {
			if parts := strings.SplitN(ns.Annotations[projectIDAnnotation], ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
				projectName = parts[1]
				desiredRole = fmt.Sprintf(projectNSGetClusterRoleNameFmt, parts[1], name)
			}
		}

		clusterRoles, err := n.m.crIndexer.ByIndex(crByNSIndex, ns.Name)
		if err != nil {
			return err
		}

		roleCli := n.m.clusterRoles
		nsInDesiredRole := false
		for _, c := range clusterRoles {
			cr, ok := c.(*rbacv1.ClusterRole)
			if !ok {
				return errors.Errorf("%v is not a ClusterRole", c)
			}

			if cr.Name == desiredRole {
				nsInDesiredRole = true
				continue
			}

			// This ClusterRole has a reference to the namespace, but is not the desired role. Namespace has been moved; remove it from this ClusterRole
			undesiredRole := cr.DeepCopy()
			modified := false
			for i := range undesiredRole.Rules {
				r := &undesiredRole.Rules[i]
				if slice.ContainsString(r.Verbs, verb) && slice.ContainsString(r.Resources, "namespaces") && slice.ContainsString(r.ResourceNames, ns.Name) {
					modified = true
					resNames := r.ResourceNames
					for i := len(resNames) - 1; i >= 0; i-- {
						if resNames[i] == ns.Name {
							resNames = append(resNames[:i], resNames[i+1:]...)
						}
					}
					r.ResourceNames = resNames
				}
			}
			//if ResourceNames is empty, delete the rule and delete the role if no rules exist
			toDeleteRules := 0
			for _, rule := range undesiredRole.Rules {
				if len(rule.ResourceNames) == 0 {
					toDeleteRules++
				}
			}
			if toDeleteRules == len(undesiredRole.Rules) {
				logrus.Infof("Deleting ClusterRole %s", undesiredRole.Name)
				if err = roleCli.Delete(undesiredRole.Name, &metav1.DeleteOptions{}); err != nil {
					return err
				}
				continue
			} else if toDeleteRules != 0 {
				var updatedRules []rbacv1.PolicyRule
				for _, rule := range undesiredRole.Rules {
					if len(rule.ResourceNames) != 0 {
						updatedRules = append(updatedRules, rule)
					}
				}
				undesiredRole.Rules = updatedRules
			}
			if modified {
				if _, err = roleCli.Update(undesiredRole); err != nil {
					return err
				}
			}
		}

		if !nsInDesiredRole && desiredRole != "" {
			mustUpdate := true
			cr, err := n.m.crLister.Get(desiredRole)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			// Create new role
			if cr == nil {
				return n.m.createProjectNSRole(desiredRole, verb, ns.Name, projectName)
			}

			// Check to see if retrieved role has the namespace (small chance cache could have been updated)
			for _, r := range cr.Rules {
				if slice.ContainsString(r.Verbs, verb) && slice.ContainsString(r.Resources, "namespaces") && slice.ContainsString(r.ResourceNames, ns.Name) {
					// ns already in the role, nothing to do
					mustUpdate = false
				}
			}
			if mustUpdate {
				cr = cr.DeepCopy()
				appendedToExisting := false
				for i := range cr.Rules {
					r := &cr.Rules[i]
					if slice.ContainsString(r.Verbs, verb) && slice.ContainsString(r.Resources, "namespaces") {
						r.ResourceNames = append(r.ResourceNames, ns.Name)
						appendedToExisting = true
						break
					}
				}

				if !appendedToExisting {
					cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
						APIGroups:     []string{""},
						Verbs:         []string{verb},
						Resources:     []string{"namespaces"},
						ResourceNames: []string{ns.Name},
					})
				}

				if _, err := roleCli.Update(cr); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (m *manager) createProjectNSRole(roleName, verb, ns, projectName string) error {
	roleCli := m.clusterRoles

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        roleName,
			Annotations: map[string]string{projectNSAnn: roleName},
		},
	}
	if ns != "" {
		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Verbs:         []string{verb},
				Resources:     []string{"namespaces"},
				ResourceNames: []string{ns},
			},
		}
	}
	// the verbs passed into this function come from projectNSVerbToSuffix which only contains two verbs, one for read
	// permissions and one for write. Only the write permission should get the manage-ns verb
	if verb == projectNSEditVerb {
		cr = addManageNSPermission(cr, projectName)
	}
	_, err := roleCli.Create(cr)
	return err
}

func addManageNSPermission(clusterRole *rbacv1.ClusterRole, projectName string) *rbacv1.ClusterRole {
	if clusterRole.Rules == nil {
		clusterRole.Rules = []rbacv1.PolicyRule{}
	}
	clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
		APIGroups:     []string{management.GroupName},
		Verbs:         []string{manageNSVerb},
		Resources:     []string{apisV3.ProjectResourceName},
		ResourceNames: []string{projectName},
	})
	if clusterRole.Annotations == nil {
		clusterRole.Annotations = map[string]string{}
	}
	return clusterRole
}

// addUpdatepsaClusterRole returns a ClusterRole with the updatepsa verb enabled.
// The name of the ClusterRole has the following format: <project_name>-namespaces-psa
func addUpdatepsaClusterRole(projectName string) *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{}
	crName := "%s-namespaces-psa"
	clusterRole.Name = fmt.Sprintf(crName, projectName)
	clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
		APIGroups:     []string{management.GroupName},
		Verbs:         []string{"updatepsa"},
		Resources:     []string{apisV3.ProjectResourceName},
		ResourceNames: []string{projectName},
	})
	if clusterRole.Annotations == nil {
		clusterRole.Annotations = map[string]string{}
	}
	return clusterRole
}

func crByNS(obj interface{}) ([]string, error) {
	cr, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return []string{}, nil
	}

	if _, ok := cr.Annotations[projectNSAnn]; !ok {
		return []string{}, nil
	}

	var result []string
	for _, r := range cr.Rules {
		if slice.ContainsString(r.Resources, "namespaces") && (slice.ContainsString(r.Verbs, "get") || slice.ContainsString(r.Verbs, "*")) {
			result = append(result, r.ResourceNames...)
		}
	}
	return result, nil
}

func updateStatusAnnotation(hasPRTBs bool, namespace *v1.Namespace, mgr *manager) {
	if _, ok := namespace.Annotations[projectIDAnnotation]; ok {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Millisecond * 500)
			clusterRoles, err := mgr.crIndexer.ByIndex(crByNSIndex, namespace.Name)
			if err != nil {
				logrus.Warnf("error getting cluster roles for ns %v for status update: %v", namespace.Name, err)
				continue
			}
			if len(clusterRoles) < 2 {
				continue
			}

			creator := namespace.Annotations["field.cattle.io/creatorId"]
			if creator != "" {
				found := false
				for _, crx := range clusterRoles {
					cr, _ := crx.(*rbacv1.ClusterRole)
					crbKey := rbRoleSubjectKey(cr.Name, rbacv1.Subject{Kind: "User", Name: creator})
					crbs, _ := mgr.crbIndexer.ByIndex(crbByRoleAndSubjectIndex, crbKey)
					if len(crbs) > 0 {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			if hasPRTBs {
				bindings, err := mgr.rbLister.List(namespace.Name, labels.Everything())
				if err != nil {
					logrus.Warnf("error getting bindings for ns %v for status update: %v", namespace.Name, err)
					continue
				}
				if len(bindings) > 0 {
					break
				}
			}
		}
	}

	for i := 0; i < 10; i++ {
		ns, err := mgr.namespaces.Get(namespace.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("error getting ns %v for status update: %v", namespace.Name, err)
			return
		}
		if err := namespaceutil.SetNamespaceCondition(ns, time.Second*1, initialRoleCondition, true, ""); err != nil {
			logrus.Warnf("fail to set %v condition on ns %v: %v", initialRoleCondition, namespace.Name, err)
			continue
		}
		_, err = mgr.namespaces.Update(ns)
		if err == nil {
			break
		}
		if !apierrors.IsConflict(err) {
			logrus.Warnf("error updating ns %v status: %v", ns.Name, err)
		}
	}

}

// asyncCleanupRBAC will wait for a Terminating namespace to be fully deleted before removing the associated RBAC.
func (n *nsLifecycle) asyncCleanupRBAC(namespaceName string) {
	go func() {
		backoff := wait.Backoff{
			Duration: 5 * time.Second,
			Factor:   2.0,
			Jitter:   0.1,
			Steps:    10,
			Cap:      5 * time.Minute,
		}

		err := wait.ExponentialBackoff(backoff, func() (bool, error) {
			_, err := n.m.nsLister.Get(namespaceName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Namespace is fully deleted, clean up RBAC
					err := n.reconcileNamespaceProjectClusterRole(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})
					if err != nil {
						logrus.Errorf("error cleaning up RBAC for namespace %s: %v", namespaceName, err)
						return true, err
					}
					logrus.Debugf("successfully cleaned up RBAC for namespace %s", namespaceName)
					return true, nil
				}
				return false, err
			}

			logrus.Debugf("namespace %s is still present. Will recheck.", namespaceName)
			return false, nil
		})

		if err != nil {
			logrus.Errorf("async cleanup of RBAC for namespace %s failed: %v", namespaceName, err)
		}
	}()
}
