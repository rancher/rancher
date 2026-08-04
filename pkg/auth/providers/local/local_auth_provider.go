package local

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
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
	userSearchIndex       = "authn.management.cattle.io/user-search-index"
	searchIndexDefaultLen = 6
)

var invalidHash, _ = bcrypt.GenerateFromPassword([]byte("invalid"), bcrypt.DefaultCost)

type PasswordVerifier interface {
	VerifyPassword(user *apiv3.User, password string) error
}

type Provider struct {
	userLister  v3.UserLister
	userIndexer cache.Indexer
	pwdVerifier PasswordVerifier
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, _ user.Manager) common.AuthProvider {
	informer := mgmtCtx.Management.Users("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{userNameIndex: userNameIndexer, userSearchIndex: userSearchIndexer}
	_ = informer.AddIndexers(indexers)

	l := &Provider{
		userIndexer: informer.GetIndexer(),
		userLister:  mgmtCtx.Management.Users("").Controller().Lister(),
		pwdVerifier: pbkdf2.New(mgmtCtx.Wrangler.Core.Secret().Cache(), mgmtCtx.Wrangler.Core.Secret()),
	}
	return l
}

func (l *Provider) LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	return nil
}

func (l *Provider) Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error {
	return nil
}

func (l *Provider) GetName() string {
	return Name
}

func (l *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = l.actionHandler
}

func (l *Provider) TransformToAuthProvider(authConfig map[string]any) (map[string]any, error) {
	return common.TransformToAuthProvider(authConfig), nil
}

func (l *Provider) getUser(username string) (*apiv3.User, error) {
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

	user, ok := objs[0].(*apiv3.User)

	if !ok {
		return nil, fmt.Errorf("fatal error. %v is not a user", objs[0])
	}
	return user, nil
}

func (l *Provider) AuthenticateUser(_ http.ResponseWriter, _ *http.Request, input any) (apiv3.Principal, []apiv3.Principal, string, error) {
	localInput, ok := input.(*apiv3.BasicLogin)
	if !ok {
		return apiv3.Principal{}, nil, "", apierror.NewAPIError(validation.ServerError, "Unexpected input type")
	}

	username := localInput.Username
	pwd := localInput.Password

	authFailedError := apierror.NewAPIError(validation.Unauthorized, "authentication failed")
	user, err := l.getUser(username)
	if err != nil {
		// If the user don't exist the password is evaluated
		// to avoid user enumeration via timing attack (time based side-channel).
		bcrypt.CompareHashAndPassword(invalidHash, []byte(pwd))
		logrus.Debugf("Get User [%s] failed during Authentication: %v", username, err)
		return apiv3.Principal{}, nil, "", authFailedError
	}

	if err := l.pwdVerifier.VerifyPassword(user, pwd); err != nil {
		logrus.Debugf("Authentication failed for User [%s]: %v", username, err)
		return apiv3.Principal{}, nil, "", authFailedError
	}

	principalID := getLocalPrincipalID(user)
	userPrincipal := l.toPrincipal("user", user.DisplayName, user.Username, principalID, nil)
	userPrincipal.Me = true

	return userPrincipal, []apiv3.Principal{}, "", nil
}

// isLocalUser reports whether a User resource represents a user that can log
// in locally. A user is local if it has a local login username, or if its only
// principal is a local:// one. External-auth users (SAML, OAuth, SCIM) always
// carry an external provider principal such as okta_user://<uid> in addition to
// the self-referential local://<uid> that the user lifecycle controller appends
// to every user, so they have more than one principal and are excluded — they
// must not surface in local principal search results.
func isLocalUser(user *apiv3.User) bool {
	return user.Username != "" ||
		(len(user.PrincipalIDs) == 1 && strings.HasPrefix(user.PrincipalIDs[0], Name+"://"))
}

