package project_cluster

import (
	"errors"
	"reflect"
	"strings"

	"encoding/json"
	"fmt"

	"github.com/rancher/norman/condition"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

const (
	// The name of the cluster create controller
	ClusterCreateController = "mgmt-cluster-rbac-delete" // TODO the word delete here is wrong, but changing it would break backwards compatibility
	// The name of the cluster remove controller
	ClusterRemoveController = "mgmt-cluster-rbac-remove"
)

var (
	defaultProjectLabels = labels.Set{
		"authz.management.cattle.io/default-project": "true",
	}
	systemProjectLabels = labels.Set{
		"authz.management.cattle.io/system-project": "true",
	}
)

type clusterLifecycle struct {
	clusterClient      v3.ClusterController
	crtbLister         v3.ClusterRoleTemplateBindingCache
	crtbClient         v3.ClusterRoleTemplateBindingController
	nsLister           corev1.NamespaceCache
	nsClient           k8scorev1.NamespaceInterface
	projects           v3.ProjectClient
	projectLister      v3.ProjectCache
	rbLister           rbacv1.RoleBindingCache
	roleBindings       rbacv1.RoleBindingController
	roleTemplateLister v3.RoleTemplateCache
	crClient           rbacv1.ClusterRoleController
}

// NewClusterLifecycle creates and returns a clusterLifecycle from a given ManagementContext
func NewClusterLifecycle(management *config.ManagementContext) *clusterLifecycle {
	return &clusterLifecycle{
		clusterClient:      management.Wrangler.Mgmt.Cluster(),
		crtbLister:         management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbClient:         management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		nsLister:           management.Wrangler.Core.Namespace().Cache(),
		nsClient:           management.K8sClient.CoreV1().Namespaces(),
		projects:           management.Wrangler.Mgmt.Project(),
		projectLister:      management.Wrangler.Mgmt.Project().Cache(),
		rbLister:           management.Wrangler.RBAC.RoleBinding().Cache(),
		roleBindings:       management.Wrangler.RBAC.RoleBinding(),
		roleTemplateLister: management.Wrangler.Mgmt.RoleTemplate().Cache(),
		crClient:           management.Wrangler.RBAC.ClusterRole(),
	}
}

// Sync gets called whenever a cluster is created or updated and ensures the cluster
// has all the necessary backing resources
func (l *clusterLifecycle) Sync(key string, orig *apisv3.Cluster) (runtime.Object, error) {
	if orig == nil || !orig.DeletionTimestamp.IsZero() {
		return orig, nil
	}

	obj := orig.DeepCopyObject()
	obj, err := reconcileResourceToNamespace(obj, ClusterCreateController, l.nsLister, l.nsClient)
	if err != nil {
		return nil, err
	}

	obj, err = l.createDefaultProject(obj)
	if err != nil {
		return nil, err
	}

	obj, err = l.createSystemProject(obj)
	if err != nil {
		return nil, err
	}
	obj, err = l.addRTAnnotation(obj, "cluster")
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%s] Updating cluster %s", ClusterCreateController, orig.Name)
		cluster := obj.(*apisv3.Cluster)
		_, err = l.clusterClient.Update(cluster)
		if err != nil {
			return nil, err
		}
	}

	if features.AggregatedRoleTemplates.Enabled() {
		if err := createMembershipRoles(obj, l.crClient); err != nil {
			return nil, err
		}
	}

	obj, err = l.reconcileClusterCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%s] Updating cluster %s", ClusterCreateController, orig.Name)
		cluster := obj.(*apisv3.Cluster)
		_, err = l.clusterClient.Update(cluster)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// Create is a no-op because the Sync function takes care of resource orchestration
func (l *clusterLifecycle) Create(obj *apisv3.Cluster) (runtime.Object, error) {
	return obj, nil
}

// Updated is a no-op because the Sync function takes care of resource orchestration
func (l *clusterLifecycle) Updated(obj *apisv3.Cluster) (runtime.Object, error) {
	return obj, nil
}

// Remove deletes all backing resources created by the cluster
func (l *clusterLifecycle) Remove(obj *apisv3.Cluster) (runtime.Object, error) {
	if len(obj.Finalizers) > 1 {
		logrus.Debugf("Skipping rbac cleanup for cluster [%s] until all other finalizers are removed.", obj.Name)
		return obj, generic.ErrSkip
	}

	returnErr := errors.Join(
		l.deleteSystemProject(obj, ClusterRemoveController),
		deleteNamespace(obj, ClusterRemoveController, l.nsClient),
	)
	return obj, returnErr
}

