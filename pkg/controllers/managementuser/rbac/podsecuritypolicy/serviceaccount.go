package podsecuritypolicy

import (
	"fmt"

	v13 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v12 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const psptpbByTargetProjectNameAnnotationIndex = "podsecuritypolicy.rbac.user.cattle.io/psptpb-by-project-id"
const roleBindingByServiceAccountIndex = "podsecuritypolicy.rbac.user.cattle.io/role-binding-by-service-account"
const psptpbRoleBindingAnnotation = "podsecuritypolicy.rbac.user.cattle.io/psptpb-role-binding"

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
	clusterName   string
	clusterLister v3.ClusterLister
	psptpbIndexer cache.Indexer

	roleBindingLister  v12.RoleBindingLister
	roleBindings       v12.RoleBindingInterface
	roleBindingIndexer cache.Indexer

	roleLister      v12.ClusterRoleLister
	namespaceLister v13.NamespaceLister
	projectLister   v3.ProjectLister
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
