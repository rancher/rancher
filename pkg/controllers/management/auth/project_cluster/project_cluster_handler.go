package project_cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/norman/condition"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	CreatorIDAnn                  = "field.cattle.io/creatorId"
	creatorOwnerBindingAnnotation = "authz.management.cattle.io/creator-owner-binding"
	roleTemplatesRequired         = "authz.management.cattle.io/creator-role-bindings"
)

var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func NewPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:     management,
		nsLister: management.Core.Namespaces("").Controller().Lister(),
	}
	p := &projectLifecycle{
		mgr:                  m,
		crtbClient:           management.Management.ClusterRoleTemplateBindings(""),
		crtbLister:           management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		prtbLister:           management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		rbLister:             management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:         management.RBAC.RoleBindings(""),
		systemAccountManager: systemaccount.NewManager(management),
	}
	c := &clusterLifecycle{
		mgr:                m,
		crtbLister:         management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		projects:           management.Wrangler.Mgmt.Project(),
		projectLister:      management.Management.Projects("").Controller().Lister(),
		rbLister:           management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:       management.RBAC.RoleBindings(""),
		roleTemplateLister: management.Management.RoleTemplates("").Controller().Lister(),
	}
	return p, c
}

type mgr struct {
	mgmt     *config.ManagementContext
	nsLister corev1.NamespaceLister
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
			logrus.Infof("[%v] Setting InitialRolesPopulated condition on cluster %v", ClusterCreateController, cluster.Name)
		}
		return nil
	}
	return nil
}
