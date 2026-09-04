package common

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// UserLister lists v3.User resources, normally from an informer cache.
type UserLister interface {
	List(namespace string, selector labels.Selector) ([]*apiv3.User, error)
}

// UserSearcher resolves principals from the v3.User resources Rancher has
// already created. Providers whose identity provider offers no user lookup use
// it to return the real principals of users that have logged in or been
// provisioned, rather than only a principal built from the search key.
type UserSearcher struct {
	users UserLister
}

// NewUserSearcher returns a UserSearcher backed by the given lister.
func NewUserSearcher(users UserLister) *UserSearcher {
	return &UserSearcher{users: users}
}

// SearchPrincipals returns the user principals owned by provider that belong to
// users matching searchKey, ordered by principal ID. A user contributes a
// principal only if it carries a principal ID for provider, so users known to
// Rancher through some other provider are left out. A search key that is empty
// or only whitespace matches nothing, to avoid returning every user.
func (s *UserSearcher) SearchPrincipals(provider, searchKey string) ([]apiv3.Principal, error) {
	queryKey := strings.ToLower(searchKey)
	normalizedQueryKey := normalizeSearchKey(queryKey)

	// A key that normalizes to nothing, because it is empty or only
	// whitespace, is contained in every name and would return every user.
	if normalizedQueryKey == "" {
		return nil, nil
	}

	users, err := s.users.List("", labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("listing users for search %q: %w", searchKey, err)
	}

	prefix := provider + "_" + UserPrincipalType + "://"

	var principals []apiv3.Principal
	for _, user := range users {
		// Users of other providers cannot contribute a principal, so skip them
		// before normalizing their display name to match it.
		if !hasPrincipalWithPrefix(user, prefix) {
			continue
		}

		if !userMatchesSearchKey(user, queryKey, normalizedQueryKey) {
			continue
		}

		for _, principalID := range user.PrincipalIDs {
			if !strings.HasPrefix(principalID, prefix) {
				continue
			}

			principals = append(principals, apiv3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: principalID},
				DisplayName:   user.DisplayName,
				PrincipalType: UserPrincipalType,
				Provider:      provider,
			})
		}
	}

	// The lister iterates the informer cache in no particular order, so sort to
	// keep repeated searches in the same order.
	slices.SortFunc(principals, func(a, b apiv3.Principal) int {
		return strings.Compare(a.Name, b.Name)
	})

	return principals, nil
}

// PrincipalsWithFallback returns the principals of provider's users matching
// searchKey, followed by fallback unless one of them already has its ID.
// Providers whose identity provider offers no user lookup call this with a
// principal built from the search key, so an admin can still enter an external
// ID by hand for an identity Rancher has not seen yet. A nil searcher yields
// only the fallback.
func PrincipalsWithFallback(searcher *UserSearcher, provider, searchKey string, fallback apiv3.Principal) ([]apiv3.Principal, error) {
	var principals []apiv3.Principal

	if searcher != nil {
		known, err := searcher.SearchPrincipals(provider, searchKey)
		if err != nil {
			return nil, err
		}
		principals = known
	}

	if !ContainsPrincipal(principals, fallback.Name) {
		principals = append(principals, fallback)
	}

	return principals, nil
}

func hasPrincipalWithPrefix(user *apiv3.User, prefix string) bool {
	return slices.ContainsFunc(user.PrincipalIDs, func(id string) bool {
		return strings.HasPrefix(id, prefix)
	})
}

// ContainsPrincipal reports whether principals holds a principal with the given
// resource name.
func ContainsPrincipal(principals []apiv3.Principal, name string) bool {
	return slices.ContainsFunc(principals, func(p apiv3.Principal) bool {
		return p.Name == name
	})
}

// UserMatchesSearchKey reports whether the user's resource name, login username
// or display name matches searchKey. The search key is expected to be lower
// case. Whitespace and diacritics are ignored so that a display name can be
// found however it is typed.
//
// Callers that test many users against one search key should normalize the key
// once with normalizeSearchKey and call userMatchesSearchKey instead.
func UserMatchesSearchKey(user *apiv3.User, searchKey string) bool {
	return userMatchesSearchKey(user, searchKey, normalizeSearchKey(searchKey))
}

// userMatchesSearchKey takes both the raw search key, matched against the user's
// resource name, and its normalized form, matched against the login username and
// display name.
func userMatchesSearchKey(user *apiv3.User, searchKey, normalizedSearchKey string) bool {
	// Every name contains the empty string, so a key that normalized to nothing
	// is no search rather than a match on everything.
	if normalizedSearchKey == "" {
		return false
	}

	normalizedDisplayName := normalizeSearchKey(user.DisplayName)

	return strings.HasPrefix(user.ObjectMeta.Name, searchKey) ||
		strings.Contains(strings.ToLower(normalizeWhitespace(user.Username)), normalizedSearchKey) ||
		strings.Contains(normalizedDisplayName, normalizedSearchKey)
}

func normalizeSearchKey(s string) string {
	return strings.ToLower(normalizeWhitespace(SimplifyString(s)))
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// SimplifyString transforms unicode characters in the string by replacing
// the characters.
//
// The set of characters that is replaced (unicode.Mn) is here
//
//	https://www.compart.com/en/unicode/category/Mn
func SimplifyString(s string) string {
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
