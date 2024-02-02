package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	v12 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v13 "k8s.io/api/core/v1"
	v1beta13 "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const namespaceByProjectNameIndex = "podsecuritypolicy.rbac.user.cattle.io/by-project-name"

// RegisterBindings updates the pod security policy for this binding if it has been changed.  Also resync service
// accounts so they pick up the change.  If no policy exists then exits without doing anything.
func RegisterBindings(ctx context.Context, context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy project handler for cluster %v", context.ClusterName)

	namespaceInformer := context.Core.Namespaces("").Controller().Informer()
	namespaceIndexers := map[string]cache.IndexFunc{
		namespaceByProjectNameIndex: namespaceByProjectName,
	}
	namespaceInformer.AddIndexers(namespaceIndexers)

	lifecycle := &lifecycle{
		policyLister:         context.Policy.PodSecurityPolicies("").Controller().Lister(),
		policies:             context.Policy.PodSecurityPolicies(""),
		psptLister:           context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		clusterLister:        context.Management.Management.Clusters("").Controller().Lister(),
		clusterRoleLister:    context.RBAC.ClusterRoles("").Controller().Lister(),
		clusterRoles:         context.RBAC.ClusterRoles(""),
		serviceAccountLister: context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccounts:      context.Core.ServiceAccounts("").Controller(),

		namespaces:       context.Core.Namespaces(""),
		namespaceIndexer: namespaceInformer.GetIndexer(),
		clusterName:      context.ClusterName,
	}

	context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").
		AddLifecycle(ctx, "PodSecurityPolicyTemplateProjectBindingsLifecycleHandler", lifecycle)
}

func namespaceByProjectName(obj interface{}) ([]string, error) {
	namespace, ok := obj.(*v13.Namespace)
	if !ok || namespace.Annotations[projectIDAnnotation] == "" {
		return []string{}, nil
	}

	return []string{namespace.Annotations[projectIDAnnotation]}, nil
}

type lifecycle struct {
	policyLister         v1beta1.PodSecurityPolicyLister
	policies             v1beta1.PodSecurityPolicyInterface
	psptLister           v3.PodSecurityPolicyTemplateLister
	clusterRoleLister    v12.ClusterRoleLister
	clusterRoles         v12.ClusterRoleInterface
	serviceAccountLister v1.ServiceAccountLister
	serviceAccounts      v1.ServiceAccountController

	namespaces       v1.NamespaceInterface
	namespaceIndexer cache.Indexer
	clusterName      string
	clusterLister    v3.ClusterLister
}

func (l *lifecycle) Create(obj *v3.PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	return l.sync(obj)
}

func (l *lifecycle) Updated(obj *v3.PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	return l.sync(obj)
}

func (l *lifecycle) Remove(obj *v3.PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	return obj, l.syncNamespacesInProject(obj.TargetProjectName)
}

func (l *lifecycle) sync(obj *v3.PodSecurityPolicyTemplateProjectBinding) (runtime.Object, error) {
	if obj.PodSecurityPolicyTemplateName == "" {
		return obj, nil
	}

	err := CheckClusterVersion(l.clusterName, l.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for PodSecurityPolicyTemplateProjectBinding controller: %w", err)
	}

	podSecurityPolicyName := fmt.Sprintf("%v-psp", obj.PodSecurityPolicyTemplateName)
	_, err = l.policyLister.Get("", podSecurityPolicyName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = l.createPolicy(obj, podSecurityPolicyName)
			if err != nil {
				return nil, fmt.Errorf("error creating policy: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error getting policy: %v", err)
		}
	}

	clusterRoleName := fmt.Sprintf("%v-clusterrole", obj.PodSecurityPolicyTemplateName)
	_, err = l.clusterRoleLister.Get("", clusterRoleName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			_, err = l.createClusterRole(clusterRoleName, podSecurityPolicyName, obj)
			if err != nil {
				return nil, fmt.Errorf("error creating cluster role: %v", err)
			}
		} else {
			return nil, fmt.Errorf("error getting cluster role: %v", err)
		}
	}

	return obj, l.syncNamespacesInProject(obj.TargetProjectName)
}

func (l *lifecycle) createPolicy(obj *v3.PodSecurityPolicyTemplateProjectBinding,
	podSecurityPolicyName string) (*v1beta13.PodSecurityPolicy, error) {
	template, err := l.psptLister.Get("", obj.PodSecurityPolicyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("error getting pspt: %v", err)
	}

	objectMeta := metav1.ObjectMeta{}
	objectMeta.Name = podSecurityPolicyName
	objectMeta.Annotations = make(map[string]string)
	objectMeta.Annotations[podSecurityPolicyTemplateParentAnnotation] = template.Name
	objectMeta.Annotations[podSecurityPolicyTemplateVersionAnnotation] = template.ResourceVersion

	// Setting annotations that doesn't contains podSecurityPolicyTemplateFilterAnnotation
	for k, v := range template.Annotations {
		if !strings.Contains(k, podSecurityPolicyTemplateFilterAnnotation) {
			objectMeta.Annotations[k] = v
		}
	}

	psp := &v1beta13.PodSecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       podSecurityPolicy,
			APIVersion: apiVersion,
		},
		ObjectMeta: objectMeta,
		Spec:       template.Spec,
	}

	return l.policies.Create(psp)
}

func (l *lifecycle) createClusterRole(clusterRoleName string, podSecurityPolicyName string,
	obj *v3.PodSecurityPolicyTemplateProjectBinding) (*rbac.ClusterRole, error) {
	newRole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
			Name:        clusterRoleName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterRole",
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups:     []string{"extensions"},
				Resources:     []string{"podsecuritypolicies"},
				Verbs:         []string{"use"},
				ResourceNames: []string{podSecurityPolicyName},
			},
		},
	}
	newRole.Annotations[podSecurityPolicyTemplateParentAnnotation] = obj.PodSecurityPolicyTemplateName

	return l.clusterRoles.Create(newRole)
}

func (l *lifecycle) syncNamespacesInProject(projectName string) error {
	namespaces, err := l.namespaceIndexer.ByIndex(namespaceByProjectNameIndex, projectName)
	if err != nil {
		return fmt.Errorf("error getting namespaces")
	}

	for _, rawNamespace := range namespaces {
		namespace, ok := rawNamespace.(*v13.Namespace)
		if !ok {
			return fmt.Errorf("error converting to namespace: %v", rawNamespace)
		}

		l.namespaces.Controller().Enqueue("", namespace.Name)
	}

	return nil
}
