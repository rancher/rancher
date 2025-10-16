// Package impersonation sets up service accounts that are permitted to act on behalf of a Rancher user on a cluster.
package impersonation

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/controllers"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/types/config"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

const (
	impersonationLabel = "authz.cluster.cattle.io/impersonator"
	// ImpersonationNamespace is the namespace where impersonation service accounts live.
	ImpersonationNamespace = "cattle-impersonation-system"
	// ImpersonationPrefix is the prefix for impersonation roles, bindings, and service accounts.
	ImpersonationPrefix = "cattle-impersonation-"
)

// impersonator implements the config.Impersonator interface: user impersonations against a certain cluster
type impersonator struct {
	userLister               v3.UserLister
	userAttributeLister      v3.UserAttributeLister
	secretsCache             corecontrollers.SecretCache
	namespaceCache           corecontrollers.NamespaceCache
	svcAccountCache          corecontrollers.ServiceAccountCache
	svcAccountClient         corecontrollers.ServiceAccountClient
	namespaceClient          corecontrollers.NamespaceClient
	clusterRoleCache         rbaccontrollers.ClusterRoleCache
	clusterRoleClient        rbaccontrollers.ClusterRoleClient
	clusterRoleBindingCache  rbaccontrollers.ClusterRoleBindingCache
	clusterRoleBindingClient rbaccontrollers.ClusterRoleBindingClient
	// used in serviceaccounttoken.EnsureSecretForServiceAccount
	secretsGetter    clientv1.SecretsGetter
	svcAccountGetter clientv1.ServiceAccountsGetter
}

func newControllerFactory(clientFactory client.SharedClientFactory) controller.SharedControllerFactory {
	cacheFactory := cache.NewSharedCachedFactory(clientFactory, &cache.SharedCacheFactoryOptions{
		KindNamespace: map[schema.GroupVersionKind]string{
			corev1.SchemeGroupVersion.WithKind("Secret"):         ImpersonationNamespace,
			corev1.SchemeGroupVersion.WithKind("ServiceAccount"): ImpersonationNamespace,
		},
	})
	factoryOpts := controllers.GetOptsFromEnv(controllers.User)
	return controller.NewSharedControllerFactory(cacheFactory, factoryOpts)
}

// ForCluster creates a config.Impersonator (or returns an existing one) for a given clusterContext
func ForCluster(clusterContext *config.UserContext) (config.Impersonator, error) {
	if clusterContext.Impersonator != nil {
		return clusterContext.Impersonator, nil
	}

	// Use a dedicated cache factory, restricting cached secrets and svcaccounts only to the impersonation namespace
	factory := newControllerFactory(clusterContext.ControllerFactory.SharedCacheFactory().SharedClientFactory())
	if err := clusterContext.RegisterExtraControllerFactory("impersonation", factory); err != nil {
		return nil, fmt.Errorf("registering impersonation controller factory: %w", err)
	}
	dedicatedCoreFactory := corecontrollers.New(factory)

	clusterContext.Impersonator = &impersonator{
		userLister:               clusterContext.Management.Management.Users("").Controller().Lister(),
		userAttributeLister:      clusterContext.Management.Management.UserAttributes("").Controller().Lister(),
		namespaceCache:           clusterContext.Corew.Namespace().Cache(),
		namespaceClient:          clusterContext.Corew.Namespace(),
		secretsCache:             dedicatedCoreFactory.Secret().Cache(),
		svcAccountCache:          dedicatedCoreFactory.ServiceAccount().Cache(),
		svcAccountClient:         clusterContext.Corew.ServiceAccount(),
		clusterRoleCache:         clusterContext.RBACw.ClusterRole().Cache(),
		clusterRoleClient:        clusterContext.RBACw.ClusterRole(),
		clusterRoleBindingCache:  clusterContext.RBACw.ClusterRoleBinding().Cache(),
		clusterRoleBindingClient: clusterContext.RBACw.ClusterRoleBinding(),
		secretsGetter:            clusterContext.K8sClient.CoreV1(),
		svcAccountGetter:         clusterContext.K8sClient.CoreV1(),
	}

	return clusterContext.Impersonator, nil
}

func (i *impersonator) SetUpImpersonation(userInfo user.Info) error {
	_, err := i.setup(userInfo)
	return err
}

