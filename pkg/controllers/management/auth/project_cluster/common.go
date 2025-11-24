package project_cluster

import (
	"context"
	"fmt"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
	projectContext                  = "project"
	clusterContext                  = "cluster"
)

var crtbCreatorOwnerAnnotations = map[string]string{creatorOwnerBindingAnnotation: "true"}

func deleteNamespace(controller string, nsName string, nsClient v1.NamespaceInterface) error {
	ns, err := nsClient.Get(context.TODO(), nsName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if ns.Status.Phase != v12.NamespaceTerminating {
		logrus.Infof("[%s] Deleting namespace %s", controller, nsName)
		err = nsClient.Delete(context.TODO(), nsName, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}

func reconcileResourceToNamespace(obj runtime.Object, controller string, nsName string, nsLister corev1.NamespaceCache, nsClient v1.NamespaceInterface) (runtime.Object, error) {
	return apisv3.NamespaceBackedResource.Do(obj, func() (runtime.Object, error) {
		t, err := meta.TypeAccessor(obj)
		if err != nil {
			return obj, err
		}

		ns, _ := nsLister.Get(nsName)
		if ns == nil {
			logrus.Infof("[%v] Creating namespace %v", controller, nsName)
			_, err := nsClient.Create(context.TODO(), &v12.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
					Annotations: map[string]string{
						"management.cattle.io/system-namespace": "true",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return obj, fmt.Errorf("[%s] failed to create namespace for %s %s: %w", controller, t.GetKind(), nsName, err)
			}
		}

		return obj, nil
	})
}

func createProjectMembershipRoles(project *apisv3.Project, roleController crbacv1.RoleController) error {
	ownerRef, err := newOwnerReference(project)
	if err != nil {
		return err
	}

	memberRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(project.Name, projectContext+"member"),
			Namespace:       project.Spec.ClusterName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{apisv3.ProjectResourceName},
				ResourceNames: []string{project.Name},
				Verbs:         []string{"get"},
			},
		},
	}

	ownerRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(project.Name, projectContext+"owner"),
			Namespace:       project.Spec.ClusterName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{apisv3.ProjectResourceName},
				ResourceNames: []string{project.Name},
				Verbs:         []string{rbacv1.VerbAll},
			},
		},
	}
	for _, role := range []*rbacv1.Role{memberRole, ownerRole} {
		if err := rbac.CreateOrUpdateNamespacedResource(role, roleController, func(currentRole, wantedRole *rbacv1.Role) (bool, *rbacv1.Role) {
			return !equality.Semantic.DeepEqual(currentRole.Rules, wantedRole.Rules), wantedRole
		}); err != nil {
			return err
		}
	}

	return nil
}

// createClusterMembershipRoles creates 2 cluster roles: an owner role and a member role. Used to provide cluster membership.
func createClusterMembershipRoles(cluster *apisv3.Cluster, crClient crbacv1.ClusterRoleController) error {
	ownerRef, err := newOwnerReference(cluster)
	if err != nil {
		return err
	}

	memberRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(cluster.Name, clusterContext+"member"),
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations:     map[string]string{clusterNameLabel: cluster.Name},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{apisv3.ClusterResourceName},
				ResourceNames: []string{cluster.Name},
				Verbs:         []string{"get"},
			},
		},
	}

	ownerRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(cluster.Name, clusterContext+"owner"),
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations:     map[string]string{clusterNameLabel: cluster.Name},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{apisv3.ClusterResourceName},
				ResourceNames: []string{cluster.Name},
				Verbs:         []string{rbacv1.VerbAll},
			},
		},
	}

	for _, cr := range []*rbacv1.ClusterRole{memberRole, ownerRole} {
		if err := rbac.CreateOrUpdateResource(cr, crClient, rbac.AreClusterRolesSame); err != nil {
			return err
		}
	}

	return nil
}

// newOwnerReference create an OwnerReference from a runtime.Object.
func newOwnerReference(obj runtime.Object) (metav1.OwnerReference, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("error accessing object %v: %w", obj, err)
	}
	typeInfo, err := meta.TypeAccessor(obj)
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("error accessing object %v type %T: %w", obj, obj, err)
	}
	return metav1.OwnerReference{
		APIVersion: typeInfo.GetAPIVersion(),
		Kind:       typeInfo.GetKind(),
		Name:       metadata.GetName(),
		UID:        metadata.GetUID(),
	}, nil
}
