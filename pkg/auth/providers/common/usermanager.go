package common

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/rancher/pkg/wrangler"
	wrangrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
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

// NewUserManagerNoBindings creates an instance of userManager
// without CRTB, PRTB, GRB and CRB informers and clients.
func NewUserManagerNoBindings(wranglerContext *wrangler.Context) (user.Manager, error) {
	userInformer := wranglerContext.Mgmt.User().Informer()
	// registering the same index more than once will cause an error. Since we attempt to register this index in multiple
	// locations, we need to verify if it has already been registered.
	if _, ok := userInformer.GetIndexer().GetIndexers()[userByPrincipalIndex]; !ok {
		userIndexers := map[string]cache.IndexFunc{
			userByPrincipalIndex: userByPrincipal,
		}
		if err := userInformer.AddIndexers(userIndexers); err != nil {
			return nil, err
		}
	}

	return &userManager{
		users:              wranglerContext.Mgmt.User(),
		userCache:          wranglerContext.Mgmt.User().Cache(),
		userAttributes:     wranglerContext.Mgmt.UserAttribute(),
		userAttributeCache: wranglerContext.Mgmt.UserAttribute().Cache(),
		userIndexer:        userInformer.GetIndexer(),
		rbacClient:         wranglerContext.RBAC,
	}, nil
}

// NewUserManagerNoBindings creates an instance of userManager.
func NewUserManager(wranglerContext *wrangler.Context) (user.Manager, error) {
	userInformer := wranglerContext.Mgmt.User().Informer()
	// registering the same index more than once will cause an error. Since we attempt to register this index in multiple
	// locations, we need to verify if it has already been registered.
	if _, ok := userInformer.GetIndexer().GetIndexers()[userByPrincipalIndex]; !ok {
		userIndexers := map[string]cache.IndexFunc{
			userByPrincipalIndex: userByPrincipal,
		}
		if err := userInformer.AddIndexers(userIndexers); err != nil {
			return nil, err
		}
	}

	crtbInformer := wranglerContext.Mgmt.ClusterRoleTemplateBinding().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		crtbsByPrincipalAndUserIndex: crtbsByPrincipalAndUser,
	}
	if err := crtbInformer.AddIndexers(crtbIndexers); err != nil {
		return nil, err
	}

	prtbInformer := wranglerContext.Mgmt.ProjectRoleTemplateBinding().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbsByPrincipalAndUserIndex: prtbsByPrincipalAndUser,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return nil, err
	}

	grbInformer := wranglerContext.Mgmt.GlobalRoleBinding().Informer()
	grbIndexers := map[string]cache.IndexFunc{
		grbByUserIndex: grbByUser,
	}
	if err := grbInformer.AddIndexers(grbIndexers); err != nil {
		return nil, err
	}

	return &userManager{
		manageBindings:           true,
		users:                    wranglerContext.Mgmt.User(),
		userCache:                wranglerContext.Mgmt.User().Cache(),
		userAttributes:           wranglerContext.Mgmt.UserAttribute(),
		userAttributeCache:       wranglerContext.Mgmt.UserAttribute().Cache(),
		userIndexer:              userInformer.GetIndexer(),
		crtbIndexer:              crtbInformer.GetIndexer(),
		prtbIndexer:              prtbInformer.GetIndexer(),
		globalRoleBindings:       wranglerContext.Mgmt.GlobalRoleBinding(),
		globalRoleLister:         wranglerContext.Mgmt.GlobalRole().Cache(),
		grbIndexer:               grbInformer.GetIndexer(),
		clusterRoleLister:        wranglerContext.RBAC.ClusterRole().Cache(),
		clusterRoleBindingLister: wranglerContext.RBAC.ClusterRoleBinding().Cache(),
		rbacClient:               wranglerContext.RBAC,
	}, nil
}

