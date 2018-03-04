package common

import (
	"encoding/base32"

	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	userAuthHeader         = "Impersonate-User"
	userByPrincipalIndex   = "auth.management.cattle.io/userByPrincipal"
	groupPrincpalCRTBIndex = "auth.management.cattle.io/groupPrincipalCRTB"
	groupPrincpalPRTBIndex = "auth.management.cattle.io/groupPrincipalPRTB"
)

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
		groupPrincpalCRTBIndex: groupPrincipalCRTB,
	}
	if err := crtbInformer.AddIndexers(crtbIndexers); err != nil {
		return nil, err
	}

	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		groupPrincpalPRTBIndex: groupPrincipalPRTB,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return nil, err
	}

	return &userManager{
		users:              scaledContext.Management.Users(""),
		userIndexer:        userInformer.GetIndexer(),
		crtbIndexer:        crtbInformer.GetIndexer(),
		prtbIndexer:        prtbInformer.GetIndexer(),
		tokens:             scaledContext.Management.Tokens(""),
		tokenLister:        scaledContext.Management.Tokens("").Controller().Lister(),
		globalRoleBindings: scaledContext.Management.GlobalRoleBindings(""),
	}, nil
}

type userManager struct {
	users              v3.UserInterface
	globalRoleBindings v3.GlobalRoleBindingInterface
	userIndexer        cache.Indexer
	crtbIndexer        cache.Indexer
	prtbIndexer        cache.Indexer
	tokenLister        v3.TokenLister
	tokens             v3.TokenInterface
}

func (m *userManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	userID := m.GetUser(apiContext)
	if userID == "" {
		return nil, errors.New("user not provided")
	}

	user, err := m.users.Get(userID, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !slice.ContainsString(user.PrincipalIDs, principal.Name) {
		user.PrincipalIDs = append(user.PrincipalIDs, principal.Name)
		return m.users.Update(user)
	}
	return user, nil
}

func (m *userManager) GetUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}

// checkis if the supplied principal can login based on the accessMode and allowed principals
func (m *userManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrinc v3.Principal, groups []v3.Principal) (bool, error) {
	if accessMode == "unrestricted" {
		return true, nil
	}

	if accessMode == "required" || accessMode == "restricted" {
		if slice.ContainsString(allowedPrincipalIDs, userPrinc.Name) {
			return true, nil
		}
		for _, g := range groups {
			if slice.ContainsString(allowedPrincipalIDs, g.Name) {
				return true, nil
			}
		}
		if accessMode == "restricted" {
			// check if any of the groups principals have been assigned to clusters or projects
			allowed, err := m.hasMemberGroup(groups)
			if err != nil {
				return false, err
			}
			if allowed {
				return true, nil
			}

			// check if user prinicpal exists as a user
			u, err := m.checkCache(userPrinc.Name)
			if err != nil {
				return false, err
			}
			if u != nil {
				return true, nil
			}

			// user principal not in cache, query API by label
			u, _, err = m.checkLabels(userPrinc.Name)
			if err != nil {
				return false, err
			}
			if u != nil {
				return true, nil
			}
		}
		return false, nil
	}
	return false, errors.Errorf("Unsupported accessMode: %v", accessMode)
}

func (m *userManager) EnsureToken(tokenName, description, userName string) (string, error) {
	if strings.HasPrefix(tokenName, "token-") {
		return "", errors.New("token names can't start with token-")
	}

	token, err := m.tokenLister.Get("", tokenName)
	if errors2.IsNotFound(err) {
		token, err = nil, nil
	} else if err != nil {
		return "", err
	}

	if token == nil {
		key, err := randomtoken.Generate()
		if err != nil {
			return "", fmt.Errorf("failed to generate token key")
		}

		token = &v3.Token{
			ObjectMeta: v1.ObjectMeta{
				Name: tokenName,
				Labels: map[string]string{
					tokens.UserIDLabel: userName,
				},
			},
			TTLMillis:    0,
			Description:  description,
			UserID:       userName,
			AuthProvider: "local",
			IsDerived:    true,
			Token:        key,
		}

		createdToken, err := m.tokens.Create(token)
		if err != nil {
			return "", err
		}
		token = createdToken

	}

	return token.Name + ":" + token.Token, nil
}

func (m *userManager) EnsureUser(principalName, displayName string) (*v3.User, error) {
	// First check the local cache
	u, err := m.checkCache(principalName)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Not in cache, query API by label
	u, labelSet, err := m.checkLabels(principalName)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Doesn't exist, create user
	user := &v3.User{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "user-",
			Labels:       labelSet,
		},
		DisplayName:  displayName,
		PrincipalIDs: []string{principalName},
	}

	created, err := m.users.Create(user)
	if err != nil {
		return nil, err
	}

	_, err = m.globalRoleBindings.Create(&v3.GlobalRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "globalrolebinding-",
		},
		UserName:       created.Name,
		GlobalRoleName: "user",
	})
	if err != nil {
		return nil, err
	}

	localPrincipal := "local://" + created.Name
	if !slice.ContainsString(created.PrincipalIDs, localPrincipal) {
		created.PrincipalIDs = append(created.PrincipalIDs, localPrincipal)
		return m.users.Update(created)
	}

	return created, nil
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

func (m *userManager) hasMemberGroup(groupPrincipals []v3.Principal) (bool, error) {
	for _, g := range groupPrincipals {
		crtbs, err := m.crtbIndexer.ByIndex(groupPrincpalCRTBIndex, g.Name)
		if err != nil {
			return false, err
		}
		if len(crtbs) > 0 {
			return true, nil
		}
		prtbs, err := m.prtbIndexer.ByIndex(groupPrincpalPRTBIndex, g.Name)
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

	return u.PrincipalIDs, nil
}

func groupPrincipalCRTB(obj interface{}) ([]string, error) {
	var gp []string
	b, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if b.GroupPrincipalName != "" {
		gp = append(gp, b.GroupPrincipalName)
	}
	return gp, nil
}

func groupPrincipalPRTB(obj interface{}) ([]string, error) {
	var gp []string
	b, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	if b.GroupPrincipalName != "" {
		gp = append(gp, b.GroupPrincipalName)
	}
	return gp, nil
}
