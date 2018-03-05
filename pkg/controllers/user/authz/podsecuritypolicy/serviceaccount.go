package podsecuritypolicy

import (
	"fmt"
	"strings"

	v13 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/extensions/v1beta1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	v12 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	v1beta12 "k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// RegisterServiceAccount ensures that:
// 	1. Each namespace has a pod security policy assigned to a role if:
//		a. its project has a PSPT assigned to it
//		OR
//		b. its cluster has a default PSPT assigned to it
//  2. PSPs are bound to their associated service accounts via a cluster role binding
func RegisterServiceAccount(context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy serviceaccount handler for cluster %v", context.ClusterName)

	m := &serviceAccountManager{
		clusters: context.Management.Management.Clusters(""),
		bindings: context.RBAC.RoleBindings(""),
		policies: context.Extensions.PodSecurityPolicies(""),
		roles:    context.RBAC.ClusterRoles(""),

		clusterLister:   context.Management.Management.Clusters("").Controller().Lister(),
		templateLister:  context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:    context.Extensions.PodSecurityPolicies("").Controller().Lister(),
		bindingLister:   context.RBAC.RoleBindings("").Controller().Lister(),
		roleLister:      context.RBAC.ClusterRoles("").Controller().Lister(),
		namespaceLister: context.Core.Namespaces("").Controller().Lister(),
		projectLister:   context.Management.Management.Projects("").Controller().Lister(),
	}

	context.Core.ServiceAccounts("").AddHandler("ServiceAccountSyncHandler", m.sync)
}

type serviceAccountManager struct {
	clusterLister   v3.ClusterLister
	clusters        v3.ClusterInterface
	templateLister  v3.PodSecurityPolicyTemplateLister
	policyLister    v1beta1.PodSecurityPolicyLister
	bindingLister   v12.RoleBindingLister
	bindings        v12.RoleBindingInterface
	policies        v1beta1.PodSecurityPolicyInterface
	roleLister      v12.ClusterRoleLister
	roles           v12.ClusterRoleInterface
	namespaceLister v13.NamespaceLister
	projectLister   v3.ProjectLister
}

func (m *serviceAccountManager) sync(key string, obj *v1.ServiceAccount) error {
	if obj == nil {
		logrus.Debugf("no service account provided, exiting")
		return nil
	}

	namespaceName := obj.Namespace

	// get PSPT
	// if no PSPT then get default
	// if no default then exit
	namespace, err := m.namespaceLister.Get("", namespaceName)
	if err != nil {
		return fmt.Errorf("error getting namespaces: %v", err)
	}

	annotation := namespace.Annotations[projectIDAnnotation]

	if annotation == "" {
		// no project is associated with namespace so don't do anything
		logrus.Debugf("no project is associated with namespace %v", namespaceName)
		return nil
	}

	split := strings.SplitN(annotation, ":", 2)

	if len(split) != 2 {
		return fmt.Errorf("could not parse handler key: %v got len %v", annotation, len(split))
	}

	clusterName, projectID := split[0], split[1]

	podSecurityPolicyTemplateID, err := getPodSecurityPolicyTemplateID(m.projectLister, m.clusterLister, projectID,
		clusterName)
	if err != nil {
		return err
	}

	if podSecurityPolicyTemplateID == "" {
		// Do nothing
		logrus.Debugf("no matching pod security policy template")
		return nil
	}

	policies, err := m.policyLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting policies: %v", err)
	}

	var policy *v1beta12.PodSecurityPolicy

	for _, candidate := range policies {
		if candidate.Annotations[podSecurityTemplateParentAnnotation] == podSecurityPolicyTemplateID {
			policy = candidate
		}
	}

	if policy == nil {
		template, err := m.templateLister.Get("", podSecurityPolicyTemplateID)
		if err != nil {
			return fmt.Errorf("error getting pod security policy templates: %v", err)
		}

		policy, err = fromTemplate(m.policies, m.policyLister, key, template)
		if err != nil {
			return err
		}
	}

	roles, err := m.roleLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting cluster roles: %v", err)
	}

	var role *rbac.ClusterRole

	for _, candidate := range roles {
		if candidate.Annotations[podSecurityTemplateParentAnnotation] == policy.Name {
			role = candidate
		}
	}

	if role == nil {
		// Create role
		newRole := &rbac.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Annotations:  map[string]string{},
				GenerateName: "clusterrole-",
				Namespace:    namespaceName,
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterRole",
			},
			Rules: []rbac.PolicyRule{
				{
					APIGroups:     []string{"extensions"},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
					ResourceNames: []string{policy.Name},
				},
			},
		}

		newRole.Annotations[podSecurityTemplateParentAnnotation] = policy.Name

		role, err = m.roles.Create(newRole)
		if err != nil {
			return fmt.Errorf("error creating cluster role: %v", err)
		}
	}

	return createBindingIfNotExists(m.bindings, m.bindingLister, obj, role.Name, policy.Name)
}

func createBindingIfNotExists(bindings2 v12.RoleBindingInterface, bindingLister v12.RoleBindingLister,
	serviceAccount *v1.ServiceAccount, roleName string, policyName string) error {
	bindings, err := bindingLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting bindings: %v", err)
	}

	bindingExists := false

	for _, binding := range bindings {
		if binding.Annotations[podSecurityTemplateParentAnnotation] == policyName &&
			len(binding.Subjects) > 0 && // guard against panic
			binding.Subjects[0].Name == serviceAccount.Name &&
			binding.Subjects[0].Namespace == serviceAccount.Namespace {
			bindingExists = true
		}
	}

	if !bindingExists {
		// create binding
		err = createBinding(bindings2, serviceAccount, roleName, policyName)
		if err != nil {
			return err
		}
	}

	return nil
}

func createBinding(bindings v12.RoleBindingInterface, serviceAccount *v1.ServiceAccount, roleName string,
	policyName string) error {
	newBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v-binding", roleName, policyName),
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				podSecurityTemplateParentAnnotation: policyName,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterRoleBinding",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: apiGroup,
			Name:     roleName,
			Kind:     "ClusterRole",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
	}

	_, err := bindings.Create(newBinding)
	if err != nil {
		return fmt.Errorf("error creating role binding for %v error was: %v", newBinding, err)
	}

	return nil
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