type userManager struct {
	// manageBinding means whether or not we gr, grb, crtb, and prtb exist in the cluster
	manageBindings           bool
	users                    wrangmgmtv3.UserClient
	userCache                wrangmgmtv3.UserCache
	userAttributes           wrangmgmtv3.UserAttributeClient
	userAttributeCache       wrangmgmtv3.UserAttributeCache
	globalRoleBindings       wrangmgmtv3.GlobalRoleBindingClient
	globalRoleLister         wrangmgmtv3.GlobalRoleCache
	grbIndexer               cache.Indexer
	userIndexer              cache.Indexer
	crtbIndexer              cache.Indexer
	prtbIndexer              cache.Indexer
	clusterRoleLister        wrangrbacv1.ClusterRoleCache
	clusterRoleBindingLister wrangrbacv1.ClusterRoleBindingCache
	rbacClient               wrangrbacv1.Interface
}

func (m *userManager) SetPrincipalOnCurrentUser(r *http.Request, principal v3.Principal) (*v3.User, error) {
	userID := m.GetUser(r)
	if userID == "" {
		return nil, errors.New("user not provided")
	}

	return m.SetPrincipalOnCurrentUserByUserID(userID, principal)
}

func (m *userManager) SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error) {
	user, err := m.users.Get(userID, metav1.GetOptions{})
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

func (m *userManager) GetUser(r *http.Request) string {
	return r.Header.Get(userAuthHeader)
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

		if v3.UserConditionInitialRolesPopulated.IsTrue(user) {
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
			ObjectMeta: metav1.ObjectMeta{
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

func (m *userManager) EnsureAndGetUserAttribute(userID string) (*v3.UserAttribute, bool, error) {
	attribs, err := m.userAttributeCache.Get(userID)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, false, err
	}

	if attribs == nil {
		attribs, err = m.userAttributes.Get(userID, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, false, err
		}
	}

	if attribs != nil && attribs.Name != "" {
		return attribs.DeepCopy(), false, nil
	}

	user, err := m.userCache.Get(userID)
	if err != nil {
		return nil, false, err
	}

	attribs = &v3.UserAttribute{
		ObjectMeta: metav1.ObjectMeta{
			Name: userID,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: user.APIVersion,
					Kind:       user.Kind,
					UID:        user.UID,
					Name:       user.Name,
				},
			},
		},
		GroupPrincipals: map[string]v3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
		LastRefresh:     "",
		NeedsRefresh:    false,
	}

	return attribs, true, nil
}

func (m *userManager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	attribs, needCreate, err := m.EnsureAndGetUserAttribute(userID)
	if err != nil {
		return err
	}

	if attribs.GroupPrincipals == nil {
		attribs.GroupPrincipals = make(map[string]v3.Principals)
	}

	if attribs.ExtraByProvider == nil {
		attribs.ExtraByProvider = make(map[string]map[string][]string)
	}
	if userExtraInfo == nil {
		userExtraInfo = make(map[string][]string)
	}

	shouldUpdate := m.userAttributeChanged(attribs, provider, userExtraInfo, groupPrincipals)
	if len(loginTime) > 0 && !loginTime[0].IsZero() {
		// Login time is truncated to seconds as the corresponding user label is set as epoch time.
		lastLogin := metav1.NewTime(loginTime[0].Truncate(time.Second))
		attribs.LastLogin = &lastLogin
		shouldUpdate = true
	}

	attribs.GroupPrincipals[provider] = v3.Principals{Items: groupPrincipals}
	attribs.ExtraByProvider[provider] = userExtraInfo

	if needCreate {
		_, err = m.userAttributes.Create(attribs)
		if err != nil {
			return fmt.Errorf("failed to create UserAttribute: %w", err)
		}

		return nil
	}

	if shouldUpdate {
		_, err = m.userAttributes.Update(attribs)
		if err != nil {
			return fmt.Errorf("failed to update UserAttribute: %w", err)
		}
	}

	return nil
}

func (m *userManager) userAttributeChanged(attribs *v3.UserAttribute, provider string, extraInfo map[string][]string, groupPrincipals []v3.Principal) bool {
	if len(attribs.GroupPrincipals[provider].Items) != len(groupPrincipals) {
		return true
	}

	var oldSet, newSet []string
	for _, principal := range attribs.GroupPrincipals[provider].Items {
		oldSet = append(oldSet, principal.ObjectMeta.Name)
	}
	for _, principal := range groupPrincipals {
		newSet = append(newSet, principal.ObjectMeta.Name)
	}

	slices.Sort(oldSet)
	slices.Sort(newSet)

	if !slices.Equal(oldSet, newSet) {
		return true
	}

	if attribs.ExtraByProvider == nil && extraInfo != nil {
		return true
	}

	return !reflect.DeepEqual(attribs.ExtraByProvider[provider], extraInfo)
}

func (m *userManager) IsMemberOf(token accessor.TokenAccessor, group v3.Principal) bool {
	attribs, err := m.userAttributeCache.Get(token.GetUserID())
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Warnf("Problem getting userAttribute while determining group membership for %v in %v (%v): %v",
			token.GetUserID(), group.Name, group.DisplayName, err)
		// if err not nil, then attribs will be nil. So, below code will handle it
	}

	groups := map[string]bool{}
	hitProviders := map[string]bool{}
	if attribs != nil {
		for provider, gps := range attribs.GroupPrincipals {
			for _, principal := range gps.Items {
				hitProviders[provider] = true
				groups[principal.Name] = true
			}
		}
	}

	// fallback to legacy token groupPrincipals
	if _, ok := hitProviders[token.GetAuthProvider()]; !ok {
		for _, principal := range token.GetGroupPrincipals() {
			groups[principal.Name] = true
		}
	}

	return groups[group.Name]
}