func getLocalPrincipalID(user *apiv3.User) string {
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

func (l *Provider) UsesUserSecrets() bool { return false }

// CanRefreshPrincipals reports whether the local provider can refetch group
// principals at refresh time. It cannot, because the local provider does not
// own any groups: v3.Group resources are SCIM-provisioned and owned by the
// external identity provider that created them.
func (l *Provider) CanRefreshPrincipals() bool { return false }

// RefetchGroupPrincipals is a no-op for the local provider; it is never
// invoked because CanRefreshPrincipals returns false.
func (l *Provider) RefetchGroupPrincipals(principalID string, secret string) ([]apiv3.Principal, error) {
	return []apiv3.Principal{}, nil
}

func (l *Provider) SearchPrincipals(searchKey, principalType string, token accessor.TokenAccessor) ([]apiv3.Principal, error) {
	return l.SearchPrincipalsDedupe(searchKey, principalType, token, nil)
}

// SearchPrincipalsDedupe performs principal search, and deduplicates the
// results against the supplied list (that should have come from other non-local
// auth providers) to avoid duplicate search results. Only user principals are
// returned; the local provider does not own any groups.
func (l *Provider) SearchPrincipalsDedupe(searchKey, principalType string, token accessor.TokenAccessor, principalsFromOtherProviders []apiv3.Principal) ([]apiv3.Principal, error) {
	if principalType == "group" {
		return nil, nil
	}

	fromOtherProviders := map[string]bool{}
	for _, p := range principalsFromOtherProviders {
		fromOtherProviders[p.Name] = true
	}

	queryKey := strings.ToLower(searchKey)
	var (
		matched []*apiv3.User
		err     error
	)
	if len(searchKey) > searchIndexDefaultLen {
		matched, err = l.listAllUsers(queryKey)
	} else {
		matched, err = l.listUsersByIndex(queryKey)
	}
	if err != nil {
		logrus.Infof("Failed to search User resources for %v: %v", searchKey, err)
		return nil, err
	}

	var principals []apiv3.Principal
User:
	for _, user := range matched {
		if !isLocalUser(user) {
			continue
		}
		for _, p := range user.PrincipalIDs {
			if fromOtherProviders[p] {
				continue User
			}
		}
		principalID := getLocalPrincipalID(user)
		principals = append(principals, l.toPrincipal("user", user.DisplayName, user.Username, principalID, token))
	}

	return principals, nil
}

func (l *Provider) toPrincipal(principalType, displayName, loginName, id string, token accessor.TokenAccessor) apiv3.Principal {
	if displayName == "" {
		displayName = loginName
	}

	princ := apiv3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: id},
		DisplayName:   displayName,
		LoginName:     loginName,
		Provider:      Name,
		PrincipalType: principalType,
	}
	if token != nil {
		princ.Me = common.SamePrincipal(token.GetUserPrincipal(), princ)
	}
	return princ
}

func (l *Provider) GetPrincipal(principalID string, token accessor.TokenAccessor) (apiv3.Principal, error) {
	// id looks like local://u-12345
	var name string
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return apiv3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	name = strings.TrimPrefix(parts[1], "//")

	user, err := l.userLister.Get("", name)
	if err != nil {
		return apiv3.Principal{}, err
	}

	princID := getLocalPrincipalID(user)
	princ := l.toPrincipal("user", user.DisplayName, user.Username, princID, token)
	return princ, nil
}

func (l *Provider) listAllUsers(searchKey string) ([]*apiv3.User, error) {
	allUsers, err := l.userLister.List("", labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("listing users for search %q: %w", searchKey, err)
	}

	var matched []*apiv3.User
	for _, user := range allUsers {
		if !userMatchesSearchKey(user, searchKey) {
			continue
		}
		matched = append(matched, user)
	}
	return matched, nil
}

func (l *Provider) listUsersByIndex(searchKey string) ([]*apiv3.User, error) {
	objs, err := l.userIndexer.ByIndex(userSearchIndex, searchKey)
	if err != nil {
		return nil, fmt.Errorf("indexing users for search %q: %w", searchKey, err)
	}

	matched := make([]*apiv3.User, 0, len(objs))
	for _, obj := range objs {
		user, ok := obj.(*apiv3.User)
		if !ok {
			return nil, fmt.Errorf("user index returned non-User object: %T", obj)
		}
		matched = append(matched, user)
	}
	return matched, nil
}

func (l *Provider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func userNameIndexer(obj any) ([]string, error) {
	user, ok := obj.(*apiv3.User)
	if !ok {
		return []string{}, nil
	}
	return []string{user.Username}, nil
}

func userSearchIndexer(obj any) ([]string, error) {
	user, ok := obj.(*apiv3.User)
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

func (l *Provider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []apiv3.Principal) (bool, error) {
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

func (l *Provider) GetUserExtraAttributes(userPrincipal apiv3.Principal) map[string][]string {
	return common.GetCommonUserExtraAttributes(userPrincipal)
}

// IsDisabledProvider checks if the local auth provider is currently disabled in Rancher.
// As of now, local provider can't be disabled, so this method always returns false and nil for the error.
func (l *Provider) IsDisabledProvider() (bool, error) {
	return false, nil
}

// CleanupResources deletes resources associated with the local auth provider.
func (l *Provider) CleanupResources(*apiv3.AuthConfig) error {
	return nil
}

func userMatchesSearchKey(user *apiv3.User, searchKey string) bool {
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
