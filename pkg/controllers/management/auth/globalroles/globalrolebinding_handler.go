package globalroles

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wcorev1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	wrangler "github.com/rancher/wrangler/v2/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

var (
	globalRoleBindingLabel = map[string]string{"authz.management.cattle.io/globalrolebinding": "true"}
)

const (
	catalogTemplateResourceRule        = "catalogtemplates"
	catalogTemplateVersionResourceRule = "catalogtemplateversions"
	crbNameAnnotation                  = "authz.management.cattle.io/crb-name"
	crtbGrbOwnerIndex                  = "authz.management.cattle.io/crtb-owner"
	crbNamePrefix                      = "cattle-globalrolebinding-"
	GlobalCatalogRole                  = "global-catalog"
	globalCatalogRoleBinding           = "global-catalog-binding"
	TemplateResourceRule               = "templates"
	TemplateVersionResourceRule        = "templateversions"
	localClusterName                   = "local"
	grbOwnerLabel                      = "authz.management.cattle.io/grb-owner"
)

func newGlobalRoleBindingLifecycle(management *config.ManagementContext, clusterManager *clustermanager.Manager) *globalRoleBindingLifecycle {
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crtbGrbOwnerIndex, crtbGrbOwnerIndexer)
	return &globalRoleBindingLifecycle{
		clusters:          management.Management.Clusters(""),
		clusterLister:     management.Management.Clusters("").Controller().Lister(),
		projectLister:     management.Management.Projects("").Controller().Lister(),
		clusterManager:    clusterManager,
		clusterRoles:      management.RBAC.ClusterRoles(""),
		crbClient:         management.RBAC.ClusterRoleBindings(""),
		crbLister:         management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:          management.RBAC.ClusterRoles("").Controller().Lister(),
		crtbClient:        management.Management.ClusterRoleTemplateBindings(""),
		crtbCache:         management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		grLister:          management.Management.GlobalRoles("").Controller().Lister(),
		nsCache:           management.Wrangler.Core.Namespace().Cache(),
		roles:             management.RBAC.Roles(""),
		roleLister:        management.RBAC.Roles("").Controller().Lister(),
		roleBindings:      management.RBAC.RoleBindings(""),
		roleBindingLister: management.RBAC.RoleBindings("").Controller().Lister(),
	}
}

// crtbGrbOwnerIndexer indexes a CRTB to a key identifying the target cluster and owning GRB
func crtbGrbOwnerIndexer(crtb *v3.ClusterRoleTemplateBinding) ([]string, error) {
	// the label, unlike the owner ref, is protected by the webhook, so we use it as a source of truth
	grbOwner, ok := crtb.Labels[grbOwnerLabel]
	if !ok {
		return nil, nil
	}
	return []string{fmt.Sprintf("%s/%s", crtb.ClusterName, grbOwner)}, nil
}

type globalRoleBindingLifecycle struct {
	clusters          v3.ClusterInterface
	clusterLister     v3.ClusterLister
	projectLister     v3.ProjectLister
	clusterManager    *clustermanager.Manager
	clusterRoles      rbacv1.ClusterRoleInterface
	crLister          rbacv1.ClusterRoleLister
	crbClient         rbacv1.ClusterRoleBindingInterface
	crbLister         rbacv1.ClusterRoleBindingLister
	crtbCache         mgmtcontroller.ClusterRoleTemplateBindingCache
	crtbClient        v3.ClusterRoleTemplateBindingInterface
	grLister          v3.GlobalRoleLister
	nsCache           wcorev1.NamespaceCache
	roles             rbacv1.RoleInterface
	roleLister        rbacv1.RoleLister
	roleBindings      rbacv1.RoleBindingInterface
	roleBindingLister rbacv1.RoleBindingLister
}

