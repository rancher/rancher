package auth

import (
	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	v13 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const creatorIDAnn = "field.cattle.io/creatorId"

func newPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:          management,
		nsLister:      management.Core.Namespaces("").Controller().Lister(),
		prtbLister:    management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbLister:    management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		projectLister: management.Management.Projects("").Controller().Lister(),
	}
	p := &projectLifecycle{
		mgr: m,
	}
	c := &clusterLifecycle{
		mgr: m,
	}
	return p, c
}

type projectLifecycle struct {
	mgr *mgr
}

func (l *projectLifecycle) Create(obj *v3.Project) (*v3.Project, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	if err != nil {
		return obj, err
	}

	_, err = l.mgr.reconcileCreatorRTB(obj)
	return obj, err
}

func (l *projectLifecycle) Updated(obj *v3.Project) (*v3.Project, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	if err != nil {
		return obj, err
	}
	return obj, err
}

func (l *projectLifecycle) Remove(obj *v3.Project) (*v3.Project, error) {
	err := l.mgr.deleteNamespace(obj)
	return obj, err
}

type clusterLifecycle struct {
	mgr *mgr
}

func (l *clusterLifecycle) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	if err != nil {
		return obj, err
	}

	_, err = l.mgr.createDefaultProject(obj)
	if err != nil {
		return obj, err
	}

	_, err = l.mgr.reconcileCreatorRTB(obj)
	return obj, err
}

func (l *clusterLifecycle) Updated(obj *v3.Cluster) (*v3.Cluster, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	if err != nil {
		return obj, err
	}

	_, err = l.mgr.createDefaultProject(obj)
	return obj, err
}

func (l *clusterLifecycle) Remove(obj *v3.Cluster) (*v3.Cluster, error) {
	err := l.mgr.deleteNamespace(obj)
	return obj, err
}

type mgr struct {
	mgmt          *config.ManagementContext
	nsLister      corev1.NamespaceLister
	projectLister v3.ProjectLister

	prtbLister v3.ProjectRoleTemplateBindingLister
	crtbLister v3.ClusterRoleTemplateBindingLister
}

func (m *mgr) createDefaultProject(obj *v3.Cluster) (runtime.Object, error) {
	return v3.ClusterConditionconditionDefautlProjectCreated.DoUntilTrue(obj, func() (runtime.Object, error) {
		projectName := "rancher-default"
		p, _ := m.projectLister.Get(obj.Name, projectName)
		if p != nil {
			return obj, nil
		}
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok {
			logrus.Warnf("Cluster %v has no creatorId annotation. Cannot create default project", metaAccessor.GetName())
			return obj, nil
		}

		_, err = m.mgmt.Management.Projects(obj.Name).Create(&v3.Project{
			ObjectMeta: v1.ObjectMeta{
				Name: projectName,
				Annotations: map[string]string{
					creatorIDAnn: creatorID,
				},
			},
			Spec: v3.ProjectSpec{
				DisplayName: "Default",
				Description: "Default project created for the cluster",
				ClusterName: obj.Name,
			},
		})

		return obj, err
	})
}

func (m *mgr) reconcileCreatorRTB(obj runtime.Object) (runtime.Object, error) {
	return v3.CreatorMadeOwner.Do(obj, func() (runtime.Object, error) {
		metaAccessor, err := meta.Accessor(obj)
		if err != nil {
			return obj, err
		}

		typeAccessor, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		creatorID, ok := metaAccessor.GetAnnotations()[creatorIDAnn]
		if !ok {
			logrus.Warnf("%v %v has no creatorId annotation. Cannot add creator as owner", typeAccessor.GetKind(), metaAccessor.GetName())
			return obj, nil
		}

		rtbName := "creator"
		om := v1.ObjectMeta{
			Name:      rtbName,
			Namespace: metaAccessor.GetName(),
		}
		subject := v13.Subject{
			Kind: "User",
			Name: creatorID,
		}

		switch typeAccessor.GetKind() {
		case v3.ProjectGroupVersionKind.Kind:
			if rtb, _ := m.prtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
				return obj, nil
			}
			if _, err := m.mgmt.Management.ProjectRoleTemplateBindings(metaAccessor.GetName()).Create(&v3.ProjectRoleTemplateBinding{
				ObjectMeta:       om,
				ProjectName:      metaAccessor.GetNamespace() + ":" + metaAccessor.GetName(),
				RoleTemplateName: "project-owner",
				Subject:          subject,
			}); err != nil {
				return obj, err
			}
		case v3.ClusterGroupVersionKind.Kind:
			if rtb, _ := m.crtbLister.Get(metaAccessor.GetName(), rtbName); rtb != nil {
				return obj, nil
			}
			if _, err := m.mgmt.Management.ClusterRoleTemplateBindings(metaAccessor.GetName()).Create(&v3.ClusterRoleTemplateBinding{
				ObjectMeta:       om,
				ClusterName:      metaAccessor.GetName(),
				RoleTemplateName: "cluster-owner",
				Subject:          subject,
			}); err != nil {
				return obj, err
			}
		}

		return obj, nil
	})
}

func (m *mgr) deleteNamespace(obj runtime.Object) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return condition.Error("MissingMetadata", err)
	}

	nsClient := m.mgmt.K8sClient.CoreV1().Namespaces()
	ns, err := nsClient.Get(o.GetName(), v1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		err = nsClient.Delete(o.GetName(), nil)
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func (m *mgr) reconcileResourceToNamespace(obj runtime.Object) (runtime.Object, error) {
	return v3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
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
			_, err := nsClient.Create(&v12.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: o.GetName(),
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			})
			if err != nil {
				return obj, condition.Error("NamespaceCreationFailure", errors.Wrapf(err, "failed to create namespace for %v %v", t.GetKind(), o.GetName()))
			}
		}

		return obj, nil
	})
}
