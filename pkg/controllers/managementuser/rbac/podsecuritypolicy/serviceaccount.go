package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"

	v13 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/policy/v1beta1"
	v12 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const psptpbByTargetProjectNameAnnotationIndex = "podsecuritypolicy.rbac.user.cattle.io/psptpb-by-project-id"
const roleBindingByServiceAccountIndex = "podsecuritypolicy.rbac.user.cattle.io/role-binding-by-service-account"
const psptpbRoleBindingAnnotation = "podsecuritypolicy.rbac.user.cattle.io/psptpb-role-binding"

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	psptpbInformer := scaledContext.Management.PodSecurityPolicyTemplateProjectBindings("").Controller().Informer()
	psptpbIndexers := map[string]cache.IndexFunc{
		psptpbByTargetProjectNameAnnotationIndex: psptpbByTargetProjectName,
	}
	return psptpbInformer.AddIndexers(psptpbIndexers)
}

// RegisterServiceAccount ensures that:
//  1. Each namespace has a pod security policy assigned to a role if:
//     a. its project has a PSPT assigned to it
//     OR
//     b. its cluster has a default PSPT assigned to it
//  2. PSPs are bound to their associated service accounts via a cluster role binding
func RegisterServiceAccount(ctx context.Context, context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy serviceaccount handler for cluster %v", context.ClusterName)

	psptpbInformer := context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").Controller().Informer()

	roleBindingInformer := context.RBAC.RoleBindings("").Controller().Informer()
	roleBindingIndexers := map[string]cache.IndexFunc{
		roleBindingByServiceAccountIndex: roleBindingByServiceAccount,
	}
	roleBindingInformer.AddIndexers(roleBindingIndexers)

	m := &serviceAccountManager{
		clusterName:        context.ClusterName,
		pspts:              context.Management.Management.PodSecurityPolicyTemplates(""),
		roleBindings:       context.RBAC.RoleBindings(""),
		roleBindingIndexer: roleBindingInformer.GetIndexer(),

		policies:      context.Policy.PodSecurityPolicies(""),
		psptpbIndexer: psptpbInformer.GetIndexer(),

		clusterLister:     context.Management.Management.Clusters("").Controller().Lister(),
		psptLister:        context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		templateLister:    context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:      context.Policy.PodSecurityPolicies("").Controller().Lister(),
		roleBindingLister: context.RBAC.RoleBindings("").Controller().Lister(),
		roleLister:        context.RBAC.ClusterRoles("").Controller().Lister(),
		namespaceLister:   context.Core.Namespaces("").Controller().Lister(),
		projectLister:     context.Management.Management.Projects(context.ClusterName).Controller().Lister(),
		psptpbLister: context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").
			Controller().Lister(),
	}

	context.Core.ServiceAccounts("").AddHandler(ctx, "ServiceAccountLifecycleHandler", m.sync)
}

func psptpbByTargetProjectName(obj interface{}) ([]string, error) {
	psptpb, ok := obj.(*v3.PodSecurityPolicyTemplateProjectBinding)
	if !ok || psptpb.TargetProjectName == "" {
		return []string{}, nil
	}

	return []string{psptpb.TargetProjectName}, nil
}

func roleBindingByServiceAccount(obj interface{}) ([]string, error) {
	roleBinding, ok := obj.(*rbac.RoleBinding)
	if !ok || len(roleBinding.Subjects) != 1 ||
		roleBinding.Subjects[0].Name == "" ||
		roleBinding.Subjects[0].Namespace == "" {
		return []string{}, nil
	}

	subject := roleBinding.Subjects[0]
	return []string{subject.Namespace + "-" + subject.Name}, nil
}

