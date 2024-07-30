package common

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/tokens"
	tokenUtil "github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v2/pkg/randomtoken"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

const (
	userAuthHeader               = "Impersonate-User"
	userByPrincipalIndex         = "auth.management.cattle.io/userByPrincipal"
	crtbsByPrincipalAndUserIndex = "auth.management.cattle.io/crtbByPrincipalAndUser"
	prtbsByPrincipalAndUserIndex = "auth.management.cattle.io/prtbByPrincipalAndUser"
	grbByUserIndex               = "auth.management.cattle.io/grbByUser"
	roleTemplatesRequired        = "authz.management.cattle.io/creator-role-bindings"
)

func NewUserManagerNoBindings(scaledContext *config.ScaledContext) (user.Manager, error) {
	userInformer := scaledContext.Management.Users("").Controller().Informer()
	userIndexers := map[string]cache.IndexFunc{
		userByPrincipalIndex: userByPrincipal,
	}
	if err := userInformer.AddIndexers(userIndexers); err != nil {
		return nil, err
	}

	return &userManager{
		users:       scaledContext.Management.Users(""),
		userIndexer: userInformer.GetIndexer(),
		tokens:      scaledContext.Management.Tokens(""),
		tokenLister: scaledContext.Management.Tokens("").Controller().Lister(),
		rbacClient:  scaledContext.RBAC,
	}, nil
}

var backoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   1,
	Jitter:   0,
	Steps:    7,
}

func NewUserManager(scaledContext *config.ScaledContext) (user.Manager, error) {
	userInformer := scaledContext.Management.Users("").Controller().Informer()
	userIndexers := map[string]cache.IndexFunc{
		userByPrincipalIndex: userByPrincipal,
	}
	if err := userInformer.AddIndexers(userIndexers); err != nil {
		return nil, err
	}

	crtbInformer := scaledContext.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		crtbsByPrincipalAndUserIndex: crtbsByPrincipalAndUser,
	}
	if err := crtbInformer.AddIndexers(crtbIndexers); err != nil {
		return nil, err
	}

	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbsByPrincipalAndUserIndex: prtbsByPrincipalAndUser,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return nil, err
	}

	grbInformer := scaledContext.Management.GlobalRoleBindings("").Controller().Informer()
	grbIndexers := map[string]cache.IndexFunc{
		grbByUserIndex: grbByUser,
	}
	if err := grbInformer.AddIndexers(grbIndexers); err != nil {
		return nil, err
	}

	return &userManager{
		manageBindings:           true,
		users:                    scaledContext.Management.Users(""),
		userIndexer:              userInformer.GetIndexer(),
		crtbIndexer:              crtbInformer.GetIndexer(),
		prtbIndexer:              prtbInformer.GetIndexer(),
		tokens:                   scaledContext.Management.Tokens(""),
		tokenLister:              scaledContext.Management.Tokens("").Controller().Lister(),
		globalRoleBindings:       scaledContext.Management.GlobalRoleBindings(""),
		globalRoleLister:         scaledContext.Management.GlobalRoles("").Controller().Lister(),
		grbIndexer:               grbInformer.GetIndexer(),
		clusterRoleLister:        scaledContext.RBAC.ClusterRoles("").Controller().Lister(),
		clusterRoleBindingLister: scaledContext.RBAC.ClusterRoleBindings("").Controller().Lister(),
		rbacClient:               scaledContext.RBAC,
	}, nil
}

type userManager struct {
	// manageBinding means whether or not we gr, grb, crtb, and prtb exist in the cluster
	manageBindings           bool
	users                    v3.UserInterface
	globalRoleBindings       v3.GlobalRoleBindingInterface
	globalRoleLister         v3.GlobalRoleLister
	grbIndexer               cache.Indexer
	userIndexer              cache.Indexer
	crtbIndexer              cache.Indexer
	prtbIndexer              cache.Indexer
	tokenLister              v3.TokenLister
	tokens                   v3.TokenInterface
	clusterRoleLister        rbacv1.ClusterRoleLister
	clusterRoleBindingLister rbacv1.ClusterRoleBindingLister
	rbacClient               rbacv1.Interface
}

func (m *userManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	userID := m.GetUser(apiContext)
	if userID == "" {
		return nil, errors.New("user not provided")
	}

	return m.SetPrincipalOnCurrentUserByUserID(userID, principal)
}