func (m *userManager) GetGroupsForTokenAuthProvider(token accessor.TokenAccessor) []v3.Principal {
	var groups []v3.Principal

	attribs, err := m.userAttributeCache.Get(token.GetUserID())
	if err != nil && !apierrors.IsNotFound(err) {
		logrus.Warnf("Problem getting userAttribute while getting groups for %v: %v", token.GetUserID(), err)
		// if err is not nil, then attribs will be. So, below code will handle it
	}

	hitProvider := false
	if attribs != nil {
		tokenProvider := token.GetAuthProvider()
		for provider, y := range attribs.GroupPrincipals {
			if provider == tokenProvider {
				hitProvider = true
				groups = append(groups, y.Items...)
			}
		}
	}

	// fallback to legacy token groupPrincipals
	if !hitProvider {
		groups = append(groups, token.GetGroupPrincipals()...)
	}

	return groups
}

func (m *userManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	if !m.manageBindings {
		return nil
	}

	roleName := userName + "-view"
	bindingName := "grb-" + roleName

	ownerReference := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "User",
		Name:       userName,
		UID:        userUID,
	}

	cr, err := m.clusterRoleLister.Get(roleName)
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
			ObjectMeta: metav1.ObjectMeta{
				Name:            roleName,
				OwnerReferences: []metav1.OwnerReference{ownerReference},
			},
			Rules: []k8srbacv1.PolicyRule{rule},
		}

		cr, err = m.rbacClient.ClusterRole().Create(role)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	_, err = m.clusterRoleBindingLister.Get(bindingName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// ClusterRoleBinding doesn't exit yet, create it.
		crb := &k8srbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            bindingName,
				OwnerReferences: []metav1.OwnerReference{ownerReference},
			},
			Subjects: []k8srbacv1.Subject{
				{
					Kind: "User",
					Name: userName,
				},
			},
			RoleRef: k8srbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: cr.Name,
			},
		}
		_, err = m.rbacClient.ClusterRoleBinding().Create(crb)
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
				ObjectMeta: metav1.ObjectMeta{
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
		user, err = m.users.Get(user.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		user.Annotations[roleTemplatesRequired] = rtr

		if reflect.DeepEqual(roleMap["required"], createdRoles) {
			v3.UserConditionInitialRolesPopulated.True(user)
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

	roles, err := m.globalRoleLister.List(labels.NewSelector())
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

// func (m *userManager) DeleteToken(tokenName string) error {
// 	return m.tokens.Delete(tokenName, &v1.DeleteOptions{})
// }

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
	users, err := m.users.List(metav1.ListOptions{LabelSelector: set.String()})
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

func userByPrincipal(obj any) ([]string, error) {
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

func crtbsByPrincipalAndUser(obj any) ([]string, error) {
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

func prtbsByPrincipalAndUser(obj any) ([]string, error) {
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

func grbByUser(obj any) ([]string, error) {
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
