package project_cluster

import (
	"errors"
	"reflect"

	"encoding/json"
	"fmt"

	"github.com/rancher/norman/condition"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/systemimage"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	mgr                *config.ManagementContext
	clusterClient      v3.ClusterInterface
	crtbLister         v3.ClusterRoleTemplateBindingLister
	nsLister           corev1.NamespaceLister
	projects           wranglerv3.ProjectClient
	projectLister      v3.ProjectLister
	rbLister           rbacv1.RoleBindingLister
	roleBindings       rbacv1.RoleBindingInterface
	roleTemplateLister v3.RoleTemplateLister
}

// NewClusterLifecycle creates and returns a clusterLifecycle from a given ManagementContext
func NewClusterLifecycle(management *config.ManagementContext) *clusterLifecycle {
	return &clusterLifecycle{
		mgr:                management,
		clusterClient:      management.Management.Clusters(""),
		crtbLister:         management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		nsLister:           management.Core.Namespaces("").Controller().Lister(),
		projects:           management.Wrangler.Mgmt.Project(),
		projectLister:      management.Management.Projects("").Controller().Lister(),
		rbLister:           management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:       management.RBAC.RoleBindings(""),
		roleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
	}
}

// Sync gets called whenever a cluster is created or updated and ensures the cluster
// has all the necessary backing resources
func (l *clusterLifecycle) Sync(key string, orig *apisv3.Cluster) (runtime.Object, error) {
	if orig == nil || !orig.DeletionTimestamp.IsZero() {
		return orig, nil
	}

	obj := orig.DeepCopyObject()
	obj, err := reconcileResourceToNamespace(obj, ClusterCreateController, l.nsLister, l.mgr.K8sClient.CoreV1().Namespaces())
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
		logrus.Infof("[%v] Updating cluster %v", ClusterCreateController, orig.Name)
		_, err = l.clusterClient.ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	obj, err = l.reconcileClusterCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", ClusterCreateController, orig.Name)
		_, err = l.clusterClient.ObjectClient().Update(orig.Name, obj)
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

	var returnErr error
	set := labels.Set{rbac.RestrictedAdminClusterRoleBinding: "true"}
	rbs, err := l.rbLister.List(obj.Name, labels.SelectorFromSet(set))
	returnErr = errors.Join(returnErr, err)

	for _, rb := range rbs {
		err := l.roleBindings.DeleteNamespaced(obj.Name, rb.Name, &metav1.DeleteOptions{})
		returnErr = errors.Join(returnErr, err)
	}
	returnErr = errors.Join(
		l.deleteSystemProject(obj, ClusterRemoveController),
		deleteNamespace(obj, ClusterRemoveController, l.mgr.K8sClient.CoreV1().Namespaces()),
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
		// Attempt to use the cache first
		projects, err := l.projectLister.List(metaAccessor.GetName(), labels.AsSelector())
		if err != nil || len(projects) > 0 {
			return obj, err
		}

		// Cache failed, try the API
		projects2, err := l.projects.List(metaAccessor.GetName(), metav1.ListOptions{LabelSelector: labels.String()})
		if err != nil || len(projects2.Items) > 0 {
			return obj, err
		}

		annotation := map[string]string{}

		creatorID := metaAccessor.GetAnnotations()[CreatorIDAnnotation]
		if creatorID != "" {
			annotation[CreatorIDAnnotation] = creatorID
		}

		if name == project.System {
			latestSystemVersion, err := systemimage.GetSystemImageVersion()
			if err != nil {
				return obj, err
			}
			annotation[project.SystemImageVersionAnnotation] = latestSystemVersion
		}

		project := &apisv3.Project{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "p-",
				Annotations:  annotation,
				Labels:       labels,
			},
			Spec: apisv3.ProjectSpec{
				DisplayName: name,
				Description: fmt.Sprintf("%s project created for the cluster", name),
				ClusterName: metaAccessor.GetName(),
			},
		}
		updated, err := l.addRTAnnotation(project, "project")
		if err != nil {
			return obj, err
		}
		project = updated.(*apisv3.Project)
		logrus.Infof("[%v] Creating %s project for cluster %v", ClusterCreateController, name, metaAccessor.GetName())
		if _, err = l.mgr.Management.Projects(metaAccessor.GetName()).Create(project); err != nil {
			return obj, err
		}
		return obj, nil
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

	rt, err := l.roleTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return obj, err
	}

	annoMap := make(map[string][]string)

	var restrictedAdmin bool
	if settings.RestrictedDefaultAdmin.Get() == "true" {
		restrictedAdmin = true
	}

	// Created isn't used in this function, but it is required in the annotation data
	annoMap["created"] = []string{}
	annoMap["required"] = []string{}

	switch context {
	case "project":
		// If we are in restricted mode, ensure the default projects are not granting
		// permissions to the restricted-admin
		if restrictedAdmin {
			proj := obj.(*apisv3.Project)
			if proj.Spec.ClusterName == "local" && (proj.Spec.DisplayName == "Default" || proj.Spec.DisplayName == "System") {
				break
			}
		}

		for _, role := range rt {
			if role.ProjectCreatorDefault && !role.Locked {
				annoMap["required"] = append(annoMap["required"], role.Name)
			}
		}
	case "cluster":
		// If we are in restricted mode, ensure we don't give the default restricted-admin
		// the default permissions in the cluster
		if restrictedAdmin && accessor.GetName() == "local" {
			break
		}

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
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, fmt.Errorf("error accessing cluster object %v: %w", obj, err)
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[CreatorIDAnnotation]
		if !ok || creatorID == "" {
			logrus.Warnf("[%v] %v %v has no creatorId annotation. Cannot add creator as owner", ClusterCreateController, typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}
		cluster := obj.(*apisv3.Cluster)

		if apisv3.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
			// The clusterRoleBindings are already completed, no need to check
			return obj, nil
		}

		roleJSON := cluster.Annotations[roleTemplatesRequiredAnnotation]
		if roleJSON == "" {
			return cluster, nil
		}

		roleMap := make(map[string][]string)
		err = json.Unmarshal([]byte(roleJSON), &roleMap)
		if err != nil {
			return obj, err
		}

		var createdRoles []string

		for _, role := range roleMap["required"] {
			rtbName := "creator-" + role

			if rtb, _ := l.crtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
				createdRoles = append(createdRoles, role)
				// This clusterRoleBinding exists, need to check all of them so keep going
				continue
			}

			// The clusterRoleBinding doesn't exist yet so create it
			om := metav1.ObjectMeta{
				Name:      rtbName,
				Namespace: metaAccessor.GetName(),
			}
			om.Annotations = crtbCreatorOwnerAnnotations

			logrus.Infof("[%v] Creating creator clusterRoleTemplateBinding for user %v for cluster %v", ClusterCreateController, creatorID, metaAccessor.GetName())
			if _, err := l.mgr.Management.ClusterRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ClusterRoleTemplateBinding{
				ObjectMeta:       om,
				ClusterName:      metaAccessor.GetName(),
				RoleTemplateName: role,
				UserName:         creatorID,
			}); err != nil && !apierrors.IsAlreadyExists(err) {
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
		if err != nil {
			return obj, err
		}
		return obj, nil
	})
}

func (l *clusterLifecycle) updateClusterAnnotationandCondition(cluster *apisv3.Cluster, annotation string, updateCondition bool) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on cluster %v", ClusterCreateController, cluster.Name)
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
