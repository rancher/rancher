package auth

import (
	"fmt"
	"strings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers"
	"github.com/rancher/rancher/pkg/controllers/management/auth/project_cluster"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

type userLifecycle struct {
	prtb            wranglerv3.ProjectRoleTemplateBindingController
	crtb            wranglerv3.ClusterRoleTemplateBindingController
	grb             wranglerv3.GlobalRoleBindingController
	users           wranglerv3.UserController
	tokens          wranglerv3.TokenController
	namespaces      wcorev1.NamespaceController
	namespaceLister wcorev1.NamespaceCache
	secrets         wcorev1.SecretController
	secretsLister   wcorev1.SecretCache
	prtbLister      wranglerv3.ProjectRoleTemplateBindingCache
	crtbLister      wranglerv3.ClusterRoleTemplateBindingCache
	grbLister       wranglerv3.GlobalRoleBindingCache
	prtbIndexer     cache.Indexer
	crtbIndexer     cache.Indexer
	grbIndexer      cache.Indexer
	tokenIndexer    cache.Indexer
	userManager     user.Manager
	clusterLister   wranglerv3.ClusterCache
	clusterManager  *clustermanager.Manager
	extTokenStore   *exttokenstore.SystemStore
}

const (
	crtbByUserRefKey  = "auth.management.cattle.io/crtb-by-user-ref"
	prtbByUserRefKey  = "auth.management.cattle.io/prtb-by-user-ref"
	grbByUserRefKey   = "auth.management.cattle.io/grb-by-user-ref"
	tokenByUserRefKey = "auth.management.cattle.io/token-by-user-ref"
	userController    = "mgmt-auth-users-controller"
)

func newUserLifecycle(management *config.ManagementContext, clusterManager *clustermanager.Manager) *userLifecycle {

	extTokenStore := exttokenstore.NewSystemFromWrangler(management.Wrangler)

	lfc := &userLifecycle{
		prtb:            management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
		crtb:            management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		grb:             management.Wrangler.Mgmt.GlobalRoleBinding(),
		users:           management.Wrangler.Mgmt.User(),
		tokens:          management.Wrangler.Mgmt.Token(),
		namespaces:      management.Wrangler.Core.Namespace(),
		secrets:         management.Wrangler.Core.Secret(),
		secretsLister:   management.Wrangler.Core.Secret().Cache(),
		namespaceLister: management.Wrangler.Core.Namespace().Cache(),
		userManager:     management.UserManager,
		clusterLister:   management.Wrangler.Mgmt.Cluster().Cache(),
		clusterManager:  clusterManager,
		extTokenStore:   extTokenStore,
	}

	prtbInformer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	lfc.prtbIndexer = prtbInformer.GetIndexer()

	crtbInformer := management.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	lfc.crtbIndexer = crtbInformer.GetIndexer()

	grbInformer := management.Management.GlobalRoleBindings("").Controller().Informer()
	lfc.grbIndexer = grbInformer.GetIndexer()

	tokenInformer := management.Management.Tokens("").Controller().Informer()
	lfc.tokenIndexer = tokenInformer.GetIndexer()

	return lfc
}

func grbByUserRefFunc(obj interface{}) ([]string, error) {
	globalRoleBinding, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{globalRoleBinding.UserName}, nil
}

func prtbByUserRefFunc(obj interface{}) ([]string, error) {
	projectRoleBinding, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok || projectRoleBinding.UserName == "" {
		return []string{}, nil
	}

	return []string{projectRoleBinding.UserName}, nil
}

func crtbByUserRefFunc(obj interface{}) ([]string, error) {
	clusterRoleBinding, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok || clusterRoleBinding.UserName == "" {
		return []string{}, nil
	}

	return []string{clusterRoleBinding.UserName}, nil
}

func tokenByUserRefFunc(obj interface{}) ([]string, error) {
	token, ok := obj.(*v3.Token)
	if !ok {
		return []string{}, nil
	}

	return []string{token.UserID}, nil
}

// hasLocalPrincipalID returns true in case the user
// has at least one PrincipalID that starts with "local://".
// Returns false otherwise.
func hasLocalPrincipalID(user *v3.User) bool {
	var match = false
	for _, id := range user.PrincipalIDs {
		if strings.HasPrefix(id, "local://") {
			match = true
			break
		}
	}
	return match
}

// Create creates a new user role binding and sets the Status.Conditions.Type = "InitialRolesPopulated",
// and then returns the object. Otherwise returns an error.
func (l *userLifecycle) Create(user *v3.User) (runtime.Object, error) {
	if !hasLocalPrincipalID(user) {
		user.PrincipalIDs = append(user.PrincipalIDs, "local://"+user.Name)
	}

	// creatorIDAnn indicates it was created through the API, create the new
	// user bindings and add the annotation UserConditionInitialRolesPopulated
	if user.ObjectMeta.Annotations[project_cluster.CreatorIDAnnotation] != "" {
		u, err := v32.UserConditionInitialRolesPopulated.DoUntilTrue(user, func() (runtime.Object, error) {
			err := l.userManager.CreateNewUserClusterRoleBinding(user.Name, user.UID)
			if err != nil {
				return nil, err
			}
			return user, nil
		})
		if err != nil {
			return nil, err
		}
		user = u.(*v3.User)
	}

	return user, nil
}

func (l *userLifecycle) Updated(user *v3.User) (runtime.Object, error) {
	err := l.userManager.CreateNewUserClusterRoleBinding(user.Name, user.UID)
	if err != nil {
		return nil, err
	}

	if user.Enabled != nil && !*user.Enabled {
		// New functionality. Not done for norman tokens, here. See refresher.go which
		// deletes/disables tokens when a user is disabled. Having it here makes it a more
		// immediate response to the user's change of status.

		extTokens, err := l.getExtTokensByUserName(user.Name)
		if err != nil {
			return nil, err
		}

		for _, token := range extTokens {
			if token.GetIsDerived() {
				// Non-login token. Only disable it
				logrus.Infof("[%v] Disabling ext token %v for user %v",
					userController, token.GetName(), token.GetUserID())
				token.Spec.Enabled = pointer.Bool(false)
				_, err := l.extTokenStore.Update(token, &metav1.UpdateOptions{})
				if err != nil {
					return nil, fmt.Errorf("error updating ext token: %v", err)
				}
			} else {
				// Login token. Delete it.
				logrus.Infof("[%v] Deleting token %v for user %v",
					userController, token.GetName(), token.GetUserID())
				err := l.extTokenStore.Delete(token.GetName(), &metav1.DeleteOptions{})
				if err != nil {
					return nil, fmt.Errorf("error deleting ext token: %v", err)
				}
			}
		}
	}

	return user, nil
}

func (l *userLifecycle) Remove(user *v3.User) (runtime.Object, error) {
	clusterRoles, err := l.getCRTBByUserName(user.Name)
	if err != nil {
		return nil, err
	}

	err = l.deleteAllCRTB(clusterRoles)
	if err != nil {
		return nil, err
	}

	projectRoles, err := l.getPRTBByUserName(user.Name)
	if err != nil {
		return nil, err
	}

	err = l.deleteAllPRTB(projectRoles)
	if err != nil {
		return nil, err
	}

	globalRoles, err := l.getGRBByUserName(user.Name)
	if err != nil {
		return nil, err
	}

	err = l.deleteAllGRB(globalRoles)
	if err != nil {
		return nil, err
	}

	tokens, err := l.getTokensByUserName(user.Name)
	if err != nil {
		return nil, err
	}
	extTokens, err := l.getExtTokensByUserName(user.Name)
	if err != nil {
		return nil, err
	}

	err = l.deleteClusterUserAttributes(user.Name, tokens)
	if err != nil {
		return nil, err
	}

	err = l.deleteAllTokens(tokens)
	if err != nil {
		return nil, err
	}
	err = l.deleteAllExtTokens(extTokens)
	if err != nil {
		return nil, err
	}

	err = l.deleteUserNamespace(user.Name)
	if err != nil {
		return nil, err
	}

	err = l.deleteUserSecret(user.Name)
	if err != nil {
		return nil, err
	}

	user, err = l.removeLegacyFinalizers(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (l *userLifecycle) getCRTBByUserName(username string) ([]*v3.ClusterRoleTemplateBinding, error) {
	obj, err := l.crtbIndexer.ByIndex(crtbByUserRefKey, username)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster roles: %v", err)
	}

	var crtbs []*v3.ClusterRoleTemplateBinding
	for _, o := range obj {
		crtb, ok := o.(*v3.ClusterRoleTemplateBinding)
		if !ok {
			return nil, fmt.Errorf("error converting obj to cluster role template binding: %v", o)
		}

		crtbs = append(crtbs, crtb)
	}

	return crtbs, nil
}

func (l *userLifecycle) getPRTBByUserName(username string) ([]*v3.ProjectRoleTemplateBinding, error) {
	objs, err := l.prtbIndexer.ByIndex(prtbByUserRefKey, username)
	if err != nil {
		return nil, fmt.Errorf("error getting indexed project roles: %v", err)
	}

	var prtbs []*v3.ProjectRoleTemplateBinding
	for _, obj := range objs {
		prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
		if !ok {
			return nil, fmt.Errorf("could not convert obj to v3.ProjectRoleTemplateBinding")
		}

		prtbs = append(prtbs, prtb)
	}

	return prtbs, nil
}

func (l *userLifecycle) getGRBByUserName(username string) ([]*v3.GlobalRoleBinding, error) {
	objs, err := l.grbIndexer.ByIndex(grbByUserRefKey, username)
	if err != nil {
		return nil, fmt.Errorf("error getting indexed global roles: %v", err)
	}

	var grbs []*v3.GlobalRoleBinding
	for _, obj := range objs {
		grb, ok := obj.(*v3.GlobalRoleBinding)
		if !ok {
			return nil, fmt.Errorf("could not convert obj to v3.GlobalRoleBinding")
		}

		grbs = append(grbs, grb)
	}

	return grbs, nil
}

func (l *userLifecycle) getTokensByUserName(username string) ([]*v3.Token, error) {
	objs, err := l.tokenIndexer.ByIndex(tokenByUserRefKey, username)
	if err != nil {
		return nil, fmt.Errorf("error getting indexed tokens: %v", err)
	}

	var tokens []*v3.Token
	for _, obj := range objs {
		token, ok := obj.(*v3.Token)
		if !ok {
			return nil, fmt.Errorf("could not convert to *v3.Token: %v", obj)
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (l *userLifecycle) getExtTokensByUserName(username string) ([]*ext.Token, error) {
	filterForUser := labels.Set(map[string]string{
		exttokenstore.UserIDLabel: username,
	})

	objs, err := l.extTokenStore.List(&metav1.ListOptions{
		LabelSelector: filterForUser.AsSelector().String(),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting ext tokens for user %s: %v", username, err)
	}

	// objs.Items is []ext.Token, i.e a value slice, not a pointer slice.
	tokens := []*ext.Token{}
	for _, token := range objs.Items {
		tokens = append(tokens, &token)
	}

	return tokens, nil
}

func (l *userLifecycle) deleteAllCRTB(crtbs []*v3.ClusterRoleTemplateBinding) error {
	for _, crtb := range crtbs {
		var err error
		logrus.Infof("[%v] Deleting clusterRoleTemplateBinding %v for user %v", userController, crtb.Name, crtb.UserName)
		err = l.crtb.Delete(crtb.Namespace, crtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting cluster role: %v", err)
		}
	}

	return nil
}

func (l *userLifecycle) deleteAllPRTB(prtbs []*v3.ProjectRoleTemplateBinding) error {
	for _, prtb := range prtbs {
		var err error
		logrus.Infof("[%v] Deleting projectRoleTemplateBinding %v for user %v", userController, prtb.Name, prtb.UserName)
		err = l.prtb.Delete(prtb.Namespace, prtb.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting projet role: %v", err)
		}
	}

	return nil
}

func (l *userLifecycle) deleteAllGRB(grbs []*v3.GlobalRoleBinding) error {
	// some GRBs can refer to GRs which inherit cluster Roles. Rancher's service account lacks the permission
	// to delete these GRBs directly, so it needs to bypass the webhook
	grbClient, err := l.grb.WithImpersonation(controllers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("error when impersonating webhook to delete globalRoleBindings: %w", err)
	}
	for _, grb := range grbs {
		logrus.Infof("[%v] Deleting globalRoleBinding %v for user %v", userController, grb.Name, grb.UserName)
		err = grbClient.Delete(grb.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting globalRoleBinding %v: %v", grb.Name, err)

		}
	}

	return nil
}

func (l *userLifecycle) deleteClusterUserAttributes(username string, tokens []*v3.Token) error {
	if len(tokens) == 0 {
		return nil
	}
	// find the set of clusters associated with a list of tokens
	set := make(map[string]*v3.Cluster)
	for _, token := range tokens {
		cluster, err := l.clusterLister.Get(token.ClusterName)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}
		set[token.ClusterName] = cluster
	}

	for _, cluster := range set {
		userCtx, err := l.clusterManager.UserContextNoControllers(cluster.Name)
		if err != nil {
			return err
		}
		err = userCtx.Cluster.ClusterUserAttributes("cattle-system").Delete(username, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (l *userLifecycle) deleteAllTokens(tokens []*v3.Token) error {
	for _, token := range tokens {
		logrus.Infof("[%v] Deleting token %v for user %v", userController, token.Name, token.UserID)
		err := l.tokens.Delete(token.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting token: %v", err)
		}
	}

	return nil
}

func (l *userLifecycle) deleteAllExtTokens(tokens []*ext.Token) error {
	for _, token := range tokens {
		logrus.Infof("[%v] Deleting token %v for user %v",
			userController, token.GetName(), token.GetUserID())
		err := l.extTokenStore.Delete(token.GetName(), &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting ext token: %v", err)
		}
	}

	return nil
}

func (l *userLifecycle) deleteUserNamespace(username string) error {
	namespace, err := l.namespaceLister.Get(username)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // nothing to delete
		}

		return fmt.Errorf("error getting user namespace: %v", err)
	}

	if namespace.Status.Phase == v12.NamespaceTerminating {
		return nil // nothing to do namespace is already deleting
	}

	logrus.Infof("[%v] Deleting namespace backing user %v", userController, username)
	err = l.namespaces.Delete(username, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting user namespace: %v", err)
	}

	return nil
}

func (l *userLifecycle) deleteUserSecret(username string) error {
	_, err := l.secretsLister.Get("cattle-system", username+"-secret")
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error getting user secret: %v", err)
	}

	logrus.Infof("[%v] Deleting secret backing user %v", userController, username)
	return l.secrets.Delete("cattle-system", username+"-secret", &metav1.DeleteOptions{})
}

func (l *userLifecycle) removeLegacyFinalizers(user *v3.User) (*v3.User, error) {
	finalizers := user.GetFinalizers()
	for i, finalizer := range finalizers {
		if finalizer == "controller.cattle.io/cat-user-controller" {
			finalizers = append(finalizers[:i], finalizers[i+1:]...)
			user = user.DeepCopy()
			user.SetFinalizers(finalizers)
			updatedUser, err := l.users.Update(user)
			if err != nil {
				return nil, err
			}
			return updatedUser, err
		}
	}
	return user, nil
}