func (grb *globalRoleBindingLifecycle) Create(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	var returnError error
	err := grb.reconcileClusterPermissions(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	err = grb.reconcileGlobalRoleBinding(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	err = grb.reconcileNamespacedRoleBindings(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	return obj, returnError
}

func (grb *globalRoleBindingLifecycle) Updated(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	var returnError error
	err := grb.reconcileClusterPermissions(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	err = grb.reconcileGlobalRoleBinding(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	err = grb.reconcileNamespacedRoleBindings(obj)
	if err != nil {
		returnError = errors.Join(returnError, err)
	}
	return obj, returnError
}

func (grb *globalRoleBindingLifecycle) Remove(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if obj.GlobalRoleName == rbac.GlobalAdmin || obj.GlobalRoleName == rbac.GlobalRestrictedAdmin {
		return obj, grb.deleteAdminBinding(obj)
	}
	// Don't need to delete the created ClusterRole or RoleBindings because owner reference will take care of them
	return obj, nil
}

func (grb *globalRoleBindingLifecycle) deleteAdminBinding(obj *v3.GlobalRoleBinding) error {
	// Explicit API call to ensure we have the most recent cluster info when deleting admin bindings
	clusters, err := grb.clusters.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Collect all the errors to delete as many user context bindings as possible
	var allErrors []error

	for _, cluster := range clusters.Items {
		userContext, err := grb.clusterManager.UserContext(cluster.Name)
		if err != nil {
			// ClusterUnavailable error indicates the record can't talk to the downstream cluster
			if !clustermanager.IsClusterUnavailableErr(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}

		bindingName := rbac.GrbCRBName(obj)
		b, err := userContext.RBAC.ClusterRoleBindings("").Controller().Lister().Get("", bindingName)
		if err != nil {
			// User context clusterRoleBinding doesn't exist
			if !apierrors.IsNotFound(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}

		err = userContext.RBAC.ClusterRoleBindings("").Delete(b.Name, &metav1.DeleteOptions{})
		if err != nil {
			// User context clusterRoleBinding doesn't exist
			if !apierrors.IsNotFound(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}

	}

	if len(allErrors) > 0 {
		return fmt.Errorf("errors deleting admin global role binding: %v", allErrors)
	}
	return nil
}

// reconcileClusterPermissions grants permissions for the binding in all downstream (non-local) clusters. Will also
// remove invalid bindings (bindings not for active RoleTemplates or for invalid subjects).
func (grb *globalRoleBindingLifecycle) reconcileClusterPermissions(globalRoleBinding *v3.GlobalRoleBinding) error {
	globalRole, err := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	if err != nil {
		return fmt.Errorf("unable to get globalRole %s: %w", globalRoleBinding.Name, err)
	}
	clusters, err := grb.clusterLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("unable to list clusters when reconciling globalRoleBinding %s: %w", globalRoleBinding.Name, err)
	}

	var missedClusters bool
	for _, cluster := range clusters {
		// we don't sync permissions for the local cluster, but we do want to purge user-created permissions
		if cluster.Name == localClusterName {
			err := grb.purgeCorruptRoles(nil, cluster, globalRoleBinding)
			if err != nil {
				// failure to remove bad bindings shouldn't affect our ability to sync new permissions, so we log and keep processing
				logrus.Errorf("unable to purge roles for cluster %s and grb %s, some bindings may remain: %s", cluster.Name, globalRoleBinding.Name, err.Error())
				missedClusters = true
			}
			// inheritedClusterRoles only apply on non-local clusters, so skip the local cluster
			continue
		}
		err := grb.purgeCorruptRoles(globalRole.InheritedClusterRoles, cluster, globalRoleBinding)
		if err != nil {
			// failure to remove bad bindings shouldn't affect our ability to sync new permissions, so we log and keep processing
			logrus.Errorf("unable to purge roles for cluster %s and grb %s, some bindings may remain: %s", cluster.Name, globalRoleBinding.Name, err.Error())
			missedClusters = true
		}
		missingRTs, err := grb.findMissingRTs(globalRole.InheritedClusterRoles, cluster, globalRoleBinding)
		if err != nil {
			logrus.Errorf("unable to find missing roles for cluster %s and grb %s, some permissions may be missing: %s", cluster.Name, globalRoleBinding.Name, err.Error())
			missedClusters = true
			continue
		}
		// at this point, the only remaining items are roleTemplates that we don't have a CRTB for in this cluster
		for _, wantRT := range missingRTs {
			// create a crtb in the backing namespace for the cluster
			logrus.Infof("creating backing crtb for grb %s in cluster %s for roleTemplate %s", globalRoleBinding.Name, cluster.Name, wantRT)
			_, err = grb.crtbClient.Create(&v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "crtb-grb-",
					Namespace:    cluster.Name,
					// the owner ref needs to be mutable by the k8s garbage collector but we need
					// a way to identify what CRTBs are from GRBs unambiguously for validation
					Labels: map[string]string{
						grbOwnerLabel:               globalRoleBinding.Name,
						controllers.K8sManagedByKey: controllers.ManagerValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
							Kind:       v3.GlobalRoleBindingGroupVersionKind.Kind,
							Name:       globalRoleBinding.Name,
							UID:        globalRoleBinding.UID,
						},
					},
				},
				ClusterName:        cluster.Name,
				RoleTemplateName:   wantRT,
				UserName:           globalRoleBinding.UserName,
				GroupPrincipalName: globalRoleBinding.GroupPrincipalName,
			})
			// we don't immediately return so that we can create as many CRTBs as we can
			if err != nil {
				logrus.Errorf("failed to create crtb for globalRoleBinding %s in cluster %s: %s", globalRoleBinding.Name, cluster.Name, err.Error())
				missedClusters = true
			}
		}
	}
	if missedClusters {
		return fmt.Errorf("unable to reconcile backing crtbs for globalRoleBinding %s, some permissions may be missing", globalRoleBinding.Name)
	}
	return nil
}

// purgeCorruptRoles removes any CRTBs which were created for this role in the past, but are no longer valid, either
// because they aren't for a currently requested RoleTemplate, or because they have been corrupted by user intervention.
// Will return an error if a binding can't be deleted
func (grb *globalRoleBindingLifecycle) purgeCorruptRoles(wantRTs []string, cluster *v3.Cluster, binding *v3.GlobalRoleBinding) error {
	currentCRTBs, err := grb.crtbCache.GetByIndex(crtbGrbOwnerIndex, fmt.Sprintf("%s/%s", cluster.Name, binding.Name))
	if err != nil {
		return fmt.Errorf("unable to get CRTBs for cluster %s: %w", cluster.Name, err)
	}
	var deleteErr error
	seenRTs := map[string]struct{}{}
	for _, crtb := range currentCRTBs {
		foundRT := false
		for _, roleTemplate := range wantRTs {
			if roleTemplate == crtb.RoleTemplateName {
				foundRT = true
				break
			}
		}
		_, seen := seenRTs[crtb.RoleTemplateName]
		// if the RT isn't one of the ones that we requested, or is corrupt, or refers to the same RT as a prior
		// valid RT, then we remove it.
		if !foundRT || !isCRTBValid(crtb, cluster, binding) || seen {
			// CRTBs can't update some of these fields, so the safest method is to delete/re-create
			err := grb.crtbClient.DeleteNamespaced(crtb.Namespace, crtb.Name, &metav1.DeleteOptions{})
			if err != nil {
				// failure to delete one crtb does not prevent our ability to delete other crtbs, or to determine
				// which rts we want to remove
				crtbErr := fmt.Errorf("unable to delete backing crtb %s for globalRoleBinding %s: %w", crtb.Name, binding.Name, err)
				deleteErr = errors.Join(deleteErr, crtbErr)
			}
		} else {
			seenRTs[crtb.RoleTemplateName] = struct{}{}
		}
	}
	return deleteErr
}

// findMissingRTs finds which RoleTemplates were in wantRTs but don't have a valid binding for this cluster yet
func (grb *globalRoleBindingLifecycle) findMissingRTs(wantRTs []string, cluster *v3.Cluster, binding *v3.GlobalRoleBinding) ([]string, error) {
	currentRTs := map[string]struct{}{}
	for _, wantRT := range wantRTs {
		currentRTs[wantRT] = struct{}{}
	}
	currentCRTBs, err := grb.crtbCache.GetByIndex(crtbGrbOwnerIndex, fmt.Sprintf("%s/%s", cluster.Name, binding.Name))
	if err != nil {
		return nil, fmt.Errorf("unable to get CRTBs for cluster %s: %w", cluster.Name, err)
	}
	for _, crtb := range currentCRTBs {
		_, rtOk := currentRTs[crtb.RoleTemplateName]
		if rtOk && isCRTBValid(crtb, cluster, binding) {
			delete(currentRTs, crtb.RoleTemplateName)
		}
	}
	missingRTs := make([]string, 0, len(currentRTs))
	for missingRT := range currentRTs {
		missingRTs = append(missingRTs, missingRT)
	}
	return missingRTs, nil

}

func (grb *globalRoleBindingLifecycle) reconcileGlobalRoleBinding(globalRoleBinding *v3.GlobalRoleBinding) error {
	crbName, ok := globalRoleBinding.Annotations[crbNameAnnotation]
	if !ok {
		crbName = crbNamePrefix + globalRoleBinding.Name
	}

	subject := rbac.GetGRBSubject(globalRoleBinding)
	if globalRoleBinding.GlobalRoleName == rbac.GlobalRestrictedAdmin {
		if err := grb.syncDownstreamClusterPermissions(subject, globalRoleBinding); err != nil {
			return err
		}
	}

	crb, _ := grb.crbLister.Get("", crbName)
	if crb != nil {
		subjects := []v1.Subject{subject}
		updateSubject := !reflect.DeepEqual(subjects, crb.Subjects)

		updateRoleRef := false
		var roleRef v1.RoleRef
		gr, _ := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
		if gr != nil {
			crNameFromGR := getCRName(gr)
			if crNameFromGR != crb.RoleRef.Name {
				updateRoleRef = true
				roleRef = v1.RoleRef{
					Name: crNameFromGR,
					Kind: clusterRoleKind,
				}
			}
		}
		if updateSubject || updateRoleRef {
			crb = crb.DeepCopy()
			if updateRoleRef {
				crb.RoleRef = roleRef
			}
			crb.Subjects = subjects
			logrus.Infof("[%v] Updating clusterRoleBinding %v for globalRoleBinding %v user %v", grbController, crb.Name, globalRoleBinding.Name, globalRoleBinding.UserName)
			if _, err := grb.crbClient.Update(crb); err != nil {
				return fmt.Errorf("couldn't update ClusterRoleBinding %v: %w", crb.Name, err)
			}
		}
		return grb.addRulesForTemplateAndTemplateVersions(globalRoleBinding, subject)
	}

	logrus.Infof("Creating new GlobalRoleBinding for GlobalRoleBinding %v", globalRoleBinding.Name)
	gr, _ := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	var crName string
	if gr != nil {
		crName = getCRName(gr)
	} else {
		crName = generateCRName(globalRoleBinding.GlobalRoleName)
	}
	logrus.Infof("[%v] Creating clusterRoleBinding for globalRoleBinding %v for user %v with role %v", grbController, globalRoleBinding.Name, globalRoleBinding.UserName, crName)
	_, err := grb.crbClient.Create(&v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: globalRoleBinding.APIVersion,
					Kind:       globalRoleBinding.Kind,
					Name:       globalRoleBinding.Name,
					UID:        globalRoleBinding.UID,
				},
			},
			Labels: globalRoleBindingLabel,
		},
		Subjects: []v1.Subject{subject},
		RoleRef: v1.RoleRef{
			Name: crName,
			Kind: clusterRoleKind,
		},
	})
	if err != nil {
		return err
	}
	// Add an annotation to the globalrole indicating the name we used for future updates
	if globalRoleBinding.Annotations == nil {
		globalRoleBinding.Annotations = map[string]string{}
	}
	globalRoleBinding.Annotations[crbNameAnnotation] = crbName

	return grb.addRulesForTemplateAndTemplateVersions(globalRoleBinding, subject)
}