func (m *userManager) SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error) {
	user, err := m.users.Get(userID, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// ensure this principal is unique to this user
	if conflict, err := m.GetUserByPrincipalID(principal.Name); err != nil {
		return nil, err
	} else if conflict != nil && conflict.UID != user.UID {
		logrus.Errorf("refusing to set principal [%s] on user [%s], principal already in use on user [%s]", principal.Name, user.DisplayName, conflict.DisplayName)
		return user, errors.New("refusing to set principal on user that is already bound to another user")
	}

	if providerExists(user.PrincipalIDs, principal.Provider) {
		var principalIDs []string
		for _, id := range user.PrincipalIDs {
			if !strings.Contains(id, principal.Provider) {
				principalIDs = append(principalIDs, id)
			}
		}
		user.PrincipalIDs = principalIDs
	}

	if !slice.ContainsString(user.PrincipalIDs, principal.Name) {
		user.PrincipalIDs = append(user.PrincipalIDs, principal.Name)
		logrus.Infof("Updating user %v. Adding principal", user.Name)
		return m.users.Update(user)
	}
	return user, nil
}

func (m *userManager) GetUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}

// checkis if the supplied principal can login based on the accessMode and allowed principals
func (m *userManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error) {
	if accessMode == "unrestricted" || accessMode == "" {
		return true, nil
	}

	if accessMode == "required" || accessMode == "restricted" {
		user, err := m.checkCache(userPrincipalID)
		if err != nil {
			return false, err
		}

		userPrincipals := []string{userPrincipalID}
		if user != nil {
			for _, p := range user.PrincipalIDs {
				if userPrincipalID != p {
					userPrincipals = append(userPrincipals, p)
				}
			}
		}

		for _, p := range userPrincipals {
			if slice.ContainsString(allowedPrincipalIDs, p) {
				return true, nil
			}
		}

		for _, g := range groups {
			if slice.ContainsString(allowedPrincipalIDs, g.Name) {
				return true, nil
			}
		}

		if accessMode == "restricted" {
			// check if any of the user's principals are in a project or cluster
			var userNameAndPrincipals []string
			for _, g := range groups {
				userNameAndPrincipals = append(userNameAndPrincipals, g.Name)
			}
			if user != nil {
				userNameAndPrincipals = append(userNameAndPrincipals, user.Name)
				userNameAndPrincipals = append(userNameAndPrincipals, userPrincipals...)
			}

			return m.userExistsInClusterOrProject(userNameAndPrincipals)
		}
		return false, nil
	}
	return false, errors.Errorf("Unsupported accessMode: %v", accessMode)
}

// creates tokens with 0 ttl and returns token in 'token.Name:token.Token' format
func (m *userManager) EnsureToken(input user.TokenInput) (string, error) {
	return m.EnsureClusterToken("", input)
}

func (m *userManager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, error) {
	if strings.HasPrefix(input.TokenName, "token-") {
		return "", errors.New("token names can't start with token-")
	}

	var err error
	var token *v3.Token
	if !input.Randomize {
		token, err = m.tokenLister.Get("", input.TokenName)
		if err != nil && !apierrors.IsNotFound(err) {
			return "", err
		}
		if err == nil {
			if err := m.tokens.Delete(token.Name, &v1.DeleteOptions{}); err != nil {
				return "", err
			}
		}
	}

	key, err := randomtoken.Generate()
	if err != nil {
		return "", errors.New("failed to generate token key")
	}

	token = &v3.Token{
		ObjectMeta: v1.ObjectMeta{
			Name: input.TokenName,
			Labels: map[string]string{
				tokens.UserIDLabel:    input.UserName,
				tokens.TokenKindLabel: input.Kind,
			},
		},
		TTLMillis:     0,
		Description:   input.Description,
		UserID:        input.UserName,
		AuthProvider:  input.AuthProvider,
		UserPrincipal: input.UserPrincipal,
		IsDerived:     true,
		Token:         key,
		ClusterName:   clusterName,
	}
	if input.TTL != nil {
		token.TTLMillis = *input.TTL
	}
	if input.Randomize {
		token.ObjectMeta.Name = ""
		token.ObjectMeta.GenerateName = input.TokenName
	}
	err = tokens.ConvertTokenKeyToHash(token)
	if err != nil {
		return "", err
	}

	logrus.Infof("Creating token for user %v", input.UserName)
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		// Backoff was added here because it is possible the token is in the process of deleting.
		// This should cause the create to retry until the delete is finished.
		newToken, err := m.tokens.Create(token)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil
			}
			return false, err
		}
		token = newToken
		return true, nil
	})
	if err != nil {
		return "", err
	}

	return token.Name + ":" + key, nil
}

