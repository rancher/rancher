package local

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
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

var invalidHash, _ = bcrypt.GenerateFromPassword([]byte("invalid"), bcrypt.DefaultCost)

type Provider struct {
	userLister   v3.UserLister
	groupLister  v3.GroupLister
	userIndexer  cache.Indexer
	gmIndexer    cache.Indexer
	groupIndexer cache.Indexer
	tokenMGR     *tokens.Manager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, tokenMGR *tokens.Manager) common.AuthProvider {
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
		userIndexer:  informer.GetIndexer(),
		gmIndexer:    gmInformer.GetIndexer(),
		groupLister:  mgmtCtx.Management.Groups("").Controller().Lister(),
		groupIndexer: gInformer.GetIndexer(),
		userLister:   mgmtCtx.Management.Users("").Controller().Lister(),
		tokenMGR:     tokenMGR,
	}
	return l
}

func (l *Provider) LogoutAll(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	return nil
}

func (l *Provider) Logout(apiContext *types.APIContext, token accessor.TokenAccessor) error {
	return nil
}

func (l *Provider) GetName() string {
	return Name
}

func (l *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = l.actionHandler
}

func (l *Provider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	return common.TransformToAuthProvider(authConfig), nil
}

func (l *Provider) getUser(username string) (*v3.User, error) {
	objs, err := l.userIndexer.ByIndex(userNameIndex, username)

	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, httperror.WrapAPIError(err, httperror.Unauthorized, "authentication failed")
	}
	if len(objs) > 1 {
		return nil, fmt.Errorf("found more than one users with username %v", username)
	}

	user, ok := objs[0].(*v3.User)

	if !ok {
		return nil, fmt.Errorf("fatal error. %v is not a user", objs[0])
	}
	return user, nil
}

func (l *Provider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	localInput, ok := input.(*v32.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, "", httperror.NewAPIError(httperror.ServerError, "Unexpected input type")
	}

	username := localInput.Username
	pwd := localInput.Password

	authFailedError := httperror.NewAPIError(httperror.Unauthorized, "authentication failed")
	user, err := l.getUser(username)
	if err != nil {
		// If the user don't exist the password is evaluated
		// to avoid user enumeration via timing attack (time based side-channel).
		bcrypt.CompareHashAndPassword(invalidHash, []byte(pwd))
		logrus.Debugf("Get User [%s] failed during Authentication: %v", username, err)
		return v3.Principal{}, nil, "", authFailedError
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pwd)); err != nil {
		logrus.Debugf("Authentication failed for User [%s]: %v", username, err)
		return v3.Principal{}, nil, "", authFailedError
	}

	principalID := getLocalPrincipalID(user)
	userPrincipal := l.toPrincipal("user", user.DisplayName, user.Username, principalID, nil)
	userPrincipal.Me = true

	groupPrincipals, err := l.getGroupPrincipals(user)
	if err != nil {
		return v3.Principal{}, nil, "", errors.Wrapf(err, "failed to get groups for %v", user.Name)
	}

	return userPrincipal, groupPrincipals, "", nil
}

func getLocalPrincipalID(user *v3.User) string {
	// TODO error condition handling: no principal, more than one that would match
	var principalID string
	for _, p := range user.PrincipalIDs {
		if strings.HasPrefix(p, Name+"://") {
			principalID = p
		}
	}
	if principalID == "" {
		return Name + "://" + user.Name
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

func (l *Provider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	userID := strings.SplitN(principalID, "://", 2)[1]
	user, err := l.userLister.Get("", userID)
	if err != nil {
		return nil, err
	}

	return l.getGroupPrincipals(user)
}

func (l *Provider) SearchPrincipals(searchKey, principalType string, token accessor.TokenAccessor) ([]v3.Principal, error) {
	return l.SearchPrincipalsDedupe(searchKey, principalType, token, nil)
}

// SearchPrincipalsDedupe performs principal search, but deduplicates the results against the supplied list (that should have come from other non-local auth providers)
// This is to avoid getting duplicate search results
func (l *Provider) SearchPrincipalsDedupe(searchKey, principalType string, token accessor.TokenAccessor, principalsFromOtherProviders []v3.Principal) ([]v3.Principal, error) {
	fromOtherProviders := map[string]bool{}
	for _, p := range principalsFromOtherProviders {
		fromOtherProviders[p.Name] = true
	}
	var principals []v3.Principal
	var localUsers []*v3.User
	var localGroups []*v3.Group
	var err error

	queryKey := strings.ToLower(searchKey)
	if len(searchKey) > searchIndexDefaultLen {
		localUsers, localGroups, err = l.listAllUsersAndGroups(queryKey)
	} else {
		localUsers, localGroups, err = l.listUsersAndGroupsByIndex(queryKey)
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
			userPrincipal := l.toPrincipal("user", user.DisplayName, user.Username, principalID, token)
			principals = append(principals, userPrincipal)
		}
	}

	if principalType == "" || principalType == "group" {
		for _, group := range localGroups {
			groupPrincipal := l.toPrincipal("group", group.DisplayName, "", Name+"://"+group.Name, token)
			principals = append(principals, groupPrincipal)
		}
	}

	return principals, nil
}

func (l *Provider) toPrincipal(principalType, displayName, loginName, id string, token accessor.TokenAccessor) v3.Principal {
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
			princ.Me = l.isThisUserMe(token.GetUserPrincipal(), princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = l.tokenMGR.IsMemberOf(token, princ)
		}
	}

	return princ
}