// setup ensures all necessary resources are created and returns the ServiceAccount whose token can be used for impersonation.
func (i *impersonator) setup(userInfo user.Info) (*corev1.ServiceAccount, error) {
	userInfo, err := i.getUser(userInfo)
	if err != nil {
		return nil, err
	}
	name := ImpersonationPrefix + userInfo.GetUID()
	rules := i.rulesForUser(userInfo)
	logrus.Tracef("impersonation: checking role for user %s", userInfo.GetName())
	role, err := i.checkAndUpdateRole(name, rules)
	if err != nil {
		return nil, err
	}
	roleBinding, err := i.getRoleBinding(name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if role != nil && roleBinding != nil {
		sa, err := i.getServiceAccount(name)
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
	logrus.Tracef("impersonation: creating role for user %s", userInfo.GetName())
	role, err = i.createRole(name, rules)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: creating service account for user %s", userInfo.GetName())
	sa, err := i.createServiceAccount(name, role)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: creating role binding for user %s", userInfo.GetName())
	err = i.createRoleBinding(name, role, sa)
	if err != nil {
		return nil, err
	}
	logrus.Tracef("impersonation: waiting for service account to become active for user %s", userInfo.GetName())
	return i.waitForServiceAccount(sa)
}

// GetToken accepts a service account and returns the service account's token.
func (i *impersonator) GetToken(userInfo user.Info) (string, error) {
	sa, err := i.setup(userInfo)
	if err != nil {
		return "", fmt.Errorf("error setting up impersonation for user %s: %w", userInfo.GetUID(), err)
	}

	secret, err := serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), i.secretsCache, i.secretsGetter, i.svcAccountGetter, sa)
	if err != nil {
		return "", fmt.Errorf("error getting secret: %w", err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("error getting token: invalid secret object")
	}
	return string(token), nil
}

func (i *impersonator) getServiceAccount(name string) (*corev1.ServiceAccount, error) {
	sa, err := i.svcAccountCache.Get(ImpersonationNamespace, name)
	if err != nil {
		if logrus.GetLevel() >= logrus.TraceLevel {
			logrus.Tracef("impersonation: error getting service account %s/%s: %v", ImpersonationNamespace, name, err)
			sas, debugErr := i.svcAccountCache.List(ImpersonationNamespace, labels.NewSelector())
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

func (i *impersonator) createServiceAccount(name string, role *rbacv1.ClusterRole) (*corev1.ServiceAccount, error) {
	sa, err := i.svcAccountCache.Get(ImpersonationNamespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("impersonation: error getting service account [%s:%s]: %w", ImpersonationNamespace, name, err)
	}
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating service account %s", name)
		sa, err = i.svcAccountClient.Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ImpersonationNamespace,
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
			sa, err = i.svcAccountClient.Get(ImpersonationNamespace, name, metav1.GetOptions{})
		}
		if err != nil {
			return nil, fmt.Errorf("impersonation: error getting service account [%s:%s]: %w", ImpersonationNamespace, name, err)
		}
	}
	// create secret for service account if it was not automatically generated
	_, err = serviceaccounttoken.EnsureSecretForServiceAccount(context.Background(), i.secretsCache, i.secretsGetter, i.svcAccountGetter, sa)
	if err != nil {
		return nil, fmt.Errorf("impersonation: error ensuring secret for service account %s: %w", name, err)
	}
	return sa, nil
}

func (i *impersonator) createNamespace() error {
	_, err := i.namespaceCache.Get(ImpersonationNamespace)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating namespace %s", ImpersonationNamespace)
		_, err = i.namespaceClient.Create(&corev1.Namespace{
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
func (i *impersonator) checkAndUpdateRole(name string, rules []rbacv1.PolicyRule) (*rbacv1.ClusterRole, error) {
	var role *rbacv1.ClusterRole
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		role, err = i.clusterRoleCache.Get(name)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(role.Rules, rules) {
			role.Rules = rules
			role, err = i.clusterRoleClient.Update(role)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return role, nil
}

func (i *impersonator) createRole(name string, rules []rbacv1.PolicyRule) (*rbacv1.ClusterRole, error) {
	role, err := i.clusterRoleCache.Get(name)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating role %s", name)
		role, err = i.clusterRoleClient.Create(&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					impersonationLabel: "true",
				},
			},
			Rules:           rules,
			AggregationRule: nil,
		})
		if apierrors.IsAlreadyExists(err) {
			// in case cache isn't synced yet, use raw client
			return i.clusterRoleClient.Get(name, metav1.GetOptions{})
		}
		return role, nil
	}
	return role, err
}

func (i *impersonator) rulesForUser(userInfo user.Info) []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"users"},
			ResourceNames: []string{userInfo.GetUID()},
		},
		{
			Verbs:     []string{"impersonate"},
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"userextras/" + common.ExtraRequestTokenID},
			// Note that to avoid constantly updating the ClusterRole we allow all values here.
			// The check for the token ownership is done by the impersonation authenticator.
		},
		{
			Verbs:     []string{"impersonate"},
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"userextras/" + common.ExtraRequestHost},
		},
	}

	if groups := userInfo.GetGroups(); len(groups) > 0 {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{""},
			Resources:     []string{"groups"},
			ResourceNames: groups,
		})
	}

	extras := userInfo.GetExtra()
	if principalids, ok := extras[common.UserAttributePrincipalID]; ok {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/" + common.UserAttributePrincipalID},
			ResourceNames: principalids,
		})
	}
	if usernames, ok := extras[common.UserAttributeUserName]; ok {
		rules = append(rules, rbacv1.PolicyRule{
			Verbs:         []string{"impersonate"},
			APIGroups:     []string{"authentication.k8s.io"},
			Resources:     []string{"userextras/" + common.UserAttributeUserName},
			ResourceNames: usernames,
		})
	}

	return rules
}