// newTokenForKubeconfig creates a new token for a generated kubeconfig.
func (m *userManager) newTokenForKubeconfig(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (string, error) {
	tokenTTL, err := tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds()
	if err != nil {
		return "", fmt.Errorf("failed to get default token TTL: %w", err)
	}

	input := user.TokenInput{
		TokenName:     tokenName,
		Description:   description,
		Kind:          kind,
		UserName:      userName,
		AuthProvider:  userPrincipal.Provider,
		TTL:           tokenTTL,
		Randomize:     true,
		UserPrincipal: userPrincipal,
	}

	return m.EnsureClusterToken(clusterName, input)
}

// GetKubeconfigToken creates a new token for use in a kubeconfig generated through the CLI.
func (m *userManager) GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (*v3.Token, string, error) {
	fullCreatedToken, err := m.newTokenForKubeconfig(clusterName, tokenName, description, kind, userName, userPrincipal)
	if err != nil {
		return nil, "", err
	}

	randomizedTokenName, createdTokenValue := tokens.SplitTokenParts(fullCreatedToken)
	token, err := m.tokens.Get(randomizedTokenName, v1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, createdTokenValue, err
	}

	if token.ExpiresAt != "" {
		return token, createdTokenValue, nil
	}

	// SetTokenExpiresAt requires creationTS, so can only be set post create
	tokenCopy := token.DeepCopy()
	tokenUtil.SetTokenExpiresAt(tokenCopy)

	token, err = m.tokens.Update(tokenCopy)
	if err != nil {
		if !apierrors.IsConflict(err) {
			return nil, "", fmt.Errorf("getToken: updating token [%s] failed [%v]", randomizedTokenName, err)
		}

		err = wait.ExponentialBackoff(backoff, func() (bool, error) {
			token, err = m.tokens.Get(randomizedTokenName, v1.GetOptions{})
			if err != nil {
				return false, err
			}

			if token.ExpiresAt == "" {
				tokenCopy := token.DeepCopy()
				tokenUtil.SetTokenExpiresAt(tokenCopy)

				token, err = m.tokens.Update(tokenCopy)
				if err != nil {
					logrus.Debugf("getToken: updating token [%s] failed [%v]", randomizedTokenName, err)
					if apierrors.IsConflict(err) {
						return false, nil
					}
					return false, err
				}
			}
			return true, nil
		})

		if err != nil {
			return nil, "", fmt.Errorf("getToken: retry updating token [%s] failed [%v]", randomizedTokenName, err)
		}
	}

	logrus.Debugf("getToken: token %s expiresAt %s", token.Name, token.ExpiresAt)
	return token, createdTokenValue, nil
}

func (m *userManager) EnsureUser(principalName, displayName string) (*v3.User, error) {
	var user *v3.User
	var err error
	var labelSet labels.Set

	// First check the local cache
	user, err = m.checkCache(principalName)
	if err != nil {
		return nil, err
	}

	if user == nil {
		// Not in cache, query API by label
		user, labelSet, err = m.checkLabels(principalName)
		if err != nil {
			return nil, err
		}
	}

	if user != nil {
		if displayName != "" && user.DisplayName == "" {
			user.DisplayName = displayName
			if _, err := m.users.Update(user); err != nil {
				return nil, err
			}
		}

		// If the user does not have the annotation it indicates the user was created
		// through the UI or from a previous rancher version so don't add the
		// default bindings.
		if _, ok := user.Annotations[roleTemplatesRequired]; !ok {
			return user, nil
		}

		if v32.UserConditionInitialRolesPopulated.IsTrue(user) {
			// The users global role bindings were already created. They can differ
			// from what is in the annotation if they were updated manually.
			return user, nil
		}
	} else {
		// User doesn't exist, create user
		logrus.Infof("Creating user for principal %v", principalName)

		// Create a hash of the principalName to use as the name for the user,
		// this lets k8s tell us if there are duplicate users with the same name
		// thus avoiding a race.
		hasher := sha256.New()
		hasher.Write([]byte(principalName))
		sha := base32.StdEncoding.WithPadding(-1).EncodeToString(hasher.Sum(nil))[:10]

		annotations, err := m.createUsersRoleAnnotation()
		if err != nil {
			return nil, err
		}

		user = &v3.User{
			ObjectMeta: v1.ObjectMeta{
				Name:        "u-" + strings.ToLower(sha),
				Labels:      labelSet,
				Annotations: annotations,
			},
			DisplayName:  displayName,
			PrincipalIDs: []string{principalName},
		}

		user, err = m.users.Create(user)
		if err != nil {
			return nil, err
		}

		err = m.CreateNewUserClusterRoleBinding(user.Name, user.UID)
		if err != nil {
			return nil, err
		}
	}

	logrus.Infof("Creating globalRoleBindings for %v", user.Name)
	err = m.createUsersBindings(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (m *userManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	if !m.manageBindings {
		return nil
	}

	roleName := userName + "-view"
	bindingName := "grb-" + roleName

	ownerReference := v1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "User",
		Name:       userName,
		UID:        userUID,
	}

	cr, err := m.clusterRoleLister.Get("", roleName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// ClusterRole doesn't exist yet, create it.
		rule := k8srbacv1.PolicyRule{
			Verbs:         []string{"get"},
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{"users"},
			ResourceNames: []string{userName},
		}
		role := &k8srbacv1.ClusterRole{
			ObjectMeta: v1.ObjectMeta{
				Name:            roleName,
				OwnerReferences: []v1.OwnerReference{ownerReference},
			},
			Rules: []k8srbacv1.PolicyRule{rule},
		}

		cr, err = m.rbacClient.ClusterRoles("").Create(role)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	_, err = m.clusterRoleBindingLister.Get("", bindingName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// ClusterRoleBinding doesn't exit yet, create it.
		crb := &k8srbacv1.ClusterRoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:            bindingName,
				OwnerReferences: []v1.OwnerReference{ownerReference},
			},
			Subjects: []k8srbacv1.Subject{
				k8srbacv1.Subject{
					Kind: "User",
					Name: userName,
				},
			},
			RoleRef: k8srbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: cr.Name,
			},
		}
		_, err = m.rbacClient.ClusterRoleBindings("").Create(crb)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	return nil
}

func (m *userManager) createUsersBindings(user *v3.User) error {
	if !m.manageBindings {
		return nil
	}

	roleMap := make(map[string][]string)
	err := json.Unmarshal([]byte(user.Annotations[roleTemplatesRequired]), &roleMap)
	if err != nil {
		return err
	}

	// Collect the users existing globalRoleBindings
	var existingGRB []string
	grbs, err := m.grbIndexer.ByIndex(grbByUserIndex, user.Name)
	if err != nil {
		return err
	}

	for _, grb := range grbs {
		binding, ok := grb.(*v3.GlobalRoleBinding)
		if !ok {
			continue
		}
		existingGRB = append(existingGRB, binding.GlobalRoleName)
	}

	var createdRoles []string
	for _, role := range roleMap["required"] {
		if !slice.ContainsString(existingGRB, role) {
			_, err := m.globalRoleBindings.Create(&v3.GlobalRoleBinding{
				ObjectMeta: v1.ObjectMeta{
					GenerateName: "grb-",
				},
				UserName:       user.Name,
				GlobalRoleName: role,
			})

			if err != nil {
				return err
			}
		}
		createdRoles = append(createdRoles, role)
	}

	roleMap["created"] = createdRoles
	d, err := json.Marshal(roleMap)
	if err != nil {
		return err
	}

	rtr := string(d)

	sleepTime := 100
	// The user needs updated so keep trying if there is a conflict
	for i := 0; i <= 3; i++ {
		user, err = m.users.Get(user.Name, v1.GetOptions{})
		if err != nil {
			return err
		}

		user.Annotations[roleTemplatesRequired] = rtr

		if reflect.DeepEqual(roleMap["required"], createdRoles) {
			v32.UserConditionInitialRolesPopulated.True(user)
		}

		_, err = m.users.Update(user)
		if err != nil {
			if apierrors.IsConflict(err) {
				// Conflict on the user, sleep and try again
				time.Sleep(time.Duration(sleepTime) * time.Millisecond)
				sleepTime *= 2
				continue
			}
			return err
		}
		break
	}

	return nil
}

func (m *userManager) createUsersRoleAnnotation() (map[string]string, error) {
	if !m.manageBindings {
		return nil, nil
	}

	roleMap := make(map[string][]string)

	roles, err := m.globalRoleLister.List("", labels.NewSelector())
	if err != nil {
		return nil, err
	}

	for _, gr := range roles {
		if gr.NewUserDefault {
			roleMap["required"] = append(roleMap["required"], gr.Name)
		}
	}

	d, err := json.Marshal(roleMap)
	if err != nil {
		return nil, err
	}

	annotations := make(map[string]string)
	annotations[roleTemplatesRequired] = string(d)

	return annotations, nil
}

func (m *userManager) GetUserByPrincipalID(principalName string) (*v3.User, error) {
	user, err := m.checkCache(principalName)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Not in cache, query API by label
		user, _, err = m.checkLabels(principalName)
		if err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (m *userManager) DeleteToken(tokenName string) error {
	return m.tokens.Delete(tokenName, &v1.DeleteOptions{})
}

func (m *userManager) checkCache(principalName string) (*v3.User, error) {
	users, err := m.userIndexer.ByIndex(userByPrincipalIndex, principalName)
	if err != nil {
		return nil, err
	}
	if len(users) > 1 {
		return nil, errors.Errorf("can't find unique user for principal %v", principalName)
	}
	if len(users) == 1 {
		u := users[0].(*v3.User)
		return u.DeepCopy(), nil
	}
	return nil, nil
}

func (m *userManager) userExistsInClusterOrProject(userNameAndPrincipals []string) (bool, error) {
	if !m.manageBindings {
		return false, nil
	}

	for _, principal := range userNameAndPrincipals {
		crtbs, err := m.crtbIndexer.ByIndex(crtbsByPrincipalAndUserIndex, principal)
		if err != nil {
			return false, err
		}
		if len(crtbs) > 0 {
			return true, nil
		}
		prtbs, err := m.prtbIndexer.ByIndex(prtbsByPrincipalAndUserIndex, principal)
		if err != nil {
			return false, err
		}
		if len(prtbs) > 0 {
			return true, nil
		}
	}

	return false, nil
}

func (m *userManager) checkLabels(principalName string) (*v3.User, labels.Set, error) {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalName))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	set := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})
	users, err := m.users.List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, nil, err
	}

	if len(users.Items) == 0 {
		return nil, set, nil
	}

	var match *v3.User
	for _, u := range users.Items {
		if slice.ContainsString(u.PrincipalIDs, principalName) {
			if match != nil {
				// error out on duplicates
				return nil, nil, errors.Errorf("can't find unique user for principal %v", principalName)
			}
			match = &u
		}
	}

	return match, set, nil
}

