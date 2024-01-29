// Package impersonation sets up service accounts that are permitted to act on behalf of a Rancher user on a cluster.
package impersonation

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	authcommon "github.com/rancher/rancher/pkg/auth/providers/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/types/config"
	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"
)

const (
	impersonationLabel = "authz.cluster.cattle.io/impersonator"
	// ImpersonationNamespace is the namespace where impersonation service accounts live.
	ImpersonationNamespace = "cattle-impersonation-system"
	// ImpersonationPrefix is the prefix for impersonation roles, bindings, and service accounts.
	ImpersonationPrefix = "cattle-impersonation-"
)

// Impersonator contains data for the user being impersonated.
type Impersonator struct {
	user                user.Info
	clusterContext      *config.UserContext
	userLister          v3.UserLister
	userAttributeLister v3.UserAttributeLister
	secretsCache        corecontrollers.SecretCache
}

// New creates an Impersonator from a kubernetes user.Info object and a UserContext for the cluster.
func New(userInfo user.Info, clusterContext *config.UserContext) (Impersonator, error) {
	impersonator := Impersonator{
		clusterContext:      clusterContext,
		userLister:          clusterContext.Management.Management.Users("").Controller().Lister(),
		userAttributeLister: clusterContext.Management.Management.UserAttributes("").Controller().Lister(),
		secretsCache:        clusterContext.Corew.Secret().Cache(),
	}
	user, err := impersonator.getUser(userInfo)
	impersonator.user = user
	if err != nil {
		return Impersonator{}, err
	}

	return impersonator, nil
}

// SetUpImpersonation creates a service account on a cluster with a clusterrole and clusterrolebinding allowing it to impersonate a Rancher user.
// Returns a reference to the service account, which can be used by GetToken to retrieve the account token, or an error if creating any of the resources failed.
func (i *Impersonator) SetUpImpersonation() (*corev1.ServiceAccount, error) {
	rules := i.rulesForUser()
	logrus.Tracef("impersonation: checking role for user %s", i.user.GetName())
	role, err := i.checkAndUpdateRole(rules)
	if err != nil {
		return nil, err
	}
	roleBinding, err := i.getRoleBinding()
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if role != nil && roleBinding != nil {
		sa, err := i.getServiceAccount()
		// in case the role exists but we were interrupted before creating the service account, proceed to create resources
		if err == nil || !apierrors.IsNotFound(err) {
			return sa, err
		}
	}
	logrus.Tracef("impersonation: creating impersonation namespace")
	err = i.createNamespace()
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: creating role for user %s", i.user.GetName())
	role, err = i.createRole(rules)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: creating service account for user %s", i.user.GetName())
	sa, err := i.createServiceAccount(role)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: creating role binding for user %s", i.user.GetName())
	err = i.createRoleBinding(role, sa)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: waiting for service account to become active for user %s", i.user.GetName())
	return i.waitForServiceAccount(sa)
}

// GetToken accepts a service account and returns the service account's token.
func (i *Impersonator) GetToken(sa *corev1.ServiceAccount) (string, error) {
	secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), i.secretsCache, i.clusterContext.K8sClient, sa)
	if err != nil {
		return "", fmt.Errorf("error getting secret: %w", err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("error getting token: invalid secret object")
	}
	return string(token), nil
}

func (i *Impersonator) getServiceAccount() (*corev1.ServiceAccount, error) {
	name := ImpersonationPrefix + i.user.GetUID()
	sa, err := i.clusterContext.Core.ServiceAccounts("").Controller().Lister().Get(ImpersonationNamespace, name)
	if err != nil {
		if logrus.GetLevel() >= logrus.TraceLevel {
			logrus.Tracef("impersonation: error getting service account %s/%s: %v", ImpersonationNamespace, name, err)
			sas, debugErr := i.clusterContext.Core.ServiceAccounts("").Controller().Lister().List(ImpersonationNamespace, labels.NewSelector())
			if i.clusterContext == nil {
				logrus.Tracef("impersonation: cluster context is empty")
			} else {
				logrus.Tracef("impersonation: using context for cluster %s", i.clusterContext.ClusterName)
			}
			if debugErr != nil {
				logrus.Tracef("impersonation: encountered error listing cached service accounts: %v", debugErr)
			} else {
				logrus.Tracef("impersonation: cached service accounts: %+v", sas)
			}
		}
		return nil, fmt.Errorf("failed to get service account: %s/%s, error: %w", ImpersonationNamespace, name, err)
	}
	return sa, nil
}