func (l *clusterLifecycle) createDefaultProject(obj runtime.Object) (runtime.Object, error) {
	return l.createProject(project.Default, apisv3.ClusterConditionDefaultProjectCreated, obj, defaultProjectLabels)
}

func (l *clusterLifecycle) createSystemProject(obj runtime.Object) (runtime.Object, error) {
	return l.createProject(project.System, apisv3.ClusterConditionSystemProjectCreated, obj, systemProjectLabels)
}

func (l *clusterLifecycle) createProject(name string, cond condition.Cond, obj runtime.Object, labels labels.Set) (runtime.Object, error) {
	return cond.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, fmt.Errorf("error accessing project object %v: %w", obj, err)
		}

		clusterName := metaAccessor.GetName()

		// Attempt to use the cache first
		projects, err := l.projectLister.List(clusterName, labels.AsSelector())
		if err != nil || len(projects) > 0 {
			return obj, err
		}

		// Cache failed, try the API
		projects2, err := l.projects.List(clusterName, metav1.ListOptions{LabelSelector: labels.String()})
		if err != nil || len(projects2.Items) > 0 {
			return obj, err
		}

		clusterAnnotations := metaAccessor.GetAnnotations()
		annotations := map[string]string{}

		// If we have noCreatorRBAC annotation, propagate the annotation to the project and don't add creatorId annotation
		if noCreatorRBAC, ok := clusterAnnotations[NoCreatorRBACAnnotation]; ok {
			annotations[NoCreatorRBACAnnotation] = noCreatorRBAC
		} else {
			if creatorID := clusterAnnotations[CreatorIDAnnotation]; creatorID != "" {
				annotations[CreatorIDAnnotation] = creatorID
			}

			if creatorPrincipalName := clusterAnnotations[creatorPrincipalNameAnnotation]; creatorPrincipalName != "" {
				annotations[creatorPrincipalNameAnnotation] = creatorPrincipalName
			}
		}

		project := &apisv3.Project{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "p-",
				Annotations:  annotations,
				Labels:       labels,
				Namespace:    clusterName,
			},
			Spec: apisv3.ProjectSpec{
				DisplayName: name,
				Description: name + " project created for the cluster",
				ClusterName: clusterName,
			},
		}
		updated, err := l.addRTAnnotation(project, "project")
		if err != nil {
			return obj, err
		}

		project = updated.(*apisv3.Project)

		logrus.Infof("[%s] Creating %s project for cluster %s", ClusterCreateController, name, clusterName)
		_, err = l.projects.Create(project)

		return obj, err
	})
}

// deleteSystemProject deletes the system project(s) for a cluster in preparation for deleting the cluster namespace.
// Normally, the webhook prevents deleting the system project, so Rancher needs to use the sudo user to force it.
// Otherwise, the deleted namespace will be stuck terminating because it cannot garbage collect the project.
func (l *clusterLifecycle) deleteSystemProject(cluster *apisv3.Cluster, controller string) error {
	bypassClient, err := l.projects.WithImpersonation(controllers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("[%s] failed to create impersonation client: %w", controller, err)
	}
	projects, err := l.projectLister.List(cluster.Name, systemProjectLabels.AsSelector())
	if err != nil {
		return fmt.Errorf("[%s] failed to list projects: %w", controller, err)
	}
	var deleteError error
	for _, p := range projects {
		logrus.Infof("[%s] Deleting project %s", controller, p.Name)
		err = bypassClient.Delete(p.Namespace, p.Name, nil)
		if err != nil {
			deleteError = errors.Join(deleteError, fmt.Errorf("[%s] failed to delete project '%s/%s': %w", controller, p.Namespace, p.Name, err))
		}
	}
	return deleteError
}