type serviceAccountManager struct {
	clusterName        string
	clusterLister      v3.ClusterLister
	pspts              v3.PodSecurityPolicyTemplateInterface
	psptLister         v3.PodSecurityPolicyTemplateLister
	psptpbIndexer      cache.Indexer
	templateLister     v3.PodSecurityPolicyTemplateLister
	policyLister       v1beta1.PodSecurityPolicyLister
	roleBindingLister  v12.RoleBindingLister
	roleBindings       v12.RoleBindingInterface
	roleBindingIndexer cache.Indexer
	policies           v1beta1.PodSecurityPolicyInterface
	roleLister         v12.ClusterRoleLister
	namespaceLister    v13.NamespaceLister
	projectLister      v3.ProjectLister
	psptpbLister       v3.PodSecurityPolicyTemplateProjectBindingLister
}

func (m *serviceAccountManager) sync(key string, obj *v1.ServiceAccount) (runtime.Object, error) {
	if obj == nil {
		// do nothing
		return nil, nil
	}

	err := CheckClusterVersion(m.clusterName, m.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for ServiceAccount controller: %w", err)
	}

	namespace, err := m.namespaceLister.Get("", obj.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting projects: %v", err)
	}

	var psptpbs []interface{}

	if namespace.Annotations[projectIDAnnotation] != "" {
		psptpbs, err = m.psptpbIndexer.ByIndex(psptpbByTargetProjectNameAnnotationIndex, namespace.Annotations[projectIDAnnotation])
		if err != nil {
			return nil, fmt.Errorf("error getting psptpbs: %v", err)
		}
	}

	onePSPTPBExists := false
	desiredBindings := map[string]*v3.PodSecurityPolicyTemplateProjectBinding{}

	for _, rawPSPTPB := range psptpbs {
		psptpb, ok := rawPSPTPB.(*v3.PodSecurityPolicyTemplateProjectBinding)
		if !ok {
			return nil, fmt.Errorf("could not convert to *v3.PodSecurityPolicyTemplateProjectBinding: %v", rawPSPTPB)
		}

		if psptpb.DeletionTimestamp != nil {
			continue
		}

		onePSPTPBExists = true

		key := getClusterRoleName(psptpb.PodSecurityPolicyTemplateName)
		desiredBindings[key] = psptpb
	}

	originalDesiredBindingsLen := len(desiredBindings)

	roleBindings, err := m.roleBindingIndexer.ByIndex(roleBindingByServiceAccountIndex, obj.Namespace+"-"+obj.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting role bindings: %v", err)
	}

	cluster, err := m.clusterLister.Get("", m.clusterName)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %v", err)
	}

	for _, rawRoleBinding := range roleBindings {
		roleBinding, ok := rawRoleBinding.(*rbac.RoleBinding)
		if !ok {
			return nil, fmt.Errorf("could not convert to *rbac2.RoleBinding: %v", rawRoleBinding)
		}

		key := roleBinding.RoleRef.Name

		if desiredBindings[key] == nil && okToDelete(obj, roleBinding, namespace, cluster, originalDesiredBindingsLen) {
			err = m.roleBindings.DeleteNamespaced(roleBinding.Namespace, roleBinding.Name, &metav1.DeleteOptions{})
			if err != nil {
				return nil, fmt.Errorf("error deleting role binding: %v", err)
			}
		} else {
			delete(desiredBindings, key)
		}
	}

	for clusterRoleName, desiredBinding := range desiredBindings {
		roleBindingName := getRoleBindingName(obj, clusterRoleName)
		_, err = m.roleBindings.Create(&rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleBindingName,
				Namespace: obj.Namespace,
				Annotations: map[string]string{
					podSecurityPolicyTemplateParentAnnotation: desiredBinding.PodSecurityPolicyTemplateName,
					psptpbRoleBindingAnnotation:               "true",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Name:       obj.Name,
						Kind:       "ServiceAccount",
						UID:        obj.UID,
					},
				},
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "RoleBinding",
			},
			RoleRef: rbac.RoleRef{
				APIGroup: apiGroup,
				Name:     clusterRoleName,
				Kind:     "ClusterRole",
			},
			Subjects: []rbac.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      obj.Name,
					Namespace: obj.Namespace,
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("error creating binding: %v", err)
		}
	}

	if !onePSPTPBExists && namespace.Annotations[projectIDAnnotation] != "" {
		// create default pspt role binding if it is set
		clusterRoleName := getClusterRoleName(cluster.Spec.DefaultPodSecurityPolicyTemplateName)
		roleBindingName := getDefaultRoleBindingName(obj, clusterRoleName)

		if cluster.Spec.DefaultPodSecurityPolicyTemplateName != "" {
			_, err := m.roleBindingLister.Get(obj.Namespace, roleBindingName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					_, err = m.roleBindings.Create(&rbac.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleBindingName,
							Namespace: obj.Namespace,
							Annotations: map[string]string{
								podSecurityPolicyTemplateParentAnnotation: cluster.Spec.DefaultPodSecurityPolicyTemplateName,
								psptpbRoleBindingAnnotation:               "true",
							},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "v1",
									Name:       obj.Name,
									Kind:       "ServiceAccount",
									UID:        obj.UID,
								},
							},
						},
						TypeMeta: metav1.TypeMeta{
							Kind: "RoleBinding",
						},
						RoleRef: rbac.RoleRef{
							APIGroup: apiGroup,
							Name:     clusterRoleName,
							Kind:     "ClusterRole",
						},
						Subjects: []rbac.Subject{
							{
								Kind:      "ServiceAccount",
								Name:      obj.Name,
								Namespace: obj.Namespace,
							},
						},
					})
					if err != nil {
						return nil, fmt.Errorf("error creating role binding: %v", err)
					}
				} else {
					return nil, fmt.Errorf("error getting role binding %v: %v", roleBindingName, err)
				}
			}
		}
	}

	return nil, nil
}