func (i *Impersonator) createServiceAccount(role *rbacv1.ClusterRole) (*corev1.ServiceAccount, error) {
	name := ImpersonationPrefix + i.user.GetUID()
	sa, err := i.clusterContext.Core.ServiceAccounts("").Controller().Lister().Get(ImpersonationNamespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("impersonation: error getting service account [%s:%s]: %w", ImpersonationNamespace, name, err)
	}
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating service account %s", name)
		sa, err = i.clusterContext.Core.ServiceAccounts(ImpersonationNamespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					impersonationLabel: "true",
				},
				// Use the clusterrole as the owner for the purposes of automatic cleanup
				OwnerReferences: []metav1.OwnerReference{{
					Name:       role.Name,
					UID:        role.UID,
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				}},
			},
		})
		if apierrors.IsAlreadyExists(err) {
			// in case cache isn't synced yet, use raw client
			sa, err = i.clusterContext.Core.ServiceAccounts(ImpersonationNamespace).Get(name, metav1.GetOptions{})
		}
		if err != nil {
			return nil, fmt.Errorf("impersonation: error getting service account [%s:%s]: %w", ImpersonationNamespace, name, err)
		}
	}
	// create secret for service account if it was not automatically generated
	_, err = serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), i.secretsCache, i.clusterContext.K8sClient, sa)
	if err != nil {
		return nil, fmt.Errorf("impersonation: error ensuring secret for service account %s: %w", name, err)
	}
	return sa, nil
}

func (i *Impersonator) createNamespace() error {
	_, err := i.clusterContext.Core.Namespaces("").Controller().Lister().Get("", ImpersonationNamespace)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating namespace %s", ImpersonationNamespace)
		_, err = i.clusterContext.Core.Namespaces("").Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImpersonationNamespace,
				Labels: map[string]string{
					impersonationLabel: "true",
				},
			},
		})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
	}
	return err
}

// checkAndUpdateRole checks whether the impersonation clusterrole already exists and whether it has the correct rules.
// If the role does not exist, the method returns nil for the role and createRole must be called.
// If the role does exist, the rules are updated if necessary and a reference to the role is returned.
func (i *Impersonator) checkAndUpdateRole(rules []rbacv1.PolicyRule) (*rbacv1.ClusterRole, error) {
	name := ImpersonationPrefix + i.user.GetUID()
	var role *rbacv1.ClusterRole
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		role, err = i.clusterContext.RBAC.ClusterRoles("").Controller().Lister().Get("", name)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(role.Rules, rules) {
			role.Rules = rules
			role, err = i.clusterContext.RBAC.ClusterRoles("").Update(role)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (i *Impersonator) createRole(rules []rbacv1.PolicyRule) (*rbacv1.ClusterRole, error) {
	name := ImpersonationPrefix + i.user.GetUID()
	role, err := i.clusterContext.RBAC.ClusterRoles("").Controller().Lister().Get("", name)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating role %s", name)
		role, err = i.clusterContext.RBAC.ClusterRoles("").Create(&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImpersonationPrefix + i.user.GetUID(),
				Labels: map[string]string{
					impersonationLabel: "true",
				},
			},
			Rules:           rules,
			AggregationRule: nil,
		})
		if apierrors.IsAlreadyExists(err) {
			// in case cache isn't synced yet, use raw client
			return i.clusterContext.RBAC.ClusterRoles("").Get(name, metav1.GetOptions{})
		}
		return role, nil
	}
	return role, err
}

func (i *Impersonator) rulesForUser() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{{
		Verbs:         []string{"impersonate"},
		APIGroups:     []string{""},
		Resources:     []string{"users"},
		ResourceNames: []string{i.user.GetUID()},
	}}

	if groups := i.user.GetGroups(); len(groups) > 0 {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"groups"},
			ResourceNames: groups,
		})
	}
	extras := i.user.GetExtra()
	if principalids, ok := extras[authcommon.UserAttributePrincipalID]; ok {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/principalid"},
			ResourceNames: principalids,
		})
	}
	if usernames, ok := extras[authcommon.UserAttributeUserName]; ok {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/username"},
			ResourceNames: usernames,
		})
	}
	return rules
}

func (i *Impersonator) getRoleBinding() (*rbacv1.ClusterRoleBinding, error) {
	name := ImpersonationPrefix + i.user.GetUID()
	return i.clusterContext.RBAC.ClusterRoleBindings("").Controller().Lister().Get("", name)
}

func (i *Impersonator) createRoleBinding(role *rbacv1.ClusterRole, sa *corev1.ServiceAccount) error {
	name := ImpersonationPrefix + i.user.GetUID()
	_, err := i.clusterContext.RBAC.ClusterRoleBindings("").Controller().Lister().Get("", name)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating role binding %s", name)
		_, err = i.clusterContext.RBAC.ClusterRoleBindings("").Create(&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				// Use the clusterrole as the owner for the purposes of automatic cleanup
				OwnerReferences: []metav1.OwnerReference{{
					Name:       role.Name,
					UID:        role.UID,
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "ClusterRole",
				}},
				Labels: map[string]string{
					impersonationLabel: "true",
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					APIGroup:  "",
					Name:      sa.Name,
					Namespace: sa.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     role.Name,
			},
		})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
	}
	return err
}