func (l *clusterLifecycle) addRTAnnotation(obj runtime.Object, context string) (runtime.Object, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return obj, fmt.Errorf("error accessing object %v: %w", obj, err)
	}

	// If the annotation is already there move along
	if _, ok := accessor.GetAnnotations()[roleTemplatesRequiredAnnotation]; ok {
		return obj, nil
	}

	rt, err := l.roleTemplateLister.List(labels.NewSelector())
	if err != nil {
		return obj, err
	}

	annoMap := make(map[string][]string)

	// Created isn't used in this function, but it is required in the annotation data
	annoMap["created"] = []string{}
	annoMap["required"] = []string{}

	switch context {
	case "project":
		for _, role := range rt {
			if role.ProjectCreatorDefault && !role.Locked {
				annoMap["required"] = append(annoMap["required"], role.Name)
			}
		}
	case "cluster":
		for _, role := range rt {
			if role.ClusterCreatorDefault && !role.Locked {
				annoMap["required"] = append(annoMap["required"], role.Name)
			}
		}
	}

	d, err := json.Marshal(annoMap)
	if err != nil {
		return obj, err
	}

	// Save the required role templates to the annotation on the obj
	if accessor.GetAnnotations() == nil {
		accessor.SetAnnotations(make(map[string]string))
	}
	accessor.GetAnnotations()[roleTemplatesRequiredAnnotation] = string(d)
	return obj, nil
}

func (l *clusterLifecycle) reconcileClusterCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	cluster, ok := obj.(*apisv3.Cluster)
	if !ok {
		return obj, fmt.Errorf("expected cluster, got %T", obj)
	}

	if _, ok := cluster.Annotations[NoCreatorRBACAnnotation]; ok {
		logrus.Infof("[%s] annotation %s found. Skipping adding creator as owner", ClusterCreateController, NoCreatorRBACAnnotation)
		return obj, nil
	}
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		creatorID := cluster.Annotations[CreatorIDAnnotation]
		if creatorID == "" {
			logrus.Warnf("[%s] cluster %s has no creatorId annotation. Cannot add creator as owner", ClusterCreateController, cluster.Name)
			return obj, nil
		}

		if apisv3.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
			// The clusterRoleBindings are already completed, no need to check
			return obj, nil
		}

		creatorRoleBindings := cluster.Annotations[roleTemplatesRequiredAnnotation]
		if creatorRoleBindings == "" {
			return cluster, nil
		}

		roleMap := make(map[string][]string)
		err := json.Unmarshal([]byte(creatorRoleBindings), &roleMap)
		if err != nil {
			return obj, err
		}

		var createdRoles []string
		for _, role := range roleMap["required"] {
			rtbName := "creator-" + role

			if rtb, _ := l.crtbLister.Get(cluster.Name, rtbName); rtb != nil {
				createdRoles = append(createdRoles, role)
				// This clusterRoleBinding exists, need to check all of them so keep going
				continue
			}

			// The clusterRoleBinding doesn't exist yet so create it
			crtb := &apisv3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        rtbName,
					Namespace:   cluster.Name,
					Annotations: crtbCreatorOwnerAnnotations,
				},
				ClusterName:      cluster.Name,
				RoleTemplateName: role,
				UserName:         creatorID,
			}

			if principalName := cluster.Annotations[creatorPrincipalNameAnnotation]; principalName != "" {
				if !strings.HasPrefix(principalName, "local") {
					// Setting UserPrincipalName only makes sense for non-local users.
					crtb.UserPrincipalName = principalName
					crtb.UserName = ""
				}
			}

			logrus.Infof("[%s] Creating creator clusterRoleTemplateBinding for user %s for cluster %s", ClusterCreateController, creatorID, cluster.Name)
			_, err := l.crtbClient.Create(crtb)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return obj, err
			}

			createdRoles = append(createdRoles, role)
		}

		roleMap["created"] = createdRoles
		d, err := json.Marshal(roleMap)
		if err != nil {
			return obj, err
		}

		updateCondition := reflect.DeepEqual(roleMap["required"], createdRoles)

		err = l.updateClusterAnnotationandCondition(cluster, string(d), updateCondition)

		return obj, err
	})
}

func (l *clusterLifecycle) updateClusterAnnotationandCondition(cluster *apisv3.Cluster, annotation string, updateCondition bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		c, err := l.clusterClient.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		c.Annotations[roleTemplatesRequiredAnnotation] = annotation

		if updateCondition {
			apisv3.ClusterConditionInitialRolesPopulated.True(c)
		}

		_, err = l.clusterClient.Update(c)
		if err != nil {
			return err
		}
		// Only log if we successfully updated the cluster
		if updateCondition {
			logrus.Infof("[%s] Setting InitialRolesPopulated condition on cluster %s", ClusterCreateController, cluster.Name)
		}

		return nil
	})
}
