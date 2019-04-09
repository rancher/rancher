package rbac

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/controllers/user/resourcequota"
	namespaceutil "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	projectNSGetClusterRoleNameFmt = "%v-namespaces-%v"
	projectNSAnn                   = "authz.cluster.auth.io/project-namespaces"
	initialRoleCondition           = "InitialRolesPopulated"
)

var projectNSVerbToSuffix = map[string]string{
	"get": "readonly",
	"*":   "edit",
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

func (n *nsLifecycle) Create(obj *v1.Namespace) (runtime.Object, error) {
	obj, err := n.resourceQuotaInit(obj)
	if err != nil {
		return obj, err
	}

	hasPRTBs, err := n.syncNS(obj)
	if err != nil {
		return obj, err
	}

	if err := n.assignToInitialProject(obj); err != nil {
		return obj, err
	}

	go updateStatusAnnotation(hasPRTBs, obj.DeepCopy(), n.m)

	return obj, err
}

func (n *nsLifecycle) resourceQuotaInit(obj *v1.Namespace) (*v1.Namespace, error) {
	ns, err := n.rq.CreateResourceQuota(obj)
	if ns, ok := ns.(*v1.Namespace); ok {
		return ns, err
	}
	return nil, err
}

func (n *nsLifecycle) Updated(obj *v1.Namespace) (runtime.Object, error) {
	_, err := n.syncNS(obj)
	return obj, err
}

func (n *nsLifecycle) Remove(obj *v1.Namespace) (runtime.Object, error) {
	err := n.reconcileNamespaceProjectClusterRole(obj)
	return obj, err
}

func (n *nsLifecycle) syncNS(obj *v1.Namespace) (bool, error) {
	if err := n.cleanRBFromNamespace(obj); err != nil {
		return false, err
	}

	if err := n.ensureInjectedAppRBAddToNamespace(obj); err != nil {
		return false, err
	}

	hasPRTBs, err := n.ensurePRTBAddToNamespace(obj)
	if err != nil {
		return false, err
	}

	if err := n.reconcileNamespaceProjectClusterRole(obj); err != nil {
		return false, err
	}

	return hasPRTBs, nil
}

func (n *nsLifecycle) assignToInitialProject(ns *v1.Namespace) error {
	initialProjectsToNamespaces, err := getDefaultAndSystemProjectsToNamespaces()
	if err != nil {
		return err
	}
	for projectName, namespaces := range initialProjectsToNamespaces {
		for _, nsToCheck := range namespaces {
			if nsToCheck == ns.Name {
				projectID := ns.Annotations[projectIDAnnotation]
				if projectID != "" {
					return nil
				}
				projects, err := n.m.projectLister.List(n.m.clusterName, initialProjectToLabels[projectName].AsSelector())
				if err != nil {
					return err
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

// cleanRBFromNamespace cleans all role bindings when namespace is moved
func (n *nsLifecycle) cleanRBFromNamespace(ns *v1.Namespace) error {
	// Get project that contain this namespace
	projectID := ns.Annotations[projectIDAnnotation]

	// Clean all related role bindings if namespace is moved to none
	if projectID == "" {
		rbs, err := n.m.rbLister.List(ns.Name, labels.Everything())
		if err != nil {
			return errors.Wrapf(err, "couldn't list role bindings in %s", ns.Name)
		}
		client := n.m.workload.RBAC.RoleBindings(ns.Name).ObjectClient()
		for _, rb := range rbs {
			if uid := convert.ToString(rb.Labels[rtbOwnerLabel]); uid != "" {
				logrus.Infof("Deleting role binding %s in %s", rb.Name, ns.Name)
				if err := client.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
					return errors.Wrapf(err, "couldn't delete role binding %s", rb.Name)
				}
			}
		}
		return nil
	}

	// Clean stale role bindings
	var projectName string
	if parts := strings.SplitN(projectID, ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
		projectName = parts[1]
	} else {
		return nil
	}

	rbs, err := n.m.rbLister.List(ns.Name, labels.Everything())
	if err != nil {
		return errors.Wrapf(err, "couldn't list role bindings in %s", ns.Name)
	}
	client := n.m.workload.RBAC.RoleBindings(ns.Name).ObjectClient()

	for _, rb := range rbs {
		if uid := convert.ToString(rb.Labels[rtbOwnerLabel]); uid != "" {
			apps, err := n.m.appIndexer.ByIndex(appByUIDIndex, uid)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "couldn't find apps for %s", rb.Name)
			}
			prtbs, err := n.m.prtbIndexer.ByIndex(prtbByUIDIndex, uid)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "couldn't find prtb for %s", rb.Name)
			}

			var objs []interface{}
			objs = append(objs, apps...)
			objs = append(objs, prtbs...)

			for _, obj := range objs {
				var objNamespace string

				if app, ok := obj.(*projectv3.App); ok {
					if getRoleTemplateName(app) != "" {
						objNamespace = app.Namespace
					}
				} else if prtb, ok := obj.(*v3.ProjectRoleTemplateBinding); ok {
					objNamespace = prtb.Namespace
				}

				if objNamespace != "" && objNamespace != projectName {
					logrus.Infof("Deleting role binding %s in %s", rb.Name, ns.Name)
					if err := client.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
						return errors.Wrapf(err, "couldn't delete role binding %s", rb.Name)
					}
				}
			}
		}
	}

	return nil
}