func (grb *globalRoleBindingLifecycle) addRulesForTemplateAndTemplateVersions(globalRoleBinding *v3.GlobalRoleBinding, subject v1.Subject) error {
	// Check if the current globalRole has rules for templates and templateversions
	gr, err := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	isCatalogRole := false
	if gr != nil {
		for _, rule := range gr.Rules {
			if rbac.RuleGivesResourceAccess(rule, TemplateResourceRule) || rbac.RuleGivesResourceAccess(rule, TemplateVersionResourceRule) {
				isCatalogRole = true
				break
			}
		}
	}
	// Roles that give catalog access (i.e. that give access to template/templateversions in the management api group)
	// are the only ones that need this special role/rolebinding created for them
	if isCatalogRole {
		roleName := gr.Name + "-" + GlobalCatalogRole
		_, err := grb.roleLister.Get(namespace.GlobalNamespace, roleName)
		if err != nil {
			return err
		}
		// create a binding to the namespaced role which corresponds to the GlobalRole this grb refers to
		grbName := globalRoleBinding.Name + "-" + globalCatalogRoleBinding
		_, err = grb.roleBindingLister.Get(namespace.GlobalNamespace, grbName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				rb := &v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      grbName,
						Namespace: namespace.GlobalNamespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: globalRoleBinding.APIVersion,
								Kind:       globalRoleBinding.Kind,
								Name:       globalRoleBinding.Name,
								UID:        globalRoleBinding.UID,
							},
						},
					},
					Subjects: []v1.Subject{subject},
					RoleRef: v1.RoleRef{
						Kind: "Role",
						Name: roleName,
					},
				}
				_, err = grb.roleBindings.Create(rb)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

