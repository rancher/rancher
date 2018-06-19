package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	Name                  = "local"
	userNameIndex         = "authn.management.cattle.io/user-username-index"
	gmPrincipalIndex      = "authn.management.cattle.io/groupmember-principalid-index"
	userSearchIndex       = "authn.management.cattle.io/user-search-index"
	groupSearchIndex      = "authn.management.cattle.io/group-search-index"
	searchIndexDefaultLen = 6
)

type Provider struct {
	userLister       v3.UserLister
	groupLister      v3.GroupLister
	authConfigLister v3.AuthConfigLister
	userIndexer      cache.Indexer
	gmIndexer        cache.Indexer
	groupIndexer     cache.Indexer
	userMGR          user.Manager
	tokenMGR         *tokens.Manager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	informer := mgmtCtx.Management.Users("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{userNameIndex: userNameIndexer, userSearchIndex: userSearchIndexer}
	informer.AddIndexers(indexers)

	gmInformer := mgmtCtx.Management.GroupMembers("").Controller().Informer()
	gmIndexers := map[string]cache.IndexFunc{gmPrincipalIndex: gmPIdIndexer}
	gmInformer.AddIndexers(gmIndexers)

	gInformer := mgmtCtx.Management.Groups("").Controller().Informer()
	gIndexers := map[string]cache.IndexFunc{groupSearchIndex: groupSearchIndexer}
	gInformer.AddIndexers(gIndexers)

	l := &Provider{
		userIndexer:      informer.GetIndexer(),
		gmIndexer:        gmInformer.GetIndexer(),
		groupLister:      mgmtCtx.Management.Groups("").Controller().Lister(),
		groupIndexer:     gInformer.GetIndexer(),
		userLister:       mgmtCtx.Management.Users("").Controller().Lister(),
		authConfigLister: mgmtCtx.Management.AuthConfigs("").Controller().Lister(),
		userMGR:          userMGR,
		tokenMGR:         tokenMGR,
	}
	return l
}

func (l *Provider) GetName() string {
	return Name
}

func (l *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = l.actionHandler
}

func (l *Provider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	return common.TransformToAuthProvider(authConfig)
}

func (l *Provider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, error) {
	localInput, ok := input.(*v3public.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.ServerError, "Unexpected input type")
	}

	username := localInput.Username
	pwd := localInput.Password

	objs, err := l.userIndexer.ByIndex(userNameIndex, username)
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}
	if len(objs) == 0 {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
	}
	if len(objs) > 1 {
		return v3.Principal{}, nil, nil, fmt.Errorf("found more than one users with username %v", username)
	}

	user, ok := objs[0].(*v3.User)
	if !ok {
		return v3.Principal{}, nil, nil, fmt.Errorf("fatal error. %v is not a user", objs[0])

	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pwd)); err != nil {
		return v3.Principal{}, nil, nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
	}

	principalID := getLocalPrincipalID(user)
	userPrincipal := l.toPrincipal("user", user.DisplayName, user.Username, principalID, nil)
	userPrincipal.Me = true

	groupPrincipals, err := l.getGroupPrincipals(user)
	if err != nil {
		return v3.Principal{}, nil, nil, errors.Wrapf(err, "failed to get groups for %v", user.ObjectMeta.Name)
	}

	acs, err := l.authConfigLister.List("", labels.Everything())
	if err != nil {
		return v3.Principal{}, nil, nil, err
	}

	var checked, allowed bool
	for _, config := range acs {
		if config.Name != Name && config.Enabled {
			checked = true
			allowed, err = l.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal, groupPrincipals)
			if err != nil {
				return v3.Principal{}, nil, nil, err
			}
			if allowed {
				break
			}
		}
	}

	if checked && !allowed {
		return v3.Principal{}, nil, nil, httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	return userPrincipal, groupPrincipals, map[string]string{}, nil
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

func (l *Provider) getGroupPrincipals(user *v3.User) ([]v3.Principal, error) {
	groupPrincipals := []v3.Principal{}

	for _, pid := range user.PrincipalIDs {
		objs, err := l.gmIndexer.ByIndex(gmPrincipalIndex, pid)
		if err != nil {
			return []v3.Principal{}, err
		}

		for _, o := range objs {
			gm, ok := o.(*v3.GroupMember)
			if !ok {
				continue
			}

			//find group for this member mapping
			localGroup, err := l.groupLister.Get("", gm.GroupName)
			if err != nil {
				logrus.Errorf("Failed to get Group resource %v: %v", gm.GroupName, err)
				continue
			}

			groupPrincipal := l.toPrincipal("group", localGroup.DisplayName, "", Name+"://"+localGroup.Name, nil)
			groupPrincipal.MemberOf = true
			groupPrincipals = append(groupPrincipals, groupPrincipal)
		}
	}

	return groupPrincipals, nil
}

func (l *Provider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	return l.SearchPrincipalsDedupe(searchKey, principalType, token, nil)
}

// SearchPrincipalsDedupe performs principal search, but deduplicates the results against the supplied list (that should have come from other non-local auth providers)
// This is to avoid getting duplicate search results
func (l *Provider) SearchPrincipalsDedupe(searchKey, principalType string, token v3.Token, principalsFromOtherProviders []v3.Principal) ([]v3.Principal, error) {
	fromOtherProviders := map[string]bool{}
	for _, p := range principalsFromOtherProviders {
		fromOtherProviders[p.Name] = true
	}
	var principals []v3.Principal
	var localUsers []*v3.User
	var localGroups []*v3.Group
	var err error

	if len(searchKey) > searchIndexDefaultLen {
		localUsers, localGroups, err = l.listAllUsersAndGroups(searchKey)
	} else {
		localUsers, localGroups, err = l.listUsersAndGroupsByIndex(searchKey)
	}

	if err != nil {
		logrus.Infof("Failed to search User/Group resources for %v: %v", searchKey, err)
		return principals, err
	}

	if principalType == "" || principalType == "user" {
	User:
		for _, user := range localUsers {
			for _, p := range user.PrincipalIDs {
				if fromOtherProviders[p] {
					continue User
				}
			}
			principalID := getLocalPrincipalID(user)
			userPrincipal := l.toPrincipal("user", user.DisplayName, user.Username, principalID, &token)
			principals = append(principals, userPrincipal)
		}
	}

	if principalType == "" || principalType == "group" {
		for _, group := range localGroups {
			groupPrincipal := l.toPrincipal("group", group.DisplayName, "", Name+"://"+group.Name, &token)
			principals = append(principals, groupPrincipal)
		}
	}

	return principals, nil
}

func (l *Provider) toPrincipal(principalType, displayName, loginName, id string, token *v3.Token) v3.Principal {
	if displayName == "" {
		displayName = loginName
	}

	princ := v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: id},
		DisplayName: displayName,
		LoginName:   loginName,
		Provider:    Name,
		Me:          false,
	}

	if principalType == "user" {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = l.isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = l.tokenMGR.IsMemberOf(*token, princ)
		}
	}

	return princ
}

