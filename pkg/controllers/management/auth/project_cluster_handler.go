package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/norman/condition"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/managementuserlegacy/systemimage"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	creatorIDAnn                  = "field.cattle.io/creatorId"
	creatorOwnerBindingAnnotation = "authz.management.cattle.io/creator-owner-binding"
	roleTemplatesRequired         = "authz.management.cattle.io/creator-role-bindings"
)

var defaultProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/default-project": "true"})
var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})
var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func newPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:                 management,
		nsLister:             management.Core.Namespaces("").Controller().Lister(),
		prtbLister:           management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbLister:           management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		crtbClient:           management.Management.ClusterRoleTemplateBindings(""),
		projectLister:        management.Management.Projects("").Controller().Lister(),
		projects:             management.Wrangler.Mgmt.Project(),
		roleTemplateLister:   management.Management.RoleTemplates("").Controller().Lister(),
		systemAccountManager: systemaccount.NewManager(management),
		rbLister:             management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:         management.RBAC.RoleBindings(""),
	}
	p := &projectLifecycle{
		mgr: m,
	}
	c := &clusterLifecycle{
		mgr: m,
	}
	return p, c
}

type mgr struct {
	mgmt          *config.ManagementContext
	nsLister      corev1.NamespaceLister
	projectLister v3.ProjectLister
	projects      wranglerv3.ProjectClient

	prtbLister           v3.ProjectRoleTemplateBindingLister
	crtbLister           v3.ClusterRoleTemplateBindingLister
	crtbClient           v3.ClusterRoleTemplateBindingInterface
	roleTemplateLister   v3.RoleTemplateLister
	systemAccountManager *systemaccount.Manager
	rbLister             rbacv1.RoleBindingLister
	roleBindings         rbacv1.RoleBindingInterface
}

func (m *mgr) createDefaultProject(obj runtime.Object) (runtime.Object, error) {
	return m.createProject(project.Default, apisv3.ClusterConditionDefaultProjectCreated, obj, defaultProjectLabels)
}

func (m *mgr) createSystemProject(obj runtime.Object) (runtime.Object, error) {
	return m.createProject(project.System, apisv3.ClusterConditionSystemProjectCreated, obj, systemProjectLabels)
}

func (m *mgr) createProject(name string, cond condition.Cond, obj runtime.Object, labels labels.Set) (runtime.Object, error) {
	return cond.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}
		// Attempt to use the cache first
		projects, err := m.projectLister.List(metaAccessor.GetName(), labels.AsSelector())
		if err != nil || len(projects) > 0 {
			return obj, err
		}

		// Cache failed, try the API
		projects2, err := m.projects.List(metaAccessor.GetName(), v1.ListOptions{LabelSelector: labels.String()})
		if err != nil || len(projects2.Items) > 0 {
			return obj, err
		}

		annotation := map[string]string{}

		creatorID := metaAccessor.GetAnnotations()[creatorIDAnn]
		if creatorID != "" {
			annotation[creatorIDAnn] = creatorID
		}

		if name == project.System {
			latestSystemVersion, err := systemimage.GetSystemImageVersion()
			if err != nil {
				return obj, err
			}
			annotation[project.SystemImageVersionAnn] = latestSystemVersion
		}

		project := &apisv3.Project{
			ObjectMeta: v1.ObjectMeta{
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
		updated, err := m.addRTAnnotation(project, "project")
		if err != nil {
			return obj, err
		}
		project = updated.(*apisv3.Project)
		logrus.Infof("[%v] Creating %s project for cluster %v", clusterCreateController, name, metaAccessor.GetName())
		if _, err = m.mgmt.Management.Projects(metaAccessor.GetName()).Create(project); err != nil {
			return obj, err
		}
		return obj, nil
	})
}