func (grb *globalRoleBindingLifecycle) syncDownstreamClusterPermissions(subject v1.Subject, globalRoleBinding *v3.GlobalRoleBinding) error {
	if err := grb.createRestrictedAdminCRBsForUserClusters(subject, globalRoleBinding); err != nil {
		return err
	}

	return grb.grantRestrictedAdminUserClusterPermissions(subject, globalRoleBinding)
}

func (grb *globalRoleBindingLifecycle) createRestrictedAdminCRBsForUserClusters(subject v1.Subject, globalRoleBinding *v3.GlobalRoleBinding) error {
	// Get CR for each downstream cluster, create CRB with this subject for each such CR
	r, _ := labels.NewRequirement(rbac.RestrictedAdminCRForClusters, selection.Exists, []string{})
	crs, err := grb.crLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return err
	}

	var returnErr error
	for _, cr := range crs {
		clusterName := cr.Labels[rbac.RestrictedAdminCRForClusters]
		crbName := clusterName + rbac.RestrictedAdminCRBForClusters + globalRoleBinding.Name
		crb, err := grb.crbLister.Get("", crbName)
		if err != nil && !apierrors.IsNotFound(err) {
			returnErr = errors.Join(returnErr, err)
			continue
		}
		if crb != nil {
			continue
		}
		_, err = grb.crbClient.Create(&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            crbName,
				OwnerReferences: cr.OwnerReferences,
			},
			RoleRef: v1.RoleRef{
				Kind: "ClusterRole",
				Name: cr.Name,
			},
			Subjects: []v1.Subject{subject},
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			returnErr = errors.Join(returnErr, err)
		}
	}
	return returnErr
}