func userByPrincipal(obj interface{}) ([]string, error) {
	u, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}

	match := false
	for _, id := range u.PrincipalIDs {
		if strings.HasPrefix(id, "local://") {
			match = true
			break
		}
	}

	if match {
		return u.PrincipalIDs, nil
	}
	return append(u.PrincipalIDs, "local://"+u.Name), nil
}

func crtbsByPrincipalAndUser(obj interface{}) ([]string, error) {
	var principals []string
	b, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if b.GroupPrincipalName != "" {
		principals = append(principals, b.GroupPrincipalName)
	}
	if b.UserPrincipalName != "" {
		principals = append(principals, b.UserPrincipalName)
	}
	if b.UserName != "" {
		principals = append(principals, b.UserName)
	}
	return principals, nil
}

func prtbsByPrincipalAndUser(obj interface{}) ([]string, error) {
	var principals []string
	b, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if b.GroupPrincipalName != "" {
		principals = append(principals, b.GroupPrincipalName)
	}
	if b.UserPrincipalName != "" {
		principals = append(principals, b.UserPrincipalName)
	}
	if b.UserName != "" {
		principals = append(principals, b.UserName)
	}
	return principals, nil
}

func grbByUser(obj interface{}) ([]string, error) {
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{grb.UserName}, nil
}

func providerExists(principalIDs []string, provider string) bool {
	for _, id := range principalIDs {
		splitID := strings.Split(id, ":")[0]
		if strings.Contains(splitID, provider) {
			return true
		}
	}
	return false
}