func okToDelete(svcAct *v1.ServiceAccount, rb *rbac.RoleBinding, namespace *v1.Namespace, cluster *v3.Cluster,
	originalDesiredBindingsLen int) bool {
	// This is not a role binding this logic should manage so exit immediately
	if rb.Annotations[psptpbRoleBindingAnnotation] == "" {
		return false
	}

	// Namespace isn't in a project so it should have no role bindings
	if namespace.Annotations[projectIDAnnotation] == "" {
		return true
	}

	// No default PSPT is set so its ok to delete this if its a normal rolebinding or a leftover default PSPT binding
	if cluster.Spec.DefaultPodSecurityPolicyTemplateName == "" {
		return true
	}

	// at least one PSPTPB exists so we need to delete all default PSPT bindings
	if originalDesiredBindingsLen > 0 {
		return true
	}

	// the default PSPT has changed so we need to clean it up before creating the new one
	if getDefaultRoleBindingName(svcAct,
		getClusterRoleName(cluster.Spec.DefaultPodSecurityPolicyTemplateName)) != rb.Name {
		return true
	}

	return false
}

func getRoleBindingName(obj *v1.ServiceAccount, clusterRoleName string) string {
	return fmt.Sprintf("%v-%v-%v-binding", obj.Name, obj.Namespace, clusterRoleName)
}

func getDefaultRoleBindingName(obj *v1.ServiceAccount, clusterRoleName string) string {
	return fmt.Sprintf("default-%v-%v-%v-binding", obj.Name, obj.Namespace, clusterRoleName)
}

func getClusterRoleName(podSecurityPolicyTemplateName string) string {
	return fmt.Sprintf("%v-clusterrole", podSecurityPolicyTemplateName)
}

func resyncServiceAccounts(serviceAccountLister v13.ServiceAccountLister,
	serviceAccountController v13.ServiceAccountController, namespace string) error {
	serviceAccounts, err := serviceAccountLister.List(namespace, labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting service accounts: %v", err)
	}

	for _, account := range serviceAccounts {
		serviceAccountController.Enqueue(account.Namespace, account.Name)
	}

	return nil
}