func (grb *globalRoleBindingLifecycle) grantRestrictedAdminUserClusterPermissions(subject v1.Subject, globalRoleBinding *v3.GlobalRoleBinding) error {
	var returnErr error
	clusters, err := grb.clusterLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		if cluster.Name == "local" {
			continue
		}
		rbName := fmt.Sprintf("%s-%s", globalRoleBinding.Name, rbac.RestrictedAdminClusterRoleBinding)
		_, err := grb.roleBindingLister.Get(cluster.Name, rbName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				returnErr = errors.Join(returnErr, err)
				continue
			}
			_, err := grb.roleBindings.Create(&v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rbName,
					Namespace: cluster.Name,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: globalRoleBinding.APIVersion,
							Kind:       globalRoleBinding.Kind,
							UID:        globalRoleBinding.UID,
							Name:       globalRoleBinding.Name,
						},
					},
				},
				RoleRef: v1.RoleRef{
					Name: rbac.ClusterCRDsClusterRole,
					Kind: "ClusterRole",
				},
				Subjects: []v1.Subject{subject},
			})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				returnErr = errors.Join(returnErr, err)
				continue
			}
		}

		projects, err := grb.projectLister.List(cluster.Name, labels.NewSelector())
		if err != nil {
			returnErr = errors.Join(returnErr, err)
			continue
		}

		for _, project := range projects {
			rbName := fmt.Sprintf("%s-%s", globalRoleBinding.Name, rbac.RestrictedAdminProjectRoleBinding)
			_, err := grb.roleBindingLister.Get(project.Name, rbName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					_, err := grb.roleBindings.Create(&v1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      rbName,
							Namespace: project.Name,
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: globalRoleBinding.APIVersion,
									Kind:       globalRoleBinding.Kind,
									UID:        globalRoleBinding.UID,
									Name:       globalRoleBinding.Name,
								},
							},
						},
						RoleRef: v1.RoleRef{
							Name: rbac.ProjectCRDsClusterRole,
							Kind: "ClusterRole",
						},
						Subjects: []v1.Subject{subject},
					})
					if err != nil && !apierrors.IsAlreadyExists(err) {
						returnErr = errors.Join(returnErr, err)
					}
				} else {
					returnErr = errors.Join(returnErr, err)
				}
			}
		}
	}
	return returnErr
}

