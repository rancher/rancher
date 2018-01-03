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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func newPandCLifecycles(management *config.ManagementContext) (*projectLifecycle, *clusterLifecycle) {
	m := &mgr{
		mgmt:       management,
		nsLister:   management.Core.Namespaces("").Controller().Lister(),
		prtbLister: management.Management.ProjectRoleTemplateBindings("").Controller().Lister(),
		crtbLister: management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
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
	return obj, nil
}

type clusterLifecycle struct {
	mgr *mgr
}

func (l *clusterLifecycle) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	if err != nil {
		return obj, err
	}

	_, err = l.mgr.reconcileCreatorRTB(obj)
	return obj, err
}

func (l *clusterLifecycle) Updated(obj *v3.Cluster) (*v3.Cluster, error) {
	_, err := l.mgr.reconcileResourceToNamespace(obj)
	return obj, err
}

func (l *clusterLifecycle) Remove(obj *v3.Cluster) (*v3.Cluster, error) {
	return obj, nil
}

type mgr struct {
	mgmt       *config.ManagementContext
	nsLister   corev1.NamespaceLister
	prtbLister v3.ProjectRoleTemplateBindingLister
	crtbLister v3.ClusterRoleTemplateBindingLister
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

		creatorID, ok := metaAccessor.GetAnnotations()["field.cattle.io/creatorId"]
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
					OwnerReferences: []v1.OwnerReference{
						{
							APIVersion: t.GetAPIVersion(),
							Kind:       t.GetKind(),
							Name:       o.GetName(),
							UID:        o.GetUID(),
						},
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
