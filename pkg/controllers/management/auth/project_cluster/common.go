package project_cluster

import (
	"context"
	"fmt"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	CreatorIDAnnotation             = "field.cattle.io/creatorId"
	NoCreatorRBACAnnotation         = "field.cattle.io/no-creator-rbac"
	creatorPrincipalNameAnnotation  = "field.cattle.io/creator-principal-name"
	creatorOwnerBindingAnnotation   = "authz.management.cattle.io/creator-owner-binding"
	roleTemplatesRequiredAnnotation = "authz.management.cattle.io/creator-role-bindings"
	clusterNameLabel                = "cluster.cattle.io/name"
)

var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func deleteNamespace(obj runtime.Object, controller string, nsClient v1.NamespaceInterface) error {
	o, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("[%s] error accessing object %v: %w", controller, obj, err)
	}

	ns, err := nsClient.Get(context.TODO(), o.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%s] Deleting namespace %s", controller, o.GetName())
		err = nsClient.Delete(context.TODO(), o.GetName(), metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func reconcileResourceToNamespace(obj runtime.Object, controller string, nsLister corev1.NamespaceLister, nsClient v1.NamespaceInterface) (runtime.Object, error) {
	return apisv3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		o, err := meta.Accessor(obj)
		if err != nil {
			return obj, fmt.Errorf("[%s] error accessing object %v: %w", controller, obj, err)
		}
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		ns, _ := nsLister.Get("", o.GetName())
		if ns == nil {
			logrus.Infof("[%v] Creating namespace %v", controller, o.GetName())
			_, err := nsClient.Create(context.TODO(), &v12.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.GetName(),
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return obj, fmt.Errorf("[%s] failed to create namespace for %v %v: %w", controller, t.GetKind(), o.GetName(), err)
			}
		}

		return obj, nil
	})
}

// createMembershipRoles creates 2 cluster roles: an owner role and a member role. To be used to create project/cluster membership roles.
func createMembershipRoles(obj runtime.Object, resourceType string, crClient crbacv1.ClusterRoleController) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("error accessing object %v: %w", obj, err)
	}
	typeInfo, err := meta.TypeAccessor(obj)
	if err != nil {
		return fmt.Errorf("error accessing object type %v: %w", obj, err)
	}

	rules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{resourceType},
			ResourceNames: []string{accessor.GetName()},
			Verbs:         []string{"get"},
		},
	}

	objectMeta := metav1.ObjectMeta{
		Name: accessor.GetName() + "-member",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: typeInfo.GetAPIVersion(),
				Kind:       typeInfo.GetKind(),
				Name:       accessor.GetName(),
				UID:        accessor.GetUID(),
			},
		},
	}

	if resourceType == "cluster" {
		objectMeta.Annotations = map[string]string{clusterNameLabel: accessor.GetName()}
	}

	memberRole := &rbacv1.ClusterRole{
		ObjectMeta: objectMeta,
		Rules:      rules,
	}
	if _, err := crClient.Create(memberRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	ownerRole := memberRole
	ownerRole.Name = accessor.GetName() + "-owner"
	ownerRole.Rules[0].Verbs = []string{"*"}
	if _, err := crClient.Create(ownerRole); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