func (i *Impersonator) waitForServiceAccount(sa *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
	logrus.Debugf("impersonation: waiting for service account %s/%s to be ready", sa.Namespace, sa.Name)
	backoff := wait.Backoff{
		Duration: 200 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    10,
	}
	var ret *corev1.ServiceAccount
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		ret, err = i.clusterContext.Core.ServiceAccounts("").Controller().Lister().Get(ImpersonationNamespace, sa.Name)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		secret, err := serviceaccounttoken.ServiceAccountSecret(context.Background(), sa, i.secretsCache.List, i.clusterContext.K8sClient.CoreV1().Secrets(sa.Namespace))
		if err != nil {
			return false, err
		}
		if secret == nil {
			return false, nil
		}
		if _, found := secret.Data[corev1.ServiceAccountTokenKey]; found {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		if logrus.GetLevel() >= logrus.TraceLevel {
			logrus.Tracef("impersonation: error waiting for service account %s/%s: %v", sa.Namespace, sa.Name, err)
			sas, debugErr := i.clusterContext.Core.ServiceAccounts("").Controller().Lister().List(ImpersonationNamespace, labels.NewSelector())
			if i.clusterContext == nil {
				logrus.Tracef("impersonation: cluster context is empty")
			} else {
				logrus.Tracef("impersonation: using context for cluster %s", i.clusterContext.ClusterName)
			}
			if debugErr != nil {
				logrus.Tracef("impersonation: encountered error listing cached service accounts: %v", debugErr)
			} else {
				logrus.Tracef("impersonation: cached service accounts: %+v", sas)
			}
		}
		return nil, fmt.Errorf("failed to get secret for service account: %s/%s, error: %w", sa.Namespace, sa.Name, err)
	}
	return ret, nil
}

func (i *Impersonator) getUser(userInfo user.Info) (user.Info, error) {
	u, err := i.userLister.Get("", userInfo.GetUID())
	if err != nil {
		return &user.DefaultInfo{}, err
	}

	groups := []string{"system:authenticated", "system:cattle:authenticated"}
	extras := make(map[string][]string)
	attribs, err := i.userAttributeLister.Get("", userInfo.GetUID())
	if err != nil && !apierrors.IsNotFound(err) {
		return &user.DefaultInfo{}, err
	}
	if attribs == nil { // system users do not have userattributes, but principalid and username are on the user
		// See https://github.com/rancher/rancher/blob/7ce603ea90ca656f5baa29b0149c19c8d7f73e8f/pkg/auth/requests/authenticate.go#L185-L194
		// If the extras are not in userattributes, use displayName and principalIDs from the user.
		if u.DisplayName != "" {
			extras[authcommon.UserAttributeUserName] = []string{u.DisplayName}
		}
		if len(u.PrincipalIDs) > 0 {
			extras[authcommon.UserAttributePrincipalID] = u.PrincipalIDs
		}
	} else { // real users have groups and extras in userattributes
		for _, gps := range attribs.GroupPrincipals {
			for _, groupPrincipal := range gps.Items {
				if !isInList(groupPrincipal.Name, groups) {
					groups = append(groups, groupPrincipal.Name)
				}
			}
		}
		for _, exs := range attribs.ExtraByProvider {
			if usernames, ok := exs[authcommon.UserAttributeUserName]; ok && len(usernames) > 0 {
				if _, ok := extras[authcommon.UserAttributeUserName]; !ok {
					extras[authcommon.UserAttributeUserName] = make([]string, 0)
				}
				extras[authcommon.UserAttributeUserName] = append(extras[authcommon.UserAttributeUserName], usernames...)
			}
			if principalids, ok := exs[authcommon.UserAttributePrincipalID]; ok && len(principalids) > 0 {
				if _, ok := extras[authcommon.UserAttributePrincipalID]; !ok {
					extras[authcommon.UserAttributePrincipalID] = make([]string, 0)
				}
				extras[authcommon.UserAttributePrincipalID] = append(extras[authcommon.UserAttributePrincipalID], principalids...)
			}
		}
	}
	// sort to make comparable
	sort.Strings(groups)
	if _, ok := extras[authcommon.UserAttributeUserName]; ok {
		sort.Strings(extras[authcommon.UserAttributeUserName])
	}
	if _, ok := extras[authcommon.UserAttributePrincipalID]; ok {
		sort.Strings(extras[authcommon.UserAttributePrincipalID])
	}

	user := &user.DefaultInfo{
		UID:    u.GetName(),
		Name:   u.Username,
		Groups: groups,
		Extra:  extras,
	}
	return user, nil
}

func isInList(item string, list []string) bool {
	for _, s := range list {
		if item == s {
			return true
		}
	}
	return false
}