// deleteSystemProject deletes the system project(s) for a cluster in preparation for deleting the cluster namespace.
// Normally, the webhook prevents deleting the system project, so Rancher needs to use the sudo user to force it.
// Otherwise, the deleted namespace will be stuck terminating because it cannot garbage collect the project.
func (m *mgr) deleteSystemProject(cluster *apisv3.Cluster, controller string) error {
	bypassClient, err := m.projects.WithImpersonation(controllers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("[%s] failed to create impersonation client: %w", controller, err)
	}
	projects, err := m.projectLister.List(cluster.Name, systemProjectLabels.AsSelector())
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

func (m *mgr) reconcileCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return apisv3.CreatorMadeOwner.DoUntilTrue(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok || creatorID == "" {
			logrus.Warnf("%v %v has no creatorId annotation. Cannot add creator as owner", typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}

		switch typeAccessor.GetKind() {
		case v3.ProjectGroupVersionKind.Kind:
			project := obj.(*apisv3.Project)

			if apisv3.ProjectConditionInitialRolesPopulated.IsTrue(project) {
				// The projectRoleBindings are already completed, no need to check
				break
			}

			// If the project does not have the annotation it indicates the
			// project is from a previous rancher version so don't add the
			// default bindings.
			roleJSON, ok := project.Annotations[roleTemplatesRequired]
			if !ok {
				return project, nil
			}

			roleMap := make(map[string][]string)
			err = json.Unmarshal([]byte(roleJSON), &roleMap)
			if err != nil {
				return obj, err
			}

			var createdRoles []string

			for _, role := range roleMap["required"] {
				rtbName := "creator-" + role

				if rtb, _ := m.prtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This projectRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The projectRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}

				logrus.Infof("[%v] Creating creator projectRoleTemplateBinding for user %v for project %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ProjectRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ProjectRoleTemplateBinding{
					ObjectMeta:       om,
					ProjectName:      metaAccessor.GetNamespace() + ":" + metaAccessor.GetName(),
					RoleTemplateName: role,
					UserName:         creatorID,
				}); err != nil && !apierrors.IsAlreadyExists(err) {
					return obj, err
				}
				createdRoles = append(createdRoles, role)
			}

			project = project.DeepCopy()

			roleMap["created"] = createdRoles
			d, err := json.Marshal(roleMap)
			if err != nil {
				return obj, err
			}

			project.Annotations[roleTemplatesRequired] = string(d)

			if reflect.DeepEqual(roleMap["required"], createdRoles) {
				apisv3.ProjectConditionInitialRolesPopulated.True(project)
				logrus.Infof("[%v] Setting InitialRolesPopulated condition on project %v", ctrbMGMTController, project.Name)
			}
			if _, err := m.mgmt.Management.Projects("").Update(project); err != nil {
				return obj, err
			}

		case v3.ClusterGroupVersionKind.Kind:
			cluster := obj.(*apisv3.Cluster)

			if apisv3.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
				// The clusterRoleBindings are already completed, no need to check
				break
			}

			roleJSON, ok := cluster.Annotations[roleTemplatesRequired]
			if !ok {
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

				if rtb, _ := m.crtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
					createdRoles = append(createdRoles, role)
					// This clusterRoleBinding exists, need to check all of them so keep going
					continue
				}

				// The clusterRoleBinding doesn't exist yet so create it
				om := v1.ObjectMeta{
					Name:      rtbName,
					Namespace: metaAccessor.GetName(),
				}
				om.Annotations = crtbCreatorOwnerAnnotations

				logrus.Infof("[%v] Creating creator clusterRoleTemplateBinding for user %v for cluster %v", projectCreateController, creatorID, metaAccessor.GetName())
				if _, err := m.mgmt.Management.ClusterRoleTemplateBindings(metaAccessor.GetName()).Create(&apisv3.ClusterRoleTemplateBinding{
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

			err = m.updateClusterAnnotationandCondition(cluster, string(d), updateCondition)
			if err != nil {
				return obj, err
			}
		}

		return obj, nil
	})
}

func (m *mgr) deleteNamespace(obj runtime.Object, controller string) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return condition.Error("MissingMetadata", err)
	}

	nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
	ns, err := nsClient.Get(context.TODO(), o.GetName(), v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%s] Deleting namespace %s", controller, o.GetName())
		err = nsClient.Delete(context.TODO(), o.GetName(), v1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func (m *mgr) reconcileResourceToNamespace(obj runtime.Object, controller string) (runtime.Object, error) {
	return apisv3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		o, err := meta.Accessor(obj)
		if err != nil {
			return obj, condition.Error("MissingMetadata", err)
		}
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, condition.Error("MissingTypeMetadata", err)
		}

		ns, _ := m.nsLister.Get("", o.GetName())
		if ns == nil {
			nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
			logrus.Infof("[%v] Creating namespace %v", controller, o.GetName())
			_, err := nsClient.Create(context.TODO(), &v12.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: o.GetName(),
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			}, v1.CreateOptions{})
			if err != nil {
				return obj, condition.Error("NamespaceCreationFailure", fmt.Errorf("failed to create namespace for %v %v: %w", t.GetKind(), o.GetName(), err))
			}
		}

		return obj, nil
	})
}

func (m *mgr) addRTAnnotation(obj runtime.Object, context string) (runtime.Object, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	// If the annotation is already there move along
	if _, ok := meta.GetAnnotations()[roleTemplatesRequired]; ok {
		return obj, nil
	}

	rt, err := m.roleTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return obj, err
	}

	annoMap := make(map[string][]string)

	var restrictedAdmin bool
	if settings.RestrictedDefaultAdmin.Get() == "true" {
		restrictedAdmin = true
	}

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
		if restrictedAdmin && meta.GetName() == "local" {
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
	if meta.GetAnnotations() == nil {
		meta.SetAnnotations(make(map[string]string))
	}
	meta.GetAnnotations()[roleTemplatesRequired] = string(d)
	return obj, nil
}

func (m *mgr) updateClusterAnnotationandCondition(cluster *apisv3.Cluster, anno string, updateCondition bool) error {
	sleep := 100
	for i := 0; i <= 3; i++ {
		c, err := m.mgmt.Management.Clusters("").Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}

		c.Annotations[roleTemplatesRequired] = anno

		if updateCondition {
			apisv3.ClusterConditionInitialRolesPopulated.True(c)
		}
		_, err = m.mgmt.Management.Clusters("").Update(c)
		if err != nil {
			if apierrors.IsConflict(err) {
				time.Sleep(time.Duration(sleep) * time.Millisecond)
				sleep *= 2
				continue
			}
			return err
		}
		// Only log if we successfully updated the cluster
		if updateCondition {
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on cluster %v", ctrbMGMTController, cluster.Name)
		}
		return nil
	}
	return nil
}