// ensureInjectedAppRBAddToNamespace ensures the injected apps have permissions in project-owned namespaces
func (n *nsLifecycle) ensureInjectedAppRBAddToNamespace(ns *v1.Namespace) error {
	// Get project that contain this namespace
	projectName := ns.Annotations[projectIDAnnotation]
	if projectName == "" {
		return nil
	}

	projectID := strings.TrimPrefix(projectName, n.m.clusterName+":")

	// Get all apps in the project
	apps, err := n.m.appLister.List(projectID, labels.Everything())
	if err != nil {
		return errors.Wrapf(err, "couldn't get apps associated with project id %s", projectID)
	}

	// Ensure injected apps role bindings
	for _, app := range apps {
		if app.DeletionTimestamp != nil {
			continue
		}

		injectRoleTemplate := getRoleTemplateName(app)
		if injectRoleTemplate == "" {
			continue
		}

		rt, err := n.m.rtLister.Get(metav1.NamespaceAll, injectRoleTemplate)
		if err != nil {
			return errors.Wrapf(err, "couldn't get role template %v", injectRoleTemplate)
		}

		roles := map[string]*v3.RoleTemplate{}
		if err := n.m.gatherRoles(rt, roles); err != nil {
			return err
		}

		if err := n.m.ensureRoles(roles); err != nil {
			return errors.Wrap(err, "couldn't ensure roles")
		}

		if err := n.m.ensureAppServiceAccountRoleBindings(ns.Name, roles, app); err != nil {
			return errors.Wrapf(err, "couldn't ensure app %v in %v", app.Name, ns.Name)
		}
	}

	return nil
}

func (n *nsLifecycle) ensurePRTBAddToNamespace(ns *v1.Namespace) (bool, error) {
	// Get project that contain this namespace
	projectID := ns.Annotations[projectIDAnnotation]
	if len(projectID) == 0 {
		return false, nil
	}

	prtbs, err := n.m.prtbIndexer.ByIndex(prtbByProjectIndex, projectID)
	if err != nil {
		return false, errors.Wrapf(err, "couldn't get project role binding templates associated with project id %s", projectID)
	}
	hasPRTBs := len(prtbs) > 0

	for _, prtb := range prtbs {
		prtb, ok := prtb.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return false, errors.Wrapf(err, "object %v is not valid project role template binding", prtb)
		}

		if prtb.UserName == "" && prtb.GroupPrincipalName == "" && prtb.GroupName == "" {
			continue
		}

		if prtb.RoleTemplateName == "" {
			logrus.Warnf("ProjectRoleTemplateBinding %v has no role template set. Skipping.", prtb.Name)
			continue
		}

		rt, err := n.m.rtLister.Get("", prtb.RoleTemplateName)
		if err != nil {
			return false, errors.Wrapf(err, "couldn't get role template %v", prtb.RoleTemplateName)
		}

		roles := map[string]*v3.RoleTemplate{}
		if err := n.m.gatherRoles(rt, roles); err != nil {
			return false, err
		}

		if err := n.m.ensureRoles(roles); err != nil {
			return false, errors.Wrap(err, "couldn't ensure roles")
		}

		if err := n.m.ensureProjectRoleBindings(ns.Name, roles, prtb); err != nil {
			return false, errors.Wrapf(err, "couldn't ensure binding %v in %v", prtb.Name, ns.Name)
		}
	}

	return hasPRTBs, nil
}

// To ensure that all users in a project can do a GET on the namespaces in that project, this
// function ensures that a ClusterRole exists for the project that grants get access to the
// namespaces in the project. A corresponding PRTB handler will ensure that a binding to this
// ClusterRole exists for every project member
func (n *nsLifecycle) reconcileNamespaceProjectClusterRole(ns *v1.Namespace) error {
	for verb, name := range projectNSVerbToSuffix {
		var desiredRole string
		if ns.DeletionTimestamp == nil {
			if parts := strings.SplitN(ns.Annotations[projectIDAnnotation], ":", 2); len(parts) == 2 && len(parts[1]) > 0 {
				desiredRole = fmt.Sprintf(projectNSGetClusterRoleNameFmt, parts[1], name)
			}
		}

		clusterRoles, err := n.m.crIndexer.ByIndex(crByNSIndex, ns.Name)
		if err != nil {
			return err
		}

		roleCli := n.m.workload.RBAC.ClusterRoles("")
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
			cr, err := n.m.crLister.Get("", desiredRole)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			// Create new role
			if cr == nil {
				return n.m.createProjectNSRole(desiredRole, verb, ns.Name)
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

				_, err = roleCli.Update(cr)
				return err
			}
		}
	}

	return nil
}

func (m *manager) createProjectNSRole(roleName, verb, ns string) error {
	roleCli := m.workload.RBAC.ClusterRoles("")

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
	_, err := roleCli.Create(cr)
	return err
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
		ns, err := mgr.workload.Core.Namespaces("").Get(namespace.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("error getting ns %v for status update: %v", namespace.Name, err)
			return
		}
		if err := namespaceutil.SetNamespaceCondition(ns, time.Second*1, initialRoleCondition, true, ""); err != nil {
			logrus.Warnf("fail to set %v condition on ns %v: %v", initialRoleCondition, namespace.Name, err)
			continue
		}
		_, err = mgr.workload.Core.Namespaces("").Update(ns)
		if err == nil {
			break
		}
		logrus.Warnf("error updating ns %v status: %v", ns.Name, err)
	}

}

// getRoleTemplateName returns the injected status of App and which role template to inject
func getRoleTemplateName(app *projectv3.App) (injectRoleTemplate string) {
	value, ok := app.Annotations["field.cattle.io/injectAccount"]
	if !ok {
		return ""
	}

	if value != "project-monitoring-view" {
		return ""
	}

	return value
}
