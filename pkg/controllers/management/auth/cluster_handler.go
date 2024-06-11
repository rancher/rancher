package auth

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
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterCreateController = "mgmt-cluster-rbac-delete" // TODO the word delete here is wrong, but changing it would break backwards compatibility
	clusterRemoveController = "mgmt-cluster-rbac-remove"
)

var defaultProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/default-project": "true"})
var systemProjectLabels = labels.Set(map[string]string{"authz.management.cattle.io/system-project": "true"})

type clusterLifecycle struct {
	mgr                *mgr
	projects           wranglerv3.ProjectClient
	projectLister      v3.ProjectLister
	rbLister           rbacv1.RoleBindingLister
	roleBindings       rbacv1.RoleBindingInterface
	roleTemplateLister v3.RoleTemplateLister
}

func (l *clusterLifecycle) sync(key string, orig *apisv3.Cluster) (runtime.Object, error) {
	if orig == nil || !orig.DeletionTimestamp.IsZero() {
		return orig, nil
	}

	obj := orig.DeepCopyObject()
	obj, err := l.mgr.reconcileResourceToNamespace(obj, clusterCreateController)
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
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	obj, err = l.mgr.reconcileCreatorRTB(obj)
	if err != nil {
		return nil, err
	}

	// update if it has changed
	if obj != nil && !reflect.DeepEqual(orig, obj) {
		logrus.Infof("[%v] Updating cluster %v", clusterCreateController, orig.Name)
		_, err = l.mgr.mgmt.Management.Clusters("").ObjectClient().Update(orig.Name, obj)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (l *clusterLifecycle) Create(obj *apisv3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

func (l *clusterLifecycle) Updated(obj *apisv3.Cluster) (runtime.Object, error) {
	// no-op because the sync function will take care of it
	return obj, nil
}

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
		err := l.roleBindings.DeleteNamespaced(obj.Name, rb.Name, &v1.DeleteOptions{})
		returnErr = errors.Join(returnErr, err)
	}
	returnErr = errors.Join(
		l.deleteSystemProject(obj, clusterRemoveController),
		l.mgr.deleteNamespace(obj, clusterRemoveController),
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
			return obj, err
		}
		// Attempt to use the cache first
		projects, err := l.projectLister.List(metaAccessor.GetName(), labels.AsSelector())
		if err != nil || len(projects) > 0 {
			return obj, err
		}

		// Cache failed, try the API
		projects2, err := l.projects.List(metaAccessor.GetName(), v1.ListOptions{LabelSelector: labels.String()})
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
		updated, err := l.addRTAnnotation(project, "project")
		if err != nil {
			return obj, err
		}
		project = updated.(*apisv3.Project)
		logrus.Infof("[%v] Creating %s project for cluster %v", clusterCreateController, name, metaAccessor.GetName())
		if _, err = l.mgr.mgmt.Management.Projects(metaAccessor.GetName()).Create(project); err != nil {
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
	meta, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	// If the annotation is already there move along
	if _, ok := meta.GetAnnotations()[roleTemplatesRequired]; ok {
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