// reconcileNamespacedRoleBindings ensures that RoleBindings exist for each namespace listed in NamespacedRules
// from the associated GlobalRole
func (grb *globalRoleBindingLifecycle) reconcileNamespacedRoleBindings(globalRoleBinding *v3.GlobalRoleBinding) error {
	var returnError error
	grbName := wrangler.SafeConcatName(globalRoleBinding.Name)
	gr, err := grb.grLister.Get("", globalRoleBinding.GlobalRoleName)
	if err != nil {
		return fmt.Errorf("unable to get globalRole %s: %w", globalRoleBinding.GlobalRoleName, err)
	}

	roleBindingUIDs := map[types.UID]struct{}{}

	for ns := range gr.NamespacedRules {
		namespace, err := grb.nsCache.Get(ns)
		if apierrors.IsNotFound(err) || namespace == nil {
			// When a namespace is not found, don't re-enqueue GlobalRoleBinding
			logrus.Warnf("[%v] Namespace %s not found. Not re-enqueueing GlobalRoleBinding %s", grController, ns, globalRoleBinding.Name)
			continue
		} else if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("couldn't get namespace %s: %w", ns, err))
			continue
		}

		roleName := wrangler.SafeConcatName(gr.Name, ns)
		roleRef := v1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		}

		subject := rbac.GetGRBSubject(globalRoleBinding)
		rbName := wrangler.SafeConcatName(globalRoleBinding.Name, ns)
		roleBinding, err := grb.roleBindingLister.Get(ns, rbName)
		if err == nil {
			if reflect.DeepEqual(roleRef, roleBinding.RoleRef) && roleBinding.Labels != nil && roleBinding.Labels[grbOwnerLabel] == grbName {
				roleBindingUIDs[roleBinding.UID] = struct{}{}
				continue
			}
			// Since roleRef is immutable, we have to delete and recreate the RB
			err = grb.roleBindings.DeleteNamespaced(roleBinding.Namespace, roleBinding.Name, &metav1.DeleteOptions{})
			if err != nil {
				returnError = errors.Join(returnError, err)
				continue
			}
		} else if !apierrors.IsNotFound(err) {
			returnError = errors.Join(returnError, err)
			continue
		}

		// If the namespace is terminating, don't create RoleBinding
		if namespace.Status.Phase == corev1.NamespaceTerminating {
			logrus.Warnf("[%v] Namespace %s is terminating. Not creating roleBinding %s for %s", grController, ns, rbName, globalRoleBinding.Name)
			continue
		}

		// Create a new RoleBinding
		newRoleBinding := &v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rbName,
				Namespace: ns,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: globalRoleBinding.APIVersion,
						Kind:       globalRoleBinding.Kind,
						UID:        globalRoleBinding.UID,
						Name:       globalRoleBinding.Name,
					},
				},
				Labels: map[string]string{
					grbOwnerLabel: grbName,
				},
			},
			RoleRef:  roleRef,
			Subjects: []v1.Subject{subject},
		}

		createdRB, err := grb.roleBindings.Create(newRoleBinding)
		if err != nil {
			returnError = errors.Join(returnError, err)
			continue
		}
		roleBindingUIDs[createdRB.UID] = struct{}{}
	}

	// get all the roleBindings claiming to be owned by this GRB and remove any that shouldn't exist
	r, err := labels.NewRequirement(grbOwnerLabel, selection.Equals, []string{grbName})
	if err != nil {
		return errors.Join(returnError, fmt.Errorf("couldn't create label: %s: %w", grOwnerLabel, err))
	}

	rbs, err := grb.roleBindingLister.List("", labels.NewSelector().Add(*r))
	if err != nil {
		return errors.Join(returnError,
			fmt.Errorf("couldn't list roleBindings with label %s : %s: %w", grbOwnerLabel, grbName, err))
	}

	// After creating/updating all RBs, if the number of RBs with the grbOwnerLabel is the same as
	// as the number of created/updated RBs, we know there are no invalid RBs to purge
	if len(rbs) != len(roleBindingUIDs) {
		err = grb.purgeInvalidNamespacedRBs(rbs, roleBindingUIDs)
		if err != nil {
			returnError = errors.Join(returnError, err)
		}
	}
	return returnError
}

