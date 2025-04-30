package project_cluster

import (
	"context"
	"fmt"
	"strings"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	psautils "github.com/rancher/rancher/pkg/controllers/management/auth/psautils"
	v32 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	crbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	rbackv1 "k8s.io/api/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

// createMembershipRoles creates 2 cluster roles: an owner role and a member role. To be used to create project/cluster membership roles.
func createMembershipRoles(obj runtime.Object, crClient crbacv1.ClusterRoleController) error {
	var resourceName, resourceType, context string
	var annotations map[string]string

	switch v := obj.(type) {
	case *apisv3.Project:
		context = projectContext
		resourceType = apisv3.ProjectResourceName
		resourceName = v.GetName()
	case *apisv3.Cluster:
		annotations = map[string]string{clusterNameLabel: v.GetName()}
		context = clusterContext
		resourceType = apisv3.ClusterResourceName
		resourceName = v.GetName()
	default:
		return fmt.Errorf("cannot create membership roles for unsupported type %T", v)
	}

	ownerRef, err := newOwnerReference(obj)
	if err != nil {
		return err
	}

	memberRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(resourceName, context, "member"),
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations:     annotations,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{resourceType},
				ResourceNames: []string{resourceName},
				Verbs:         []string{"get"},
			},
		},
	}

	ownerRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(resourceName, context, "owner"),
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations:     annotations,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
				Resources:     []string{resourceType},
				ResourceNames: []string{resourceName},
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

// checkPSAMembershipRole creates (if needed) an additional cluster role to grant updatepsa permissions, if needed.
func checkPSAMembershipRole(obj runtime.Object, crClient crbacv1.ClusterRoleController, prtbLister v32.ProjectRoleTemplateBindingCache, rtLister v32.RoleTemplateCache) error {
	var resourceName, resourceNamespace, resourceType, context string
	var annotations map[string]string

	switch v := obj.(type) {
	case *apisv3.Project:
		context = projectContext
		resourceType = apisv3.ProjectResourceName
		resourceName = v.GetName()
		resourceNamespace = v.Status.BackingNamespace
	default:
		return fmt.Errorf("cannot create membership roles for unsupported type %T", v)
	}

	ownerRef, err := newOwnerReference(obj)
	if err != nil {
		return err
	}

	psaNeeded, err := needsPSARole(resourceName, resourceNamespace, prtbLister, rtLister)
	if err != nil {
		return err
	}

	var psaRole *rbackv1.ClusterRole
	if psaNeeded {
		psaRole = &rbackv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name.SafeConcatName(resourceName, context, "psa"),
				OwnerReferences: []metav1.OwnerReference{ownerRef},
				Annotations:     annotations,
			},
			Rules: []rbackv1.PolicyRule{
				{
					APIGroups:     []string{apisv3.SchemeGroupVersion.Group},
					Resources:     []string{resourceType},
					ResourceNames: []string{resourceName},
					Verbs:         []string{psautils.UpdatepsaVerb},
				},
			},
		}
		if err := rbac.CreateOrUpdateResource(psaRole, crClient, rbac.AreClusterRolesSame); err != nil {
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

// needsPSARole ensure that given the project name, it needs a psa role to work properly.
func needsPSARole(projectName, projectNamespace string, prtbLister v32.ProjectRoleTemplateBindingCache, rtLister v32.RoleTemplateCache) (bool, error) {
	// lookup the roletemplate(s) associated with the project name
	// to check if those contain updatepsa verb
	roleTemplates, err := roleTemplatesLookup(projectName, projectNamespace, prtbLister, rtLister)
	if err != nil {
		return false, err
	}

	return psautils.IsPSAAllowed(roleTemplates), nil
}

// roleTemplatesLookup returns the list of roletemplates associated to the project.
func roleTemplatesLookup(projectName, projectNamespace string, prtbLister v32.ProjectRoleTemplateBindingCache, rtLister v32.RoleTemplateCache) ([]*v3.RoleTemplate, error) {
	// list all the prtbs in the project namespace
	prtbs, err := prtbLister.List(projectNamespace, labels.Everything())
	if err != nil {
		return nil, err
	}
	if len(prtbs) == 0 {
		return nil, fmt.Errorf("PRTBs not found")
	}

	// find all the roletemplate names associated to the prtbs
	var rtNames []string
	for _, prtb := range prtbs {
		var pName string
		// the ProjectName is expressed with the following format:
		// namespace:name, so we only need its name for comparison.
		projectInfo := strings.Split(prtb.ProjectName, ":")
		if len(projectInfo) != 2 {
			continue
		}
		pName = projectInfo[1]
		if pName == projectName {
			rtNames = append(rtNames, prtb.RoleTemplateName)
		}
	}
	if len(rtNames) == 0 {
		return nil, fmt.Errorf("RoleTemplates not found")
	}

	var roleTemplates []*v3.RoleTemplate
	for _, rtName := range rtNames {
		roleTemplate, err := rtLister.Get(rtName)
		if err != nil {
			return nil, err
		}
		roleTemplates = append(roleTemplates, roleTemplate)
	}
	return roleTemplates, nil
}
