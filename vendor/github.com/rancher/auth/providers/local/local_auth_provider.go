package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// TODO Cleanup error logging. If error is being returned, use errors.wrap to return and dont log here

//Constants for github
const (
	Name             = "local"
	userNameIndex    = "authn.management.cattle.io/user-username-index"
	gmPrincipalIndex = "authn.management.cattle.io/groupmember-principalid-index"
)

func userNameIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}
	return []string{user.UserName}, nil
}

func gmPIdIndexer(obj interface{}) ([]string, error) {
	gm, ok := obj.(*v3.GroupMember)
	if !ok {
		return []string{}, nil
	}
	return []string{gm.PrincipalID}, nil
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) *LProvider {
	informer := mgmtCtx.Management.Users("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{userNameIndex: userNameIndexer}
	informer.AddIndexers(indexers)

	gmInformer := mgmtCtx.Management.GroupMembers("").Controller().Informer()
	gmIndexers := map[string]cache.IndexFunc{gmPrincipalIndex: gmPIdIndexer}
	gmInformer.AddIndexers(gmIndexers)

	l := &LProvider{
		userIndexer: informer.GetIndexer(),
		gmIndexer:   gmInformer.GetIndexer(),
		groups:      mgmtCtx.Management.Groups("").Controller().Lister(),
	}
	return l
}

//LProvider implements an PrincipalProvider for local auth
type LProvider struct {
	groups      v3.GroupLister
	userIndexer cache.Indexer
	gmIndexer   cache.Indexer
}

//GetName returns the name of the provider
func (l *LProvider) GetName() string {
	return Name
}

func (l *LProvider) AuthenticateUser(loginInput v3.LoginInput) (v3.Principal, []v3.Principal, int, error) {
	// TODO fix responses to be json
	username := loginInput.LocalCredential.Username
	pwd := loginInput.LocalCredential.Password

	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	objs, err := l.userIndexer.ByIndex(userNameIndex, username)
	if err != nil {
		logrus.Infof("Failed to get User resource for %v: %v", username, err)
		return userPrincipal, groupPrincipals, 401, fmt.Errorf("Invalid Credentials")
	}
	if len(objs) == 0 {
		return userPrincipal, groupPrincipals, 401, fmt.Errorf("Invalid Credentials")
	}
	if len(objs) > 1 {
		logrus.Errorf("Found more than one user matching %v", username)
		return userPrincipal, groupPrincipals, 401, fmt.Errorf("Invalid Credentials")
	}

	user, ok := objs[0].(*v3.User)
	if !ok {
		logrus.Errorf("User isnt a user %v", objs[0])
		return userPrincipal, groupPrincipals, 500, fmt.Errorf("fatal error. User is not a user")

	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pwd)); err != nil {
		return userPrincipal, groupPrincipals, 401, fmt.Errorf("Invalid Credentials")
	}

	principalID := getLocalPrincipalID(user)
	userPrincipal = v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: principalID},
		DisplayName: user.DisplayName,
		LoginName:   user.UserName,
		Kind:        "user",
		Me:          true,
	}

	groupPrincipals, status, err := l.getGroupPrincipals(user)
	if err != nil {
		logrus.Errorf("Failed to get group identities for local user: %v, user: %v", err, user.ObjectMeta.Name)
		return userPrincipal, groupPrincipals, status, fmt.Errorf("Error getting group identities for local user %v", err)
	}

	return userPrincipal, groupPrincipals, 0, nil
}

func getLocalPrincipalID(user *v3.User) string {
	// TODO error condition handling: no principal, more than one that would match
	var principalID string
	for _, p := range user.PrincipalIDs {
		if strings.HasPrefix(p, Name+"://") {
			principalID = p
		}
	}
	return principalID
}

func (l *LProvider) getGroupPrincipals(user *v3.User) ([]v3.Principal, int, error) {
	groupPrincipals := []v3.Principal{}

	for _, pid := range user.PrincipalIDs {
		objs, err := l.gmIndexer.ByIndex(gmPrincipalIndex, pid)
		if err != nil {
			return []v3.Principal{}, 500, err
		}

		for _, o := range objs {
			gm, ok := o.(*v3.GroupMember)
			if !ok {
				continue
			}

			//find group for this member mapping
			localGroup, err := l.groups.Get("", gm.GroupName)
			if err != nil {
				logrus.Errorf("Failed to get Group resource %v: %v", gm.GroupName, err)
				continue
			}

			groupPrincipal := v3.Principal{
				ObjectMeta:  metav1.ObjectMeta{Name: Name + "://" + localGroup.Name},
				DisplayName: localGroup.DisplayName,
				Kind:        "group",
			}
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}

	return groupPrincipals, 0, nil
}
