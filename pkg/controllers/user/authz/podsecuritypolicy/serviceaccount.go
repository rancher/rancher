package podsecuritypolicy

import (
	"fmt"
	"strings"
	"sync"

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
		bindings: context.RBAC.RoleBindings(""),
		policies: context.Extensions.PodSecurityPolicies(""),
		roles:    context.RBAC.ClusterRoles(""),

		clusterLister:   context.Management.Management.Clusters("").Controller().Lister(),
		templateLister:  context.Management.Management.PodSecurityPolicyTemplates("").Controller().Lister(),
		policyLister:    context.Extensions.PodSecurityPolicies("").Controller().Lister(),
		bindingLister:   context.RBAC.RoleBindings("").Controller().Lister(),
		roleLister:      context.RBAC.ClusterRoles("").Controller().Lister(),
		namespaceLister: context.Core.Namespaces("").Controller().Lister(),
		psptpbLister: context.Management.Management.PodSecurityPolicyTemplateProjectBindings("").
			Controller().Lister(),
	}

	context.Core.ServiceAccounts("").AddHandler("ServiceAccountSyncHandler", m.sync)
}

type serviceAccountManager struct {
	clusterLister   v3.ClusterLister
	templateLister  v3.PodSecurityPolicyTemplateLister
	policyLister    v1beta1.PodSecurityPolicyLister
	bindingLister   v12.RoleBindingLister
	bindings        v12.RoleBindingInterface
	policies        v1beta1.PodSecurityPolicyInterface
	roleLister      v12.ClusterRoleLister
	roles           v12.ClusterRoleInterface
	namespaceLister v13.NamespaceLister
	psptpbLister    v3.PodSecurityPolicyTemplateProjectBindingLister
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

	// instead of returning an error, i think you should log a warn level statement and return without error
	// if an namesapce is in the state and we return an error, it would cause an endless retry of all svc accounts in the ns
	// better to just log once and the next time the NS is updated (potentially fixing this annotation), these will get reprocessed
	if len(split) != 2 {
		return fmt.Errorf("could not parse handler key: %v got len %v", annotation, len(split))
	}

	clusterName, projectID := split[0], split[1]

	podSecurityPolicyTemplateID, err := getPodSecurityPolicyTemplateID(m.psptpbLister, m.clusterLister, projectID,
		clusterName)
	if err != nil {
		return err
	}

	if podSecurityPolicyTemplateID == "" {
		// Do nothing
		logrus.Debugf("no matching pod security policy template for %v", annotation)
		return nil
	}

	policies, err := m.policyLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting policies: %v", err)
	}

	var policy *v1beta12.PodSecurityPolicy

	for _, candidate := range policies {
		if candidate.Annotations[podSecurityTemplateParentAnnotation] == podSecurityPolicyTemplateID &&
			candidate.Annotations[podSecurityPolicyTemplateKey] == key {
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

	// can you call roleLister either clusterRoleLister or crLister to clarify what is for. role is a separate resource
	roles, err := m.roleLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting cluster roles: %v", err)
	}

	var role *rbac.ClusterRole

	// you just need a single role per policy, so can you just use a well-known name like role-<psp name> and then look up
	// by name instead of listing and filtering on annotation
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
				Namespace:    namespaceName, // clusterRoles are global scoped. they are not namespaced. drop this field
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

		// im hoping if you use a well known name for the clusterRole, you can eliminate this annotation
		newRole.Annotations[podSecurityTemplateParentAnnotation] = policy.Name

		role, err = m.roles.Create(newRole)
		if err != nil {
			return fmt.Errorf("error creating cluster role: %v", err)
		}
	}

	// BIG BUG? I dont see any logic for cleaning up old bindings that pointed at
	return createBindingIfNotExists(m.bindings, m.bindingLister, obj, role.Name, policy.Name)
}

var locker = &sync.Mutex{}

func createBindingIfNotExists(bindings2 v12.RoleBindingInterface, bindingLister v12.RoleBindingLister,
	serviceAccount *v1.ServiceAccount, roleName string, policyName string) error {

	bindings, err := bindingLister.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting bindings: %v", err)
	}

	bindingExists := false

	// Can we just give the binding a well known name like svcAccnt.name-clusterRole.Name and look it up by name?
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
	// BIG BUG: you are creating a binding with name <role-name>-<policy-name>-binding. But you give it a single subject of the current svc account
	// that will not work for greater than 1 service account. you need a binding per svc account but this basically limits you to exactly one
	// this should be fixed by addressing my previous comment of giving the binding a name of <svcAccount.Name-clusterRole.Name>.
	// I think this is going to mean completley refactoring what you are doing though because if you make that change, it has other implications like
	// how do you clean up (delet) old bindings
	newBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v-binding", roleName, policyName),
			Namespace: serviceAccount.Namespace,
			Annotations: map[string]string{
				podSecurityTemplateParentAnnotation: policyName,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterRoleBinding", // wrong type. should be RoleBinding (i think you dont even need this bc its implied by the fact that you are creating a &rbac.RoleBinding{
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
		return fmt.Errorf("error creating role binding: %v", err)
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