// purgeInvalidNamespacedRBs removes any roleBindings that aren't in the namespaces listed in the associated GlobalRole.namespacedRules
func (grb *globalRoleBindingLifecycle) purgeInvalidNamespacedRBs(rbs []*v1.RoleBinding, uids map[types.UID]struct{}) error {
	var returnError error
	for _, rb := range rbs {
		if _, ok := uids[rb.UID]; !ok {
			err := grb.roleBindings.DeleteNamespaced(rb.Namespace, rb.Name, &metav1.DeleteOptions{})
			if err != nil {
				returnError = errors.Join(returnError, fmt.Errorf("couldn't delete roleBinding %s: %w", rb.Name, err))
			}
		}
	}
	return returnError
}

// isCRTBValid determines if a given CRTB is up to date for a given cluster and owning global role binding. Should
// only be used in the context of CRTBs owned by GRBs
func isCRTBValid(crtb *v3.ClusterRoleTemplateBinding, cluster *v3.Cluster, binding *v3.GlobalRoleBinding) bool {
	return crtb != nil && cluster != nil && binding != nil &&
		crtb.ClusterName == cluster.Name &&
		crtb.UserName == binding.UserName &&
		crtb.GroupPrincipalName == binding.GroupPrincipalName &&
		crtb.DeletionTimestamp == nil
}
