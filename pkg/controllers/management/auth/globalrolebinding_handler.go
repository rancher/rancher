package auth

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
)

var (
	globalRoleBindingLabel = map[string]string{"authz.management.cattle.io/globalrolebinding": "true"}
)

const (
	catalogTemplateResourceRule        = "catalogtemplates"
	catalogTemplateVersionResourceRule = "catalogtemplateversions"
	crbNameAnnotation                  = "authz.management.cattle.io/crb-name"
	crbNamePrefix                      = "cattle-globalrolebinding-"
	GlobalCatalogRole                  = "global-catalog"
	globalCatalogRoleBinding           = "global-catalog-binding"
	grbController                      = "mgmt-auth-grb-controller"
	TemplateResourceRule               = "templates"
	TemplateVersionResourceRule        = "templateversions"
)

func newGlobalRoleBindingLifecycle(management *config.ManagementContext, clusterManager *clustermanager.Manager) *globalRoleBindingLifecycle {
	return &globalRoleBindingLifecycle{
		clusters:          management.Management.Clusters(""),
		clusterLister:     management.Management.Clusters("").Controller().Lister(),
		projectLister:     management.Management.Projects("").Controller().Lister(),
		clusterManager:    clusterManager,
		clusterRoles:      management.RBAC.ClusterRoles(""),
		crbClient:         management.RBAC.ClusterRoleBindings(""),
		crbLister:         management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:          management.RBAC.ClusterRoles("").Controller().Lister(),
		grLister:          management.Management.GlobalRoles("").Controller().Lister(),
		roles:             management.RBAC.Roles(""),
		roleLister:        management.RBAC.Roles("").Controller().Lister(),
		roleBindings:      management.RBAC.RoleBindings(""),
		roleBindingLister: management.RBAC.RoleBindings("").Controller().Lister(),
	}
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
	grLister          v3.GlobalRoleLister
	roles             rbacv1.RoleInterface
	roleLister        rbacv1.RoleLister
	roleBindings      rbacv1.RoleBindingInterface
	roleBindingLister rbacv1.RoleBindingLister
}

func (grb *globalRoleBindingLifecycle) Create(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return obj, err
}

func (grb *globalRoleBindingLifecycle) Updated(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	err := grb.reconcileGlobalRoleBinding(obj)
	return obj, err
}

func (grb *globalRoleBindingLifecycle) Remove(obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if obj.GlobalRoleName == rbac.GlobalAdmin || obj.GlobalRoleName == rbac.GlobalRestrictedAdmin {
		return obj, grb.deleteAdminBinding(obj)
	}
	// Don't need to delete the created ClusterRole because owner reference will take care of that
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
			if !IsClusterUnavailable(err) {
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
				return errors.Wrapf(err, "couldn't update ClusterRoleBinding %v", crb.Name)
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
			returnErr = multierror.Append(returnErr, err)
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
			returnErr = multierror.Append(returnErr, err)
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
				returnErr = multierror.Append(returnErr, err)
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
				returnErr = multierror.Append(returnErr, err)
				continue
			}
		}

		projects, err := grb.projectLister.List(cluster.Name, labels.NewSelector())
		if err != nil {
			returnErr = multierror.Append(returnErr, err)
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
						returnErr = multierror.Append(returnErr, err)
					}
				} else {
					returnErr = multierror.Append(returnErr, err)
				}
			}
		}
	}
	return returnErr
}

func IsClusterUnavailable(err error) bool {
	if apiError, ok := err.(*httperror.APIError); ok {
		return apiError.Code == httperror.ClusterUnavailable
	}
	return false
}