func (l *Provider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	// TODO implement group lookup (local groups currently not implemented, so we can skip)
	// parsing id to get the external id and type. id looks like github_[user|org|team]://12345
	var name string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	name = strings.TrimPrefix(parts[1], "//")

	user, err := l.userLister.Get("", name)
	if err != nil {
		return v3.Principal{}, err
	}

	princID := getLocalPrincipalID(user)
	princ := l.toPrincipal("user", user.DisplayName, user.Username, princID, &token)
	return princ, nil
}

func (l *Provider) listAllUsersAndGroups(searchKey string) ([]*v3.User, []*v3.Group, error) {
	var localUsers []*v3.User
	var localGroups []*v3.Group

	allUsers, err := l.userLister.List("", labels.NewSelector())
	if err != nil {
		logrus.Infof("Failed to search User resources for %v: %v", searchKey, err)
		return localUsers, localGroups, err
	}
	for _, user := range allUsers {
		if !(strings.HasPrefix(user.ObjectMeta.Name, searchKey) || strings.HasPrefix(user.Username, searchKey) || strings.HasPrefix(user.DisplayName, searchKey)) {
			continue
		}
		localUsers = append(localUsers, user)
	}

	allGroups, err := l.groupLister.List("", labels.NewSelector())
	if err != nil {
		logrus.Infof("Failed to search group resources for %v: %v", searchKey, err)
		return localUsers, localGroups, err
	}
	for _, group := range allGroups {
		if !(strings.HasPrefix(group.ObjectMeta.Name, searchKey) || strings.HasPrefix(group.DisplayName, searchKey)) {
			continue
		}
		localGroups = append(localGroups, group)
	}
	return localUsers, localGroups, err
}

func (l *Provider) listUsersAndGroupsByIndex(searchKey string) ([]*v3.User, []*v3.Group, error) {
	var localUsers []*v3.User
	var localGroups []*v3.Group
	var err error

	objs, err := l.userIndexer.ByIndex(userSearchIndex, searchKey)
	if err != nil {
		logrus.Infof("Failed to search User resources for %v: %v", searchKey, err)
		return localUsers, localGroups, err
	}

	for _, obj := range objs {
		user, ok := obj.(*v3.User)
		if !ok {
			logrus.Errorf("User isnt a user %v", obj)
			return localUsers, localGroups, err
		}
		localUsers = append(localUsers, user)
	}

	groupObjs, err := l.groupIndexer.ByIndex(groupSearchIndex, searchKey)
	if err != nil {
		logrus.Infof("Failed to search Group resources for %v: %v", searchKey, err)
		return localUsers, localGroups, err
	}

	for _, obj := range groupObjs {
		group, ok := obj.(*v3.Group)
		if !ok {
			logrus.Errorf("Object isnt a group %v", obj)
			return localUsers, localGroups, err
		}
		localGroups = append(localGroups, group)
	}
	return localUsers, localGroups, err

}

func (l *Provider) isThisUserMe(me v3.Principal, other v3.Principal) bool {

	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (l *Provider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func userNameIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}
	return []string{user.Username}, nil
}

func gmPIdIndexer(obj interface{}) ([]string, error) {
	gm, ok := obj.(*v3.GroupMember)
	if !ok {
		return []string{}, nil
	}
	return []string{gm.PrincipalID}, nil
}

func userSearchIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}
	var fieldIndexes []string

	fieldIndexes = append(fieldIndexes, indexField(user.Username, minOf(len(user.Username), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.DisplayName, minOf(len(user.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.ObjectMeta.Name, minOf(len(user.ObjectMeta.Name), searchIndexDefaultLen))...)

	return fieldIndexes, nil
}

func groupSearchIndexer(obj interface{}) ([]string, error) {
	group, ok := obj.(*v3.Group)
	if !ok {
		return []string{}, nil
	}
	var fieldIndexes []string

	fieldIndexes = append(fieldIndexes, indexField(group.DisplayName, minOf(len(group.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(group.ObjectMeta.Name, minOf(len(group.ObjectMeta.Name), searchIndexDefaultLen))...)

	return fieldIndexes, nil
}

func minOf(length int, defaultLen int) int {
	if length < defaultLen {
		return length
	}
	return defaultLen
}

func indexField(field string, maxindex int) []string {
	var fieldIndexes []string
	for i := 2; i <= maxindex; i++ {
		fieldIndexes = append(fieldIndexes, field[0:i])
	}
	return fieldIndexes
}
