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
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	projectNSGetClusterRoleNameFmt = "%v-namespaces-%v"
	projectNSAnn                   = "authz.cluster.auth.io/project-namespaces"
	initialRoleCondition           = "InitialRolesPopulated"
	manageNSVerb                   = "manage-namespaces"
	projectNSEditVerb              = "*"

	// initialRolesRequeueInterval is the backstop delay for re-checking whether a namespace's
	// project RBAC has materialized, used when no watch wakes the reconcile sooner.
	initialRolesRequeueInterval = 5 * time.Second

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

	var err error
	if obj.Annotations[normanLifecycleAnnotation] != "true" {
		obj, err = n.onCreate(obj)
	} else {
		err = n.syncNS(obj)
	}
	if err != nil {
		return obj, err
	}

	// Level-triggered: reconcile the InitialRolesPopulated condition every pass, based on the
	// namespace's final project assignment. The RB/CR enqueuers re-trigger this handler when the
	// awaited RBAC appears; until then reconcileInitialRolesCondition requeues via EnqueueAfter.
	return n.reconcileInitialRolesCondition(obj)
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

	if err := n.syncNS(obj); err != nil {
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

func (n *nsLifecycle) syncNS(obj *v1.Namespace) error {
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
			return errors.Wrapf(err, "failed to add namespace %s to system project", obj.Name)
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

	if err := n.ensurePRTBAddToNamespace(obj); err != nil {
		return fmt.Errorf("ensuring PRTBs are added to namespace %s: %w", obj.Name, err)
	}

	if err := n.reconcileNamespaceProjectClusterRole(obj); err != nil {
		return fmt.Errorf("reconciling namespace %s project cluster roles: %w", obj.Name, err)
	}

	return nil
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
// moved between projects, or when it belongs to no project at all).
func (n *nsLifecycle) ensurePRTBAddToNamespace(ns *v1.Namespace) error {
	projectID := ns.Annotations[projectIDAnnotation]

	var prtbs []any
	if projectID != "" {
		var err error
		prtbs, err = n.m.prtbIndexer.ByIndex(prtbByProjectIndex, projectID)
		if err != nil {
			return errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
		}
	}

	// Under the legacy RBAC model the per-namespace PRTB RoleBindings are created here. Under
	// aggregation they are owned by the roletemplate-aggregation controllers, which create them with
	// aggregation labels and remove them when the PRTB is deleted.
	if projectID != "" && !features.AggregatedRoleTemplates.Enabled() {
		if err := n.createLegacyProjectRoleBindings(ns.Name, prtbs); err != nil {
			return err
		}
	}

	// Remove any PRTB RoleBinding (legacy or aggregation) that doesn't belong to the namespace's
	// current project - e.g. bindings left behind when the namespace was moved between projects, or
	// when it belongs to no project at all.
	if err := n.removePRTBRoleBindingsNotInProject(ns.Name, projectID, prtbs); err != nil {
		return err
	}

	return nil
}

// createLegacyProjectRoleBindings ensures a RoleBinding exists in the namespace for each PRTB in the
// namespace's project. Only used under the non-aggregation RBAC model.
func (n *nsLifecycle) createLegacyProjectRoleBindings(nsName string, prtbs []any) error {
	for _, obj := range prtbs {
		prtb, ok := obj.(*apisV3.ProjectRoleTemplateBinding)
		if !ok {
			return fmt.Errorf("expected *apisv3.ProjectRoleTemplateBinding, got %T", obj)
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
// PRTB's namespace_name key.
var legacyOwnerIndexes = map[string]string{
	rtbOwnerLabelLegacy: prtbByUIDIndex,
	rtbOwnerLabel:       prtbByNsAndNameIndex,
}

// removePRTBRoleBindingsNotInProject removes every PRTB-owned RoleBinding in the namespace whose
// owning PRTB does not belong to the namespace's current project. Bindings left behind
// when a namespace moves between projects are the main target; when the namespace belongs to no
// project, no owner is valid so every PRTB-owned binding is removed.
func (n *nsLifecycle) removePRTBRoleBindingsNotInProject(nsName, projectLabel string, prtbs []any) error {
	var backingNamespace string
	if projectLabel != "" {
		clusterID, projectID, found := strings.Cut(projectLabel, ":")
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
			allowedOwnerLabels[rbac.GetPRTBOwnerLabel(prtb.Name)] = true
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
func (n *nsLifecycle) prtbOwnerInCurrentProject(rb *rbacv1.RoleBinding, backingNamespace string, allowedOwnerLabels map[string]bool) (owned, inProject bool, err error) {
	owned = isPRTBOwnedRoleBinding(rb)

	for key := range rb.Labels {
		if allowedOwnerLabels[key] {
			inProject = true
			break
		}
	}

	// empty legacy values naturally miss the index lookup, so no special-casing needed here.
	for label, index := range legacyOwnerIndexes {
		raw, ok := rb.Labels[label]
		if !ok {
			continue
		}
		value := convert.ToString(raw)
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
			var found bool
			_, projectName, found = strings.Cut(ns.Annotations[projectIDAnnotation], ":")
			if found && projectName != "" {
				desiredRole = fmt.Sprintf(projectNSGetClusterRoleNameFmt, projectName, name)
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

func crByNS(obj any) ([]string, error) {
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

// reconcileInitialRolesCondition sets the InitialRolesPopulated condition once the namespace's
// project RBAC has been created. Until then, it requeues the namespace via EnqueueAfter.
// The condition is write-once: after it is set, this returns early on every subsequent reconcile.
func (n *nsLifecycle) reconcileInitialRolesCondition(ns *v1.Namespace) (*v1.Namespace, error) {
	set, err := namespaceutil.IsNamespaceConditionSet(ns, initialRoleCondition, true)
	if err != nil {
		return ns, fmt.Errorf("checking %s condition on namespace %s: %w", initialRoleCondition, ns.Name, err)
	}
	if set {
		return ns, nil
	}

	ready, err := n.initialRolesReady(ns)
	if err != nil {
		return ns, err
	}
	if !ready {
		// The RB/CR enqueuers usually wake us sooner when the awaited objects appear; this is the
		// backstop for transitions no watch covers (e.g. a just-assigned project, dropped events).
		n.m.namespaces.EnqueueAfter(ns.Name, initialRolesRequeueInterval)
		return ns, nil
	}

	ns = ns.DeepCopy()
	if err := namespaceutil.SetNamespaceCondition(ns, time.Second, initialRoleCondition, true, ""); err != nil {
		return ns, fmt.Errorf("setting %s condition on namespace %s: %w", initialRoleCondition, ns.Name, err)
	}
	return n.m.namespaces.Update(ns)
}

// initialRolesReady reports whether the RBAC that InitialRolesPopulated gates has materialized: the
// project's namespace ClusterRoles exist, the namespace creator (if any) has a binding to one of
// them, and - when the project has PRTBs - at least one project RoleBinding exists in the namespace.
// A namespace that belongs to no project has no per-project roles to wait for and is always ready.
func (n *nsLifecycle) initialRolesReady(ns *v1.Namespace) (bool, error) {
	projectID := ns.Annotations[projectIDAnnotation]
	if projectID == "" {
		return true, nil
	}

	clusterRoles, err := n.m.crIndexer.ByIndex(crByNSIndex, ns.Name)
	if err != nil {
		return false, fmt.Errorf("getting cluster roles for namespace %s: %w", ns.Name, err)
	}
	if len(clusterRoles) < 2 {
		return false, nil
	}

	if creator := ns.Annotations["field.cattle.io/creatorId"]; creator != "" {
		found := false
		for _, crx := range clusterRoles {
			cr, ok := crx.(*rbacv1.ClusterRole)
			if !ok {
				continue
			}
			crbKey := rbRoleSubjectKey(cr.Name, rbacv1.Subject{Kind: "User", Name: creator})
			crbs, err := n.m.crbIndexer.ByIndex(crbByRoleAndSubjectIndex, crbKey)
			if err != nil {
				return false, fmt.Errorf("getting cluster role bindings for namespace %s: %w", ns.Name, err)
			}
			if len(crbs) > 0 {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	prtbs, err := n.m.prtbIndexer.ByIndex(prtbByProjectIndex, projectID)
	if err != nil {
		return false, fmt.Errorf("getting PRTBs for project %s: %w", projectID, err)
	}
	if len(prtbs) > 0 {
		bindings, err := n.m.rbLister.List(ns.Name, labels.Everything())
		if err != nil {
			return false, fmt.Errorf("getting role bindings for namespace %s: %w", ns.Name, err)
		}

		found := slices.ContainsFunc(bindings, func(rb *rbacv1.RoleBinding) bool {
			return isPRTBOwnedRoleBinding(rb)
		})
		if !found {
			return false, nil
		}
	}

	return true, nil
}

// roleBindingEnqueueNamespace enqueues the namespace a PRTB-owned RoleBinding lives in, so its
// InitialRolesPopulated condition is re-evaluated as soon as project-member bindings appear.
func roleBindingEnqueueNamespace(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	rb, ok := obj.(*rbacv1.RoleBinding)
	if !ok || rb == nil {
		return nil, nil
	}
	if !isPRTBOwnedRoleBinding(rb) {
		return nil, nil
	}
	return []relatedresource.Key{{Name: rb.Namespace}}, nil
}

// isPRTBOwnedRoleBinding reports whether rb is owned by a ProjectRoleTemplateBinding, under either
// the aggregation model (prtb-owner-<name> label) or the legacy model (rtb-owner labels).
func isPRTBOwnedRoleBinding(rb *rbacv1.RoleBinding) bool {
	for key := range rb.Labels {
		if strings.HasPrefix(key, rbac.PrtbOwnerLabel+"-") {
			return true
		}
	}
	for label := range legacyOwnerIndexes {
		if _, ok := rb.Labels[label]; ok {
			return true
		}
	}
	return false
}

// clusterRoleEnqueueNamespace enqueues every namespace a project-namespaces ClusterRole authorizes,
// so InitialRolesPopulated is re-evaluated when those ClusterRoles appear. crByNS already filters to
// the relevant ClusterRoles and extracts the namespaces they reference.
func clusterRoleEnqueueNamespace(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	nsNames, err := crByNS(obj)
	if err != nil {
		return nil, err
	}
	keys := make([]relatedresource.Key, 0, len(nsNames))
	for _, name := range nsNames {
		keys = append(keys, relatedresource.Key{Name: name})
	}
	return keys, nil
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