func (l *Provider) GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error) {
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
	princ := l.toPrincipal("user", user.DisplayName, user.Username, princID, token)
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
		if !userMatchesSearchKey(user, searchKey) {
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

func (l *Provider) isThisUserMe(me, other v3.Principal) bool {
	return me.ObjectMeta.Name == other.ObjectMeta.Name &&
		me.LoginName == other.LoginName &&
		me.PrincipalType == other.PrincipalType
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
	fieldIndexes := sets.New[string]()

	fieldIndexes.Insert(indexField(user.Username, min(len(user.Username), searchIndexDefaultLen))...)
	fieldIndexes.Insert(indexField(user.DisplayName, min(len(user.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes.Insert(indexField(user.ObjectMeta.Name, min(len(user.ObjectMeta.Name), searchIndexDefaultLen))...)

	splitToLower := func(s string, limit int) []string {
		var lowers []string
		for _, v := range strings.Fields(s) {
			lv := strings.ToLower(v)
			lowers = append(lowers, lv[:min(limit, len(lv))])
		}

		return lowers
	}

	fieldIndexes.Insert(splitToLower(user.Username, min(len(user.Username), searchIndexDefaultLen))...)
	fieldIndexes.Insert(splitToLower(user.DisplayName, min(len(user.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes.Insert(splitToLower(user.ObjectMeta.Name, min(len(user.ObjectMeta.Name), searchIndexDefaultLen))...)

	return fieldIndexes.UnsortedList(), nil
}

func groupSearchIndexer(obj interface{}) ([]string, error) {
	group, ok := obj.(*v3.Group)
	if !ok {
		return []string{}, nil
	}
	var fieldIndexes []string

	fieldIndexes = append(fieldIndexes, indexField(group.DisplayName, min(len(group.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(group.ObjectMeta.Name, min(len(group.ObjectMeta.Name), searchIndexDefaultLen))...)

	return fieldIndexes, nil
}

func indexField(field string, maxIndex int) []string {
	var fieldIndexes []string
	for i := 2; i <= maxIndex; i++ {
		simplified := []rune(simplifyString(field))

		// This calculates the string to be indexed after it has been
		// simplified because it may now be shorter than it was when maxIndex
		// was calculated for the call to indexField.
		fi := string(simplified)[0:min(len(simplified), i)]

		fieldIndexes = append(fieldIndexes, strings.ToLower(fi))
	}

	return fieldIndexes
}

func (l *Provider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	userID := strings.TrimPrefix(userPrincipalID, Name+"://")
	user, err := l.userLister.Get("", userID)
	if err != nil {
		return false, err
	}

	if user.Username != "" {
		return true, nil
	}

	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, "system://") {
			return true, nil
		}
	}

	return false, nil
}

func (l *Provider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider checks if the local auth provider is currently disabled in Rancher.
// As of now, local provider can't be disabled, so this method always returns false and nil for the error.
func (l *Provider) IsDisabledProvider() (bool, error) {
	return false, nil
}

// CleanupResources deletes resources associated with the local auth provider.
func (l *Provider) CleanupResources(*v3.AuthConfig) error {
	return nil
}

func userMatchesSearchKey(user *v3.User, searchKey string) bool {
	normalizedDisplayName := strings.ToLower(normalizeWhitespace(simplifyString(user.DisplayName)))
	normalizedSearchKey := strings.ToLower(normalizeWhitespace(simplifyString(searchKey)))

	return (strings.HasPrefix(user.ObjectMeta.Name, searchKey) ||
		strings.Contains(strings.ToLower(normalizeWhitespace(user.Username)), normalizedSearchKey) ||
		strings.Contains(normalizedDisplayName, normalizedSearchKey))
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// simplifyString transforms unicode characters in the string by replacing
// the characters.
//
// The set of characters that is replaced (unicode.Mn) is here
//
//	https://www.compart.com/en/unicode/category/Mn
func simplifyString(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, err := transform.String(t, s)

	// This shouldn't really happen, as the rune transformer is very forgiving
	// and bad things get changed to �
	if err != nil {
		logrus.Errorf("failed to simplify string %q: %s", s, err)
		return s
	}

	return result
}