func (i *impersonator) getRoleBinding(name string) (*rbacv1.ClusterRoleBinding, error) {
	return i.clusterRoleBindingCache.Get(name)
}

func (i *impersonator) createRoleBinding(name string, role *rbacv1.ClusterRole, sa *corev1.ServiceAccount) error {
	_, err := i.clusterRoleBindingCache.Get(name)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("impersonation: creating role binding %s", name)
		_, err = i.clusterRoleBindingClient.Create(&rbacv1.ClusterRoleBinding{
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

func (i *impersonator) waitForServiceAccount(sa *corev1.ServiceAccount) (*corev1.ServiceAccount, error) {
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
		ret, err = i.svcAccountCache.Get(ImpersonationNamespace, sa.Name)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		secret, err := serviceaccounttoken.ServiceAccountSecret(context.Background(), sa, i.secretsCache.List, i.secretsGetter.Secrets(sa.Namespace))
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
			sas, debugErr := i.svcAccountCache.List(ImpersonationNamespace, labels.NewSelector())
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

func (i *impersonator) getUser(userInfo user.Info) (user.Info, error) {
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
			extras[common.UserAttributeUserName] = []string{u.DisplayName}
		}
		if len(u.PrincipalIDs) > 0 {
			extras[common.UserAttributePrincipalID] = u.PrincipalIDs
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
			if usernames, ok := exs[common.UserAttributeUserName]; ok && len(usernames) > 0 {
				if _, ok := extras[common.UserAttributeUserName]; !ok {
					extras[common.UserAttributeUserName] = make([]string, 0)
				}
				extras[common.UserAttributeUserName] = append(extras[common.UserAttributeUserName], usernames...)
			}
			if principalids, ok := exs[common.UserAttributePrincipalID]; ok && len(principalids) > 0 {
				if _, ok := extras[common.UserAttributePrincipalID]; !ok {
					extras[common.UserAttributePrincipalID] = make([]string, 0)
				}
				extras[common.UserAttributePrincipalID] = append(extras[common.UserAttributePrincipalID], principalids...)
			}
		}
	}
	// sort to make comparable
	sort.Strings(groups)
	if _, ok := extras[common.UserAttributeUserName]; ok {
		sort.Strings(extras[common.UserAttributeUserName])
	}
	if _, ok := extras[common.UserAttributePrincipalID]; ok {
		sort.Strings(extras[common.UserAttributePrincipalID])
	}

	return &user.DefaultInfo{
		UID:    u.GetName(),
		Name:   u.Username,
		Groups: groups,
		Extra:  extras,
	}, nil
}

func isInList(item string, list []string) bool {
	for _, s := range list {
		if item == s {
			return true
		}
	}
	return false
}
