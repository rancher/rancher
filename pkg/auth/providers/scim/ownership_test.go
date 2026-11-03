package scim

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/scimconfig"
	"github.com/rancher/rancher/pkg/user/mocks"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	providerA = "okta"
	providerB = "azuread"
)

// ownershipFixtures holds the users, attributes and groups the ownership tests share.
// Every user except the system user carries local://<name>, as real users do.
type ownershipFixtures struct {
	users  map[string]*v3.User
	attrs  map[string]*v3.UserAttribute
	groups map[string]*v3.Group
}

func newOwnershipFixtures() *ownershipFixtures {
	lastLogin := metav1.NewTime(time.Now())
	groupAPrincipal := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: groupPrincipalName(providerA, "GroupA")},
		DisplayName:   "GroupA",
		MemberOf:      true,
		PrincipalType: "group",
		Provider:      providerA,
	}

	user := func(name string, principals ...string) *v3.User {
		return &v3.User{
			ObjectMeta:   metav1.ObjectMeta{Name: name},
			PrincipalIDs: append([]string{"local://" + name}, principals...),
		}
	}
	attr := func(name string, extras map[string]map[string][]string) *v3.UserAttribute {
		return &v3.UserAttribute{
			ObjectMeta:      metav1.ObjectMeta{Name: name},
			ExtraByProvider: extras,
		}
	}
	extras := func(provider, id, userName, externalID string) map[string][]string {
		return map[string][]string{
			"principalid": {userPrincipalName(provider, id)},
			"username":    {userName},
			"externalid":  {externalID},
		}
	}

	f := &ownershipFixtures{
		users: map[string]*v3.User{},
		attrs: map[string]*v3.UserAttribute{},
		groups: map[string]*v3.Group{
			"grp-a": {
				ObjectMeta:  metav1.ObjectMeta{Name: "grp-a", Labels: map[string]string{authProviderLabel: providerA}},
				DisplayName: "GroupA",
				ExternalID:  "ext-grp-a",
				Provider:    providerA,
			},
			"grp-a2": {
				ObjectMeta:  metav1.ObjectMeta{Name: "grp-a2", Labels: map[string]string{authProviderLabel: providerA}},
				DisplayName: "GroupA2",
				ExternalID:  "ext-grp-a2",
				Provider:    providerA,
			},
			"grp-b": {
				ObjectMeta:  metav1.ObjectMeta{Name: "grp-b", Labels: map[string]string{authProviderLabel: providerB}},
				DisplayName: "GroupB",
				ExternalID:  "ext-grp-b",
				Provider:    providerB,
			},
		},
	}

	// owned: created by SCIM for provider A.
	f.users["u-owned"] = user("u-owned", userPrincipalName(providerA, "u1"))
	f.attrs["u-owned"] = attr("u-owned", map[string]map[string][]string{providerA: extras(providerA, "u1", "owned.user", "ext-u1")})

	// loginOwned: created by login to provider A.
	f.users["u-loginowned"] = user("u-loginowned", userPrincipalName(providerA, "u9"))
	f.attrs["u-loginowned"] = attr("u-loginowned", map[string]map[string][]string{providerA: extras(providerA, "u9", "login.user", "ext-u9")})
	f.attrs["u-loginowned"].LastLogin = &lastLogin

	// otherProvider: created for provider B. In no group.
	f.users["u-other"] = user("u-other", userPrincipalName(providerB, "u2"))
	f.attrs["u-other"] = attr("u-other", map[string]map[string][]string{providerB: extras(providerB, "u2", "other.user", "ext-u2")})

	// otherProviderMember: created for provider B. A member of grp-a.
	f.users["u-othermember"] = user("u-othermember", userPrincipalName(providerB, "u9"))
	f.attrs["u-othermember"] = attr("u-othermember", map[string]map[string][]string{providerB: extras(providerB, "u9", "othermember.user", "ext-b-u9")})
	f.attrs["u-othermember"].GroupPrincipals = map[string]v3.Principals{providerA: {Items: []v3.Principal{groupAPrincipal}}}

	// staleExtras: holds only a provider B principal, but has a principalid extra for provider A.
	f.users["u-stale"] = user("u-stale", userPrincipalName(providerB, "u4"))
	f.attrs["u-stale"] = attr("u-stale", map[string]map[string][]string{
		providerA: extras(providerA, "u4", "stale.user", "ext-u4"),
		providerB: extras(providerB, "u4", "stale.user", "ext-u4"),
	})

	// multiProvider: holds principals of both providers.
	f.users["u-multi"] = user("u-multi", userPrincipalName(providerA, "u5"), userPrincipalName(providerB, "u5"))
	f.attrs["u-multi"] = attr("u-multi", map[string]map[string][]string{
		providerA: extras(providerA, "u5", "multi.user", "ext-u5"),
		providerB: extras(providerB, "u5", "multi.user", "ext-u5"),
	})

	// local: a local user who never linked a provider. In a cluster this user has a password
	// Secret; Secrets aren't modeled because nothing reads them. It has a stale principalid extra
	// for provider A, but no external principal.
	f.users["u-local"] = user("u-local")
	f.users["u-local"].Username = "local"
	f.attrs["u-local"] = attr("u-local", map[string]map[string][]string{providerA: extras(providerA, "u15", "local.user", "ext-u15")})

	// localLinked: a local admin who linked provider A through test-and-enable and logged in. In a
	// cluster this user has a password Secret; Secrets aren't modeled because nothing reads them.
	// A member of grp-a.
	f.users["u-locallinked"] = user("u-locallinked", userPrincipalName(providerA, "u3"))
	f.users["u-locallinked"].Username = "locallinked"
	f.attrs["u-locallinked"] = attr("u-locallinked", map[string]map[string][]string{providerA: extras(providerA, "u3", "linked.user", "ext-u3")})
	f.attrs["u-locallinked"].LastLogin = &lastLogin
	f.attrs["u-locallinked"].GroupPrincipals = map[string]v3.Principals{providerA: {Items: []v3.Principal{groupAPrincipal}}}

	// localLinkedNonMember: same as localLinked (including the password Secret, which isn't
	// modeled), but in no group.
	f.users["u-locallinkednm"] = user("u-locallinkednm", userPrincipalName(providerA, "u10"))
	f.users["u-locallinkednm"].Username = "locallinkednm"
	f.attrs["u-locallinkednm"] = attr("u-locallinkednm", map[string]map[string][]string{providerA: extras(providerA, "u10", "linkednm.user", "ext-u10")})
	f.attrs["u-locallinkednm"].LastLogin = &lastLogin

	// localLinkedPasswordField: the password is in User.Password (older data), with no Secret and
	// no Username.
	f.users["u-passwordfield"] = user("u-passwordfield", userPrincipalName(providerA, "u6"))
	f.users["u-passwordfield"].Password = "hash"
	f.attrs["u-passwordfield"] = attr("u-passwordfield", map[string]map[string][]string{providerA: extras(providerA, "u6", "passwordfield.user", "ext-u6")})

	// localLinkedUsernameOnly: Username set, no password data and no UserAttribute.
	f.users["u-usernameonly"] = user("u-usernameonly", userPrincipalName(providerA, "u7"))
	f.users["u-usernameonly"].Username = "usernameonly"

	// loginPassword: created by login to provider A and later given a local password Secret, with
	// no Username. Secrets aren't modeled because nothing reads them.
	f.users["u-loginpassword"] = user("u-loginpassword", userPrincipalName(providerA, "u11"))
	f.attrs["u-loginpassword"] = attr("u-loginpassword", map[string]map[string][]string{providerA: extras(providerA, "u11", "loginpassword.user", "ext-u11")})
	f.attrs["u-loginpassword"].LastLogin = &lastLogin

	// defaultAdmin: linked provider A through test-and-enable and logged in. In a cluster this user
	// has a password Secret; Secrets aren't modeled because nothing reads them. A member of grp-a.
	f.users["u-admin"] = user("u-admin", userPrincipalName(providerA, "u8"))
	f.users["u-admin"].Username = "admin"
	f.attrs["u-admin"] = attr("u-admin", map[string]map[string][]string{providerA: extras(providerA, "u8", "admin.user", "ext-u8")})
	f.attrs["u-admin"].LastLogin = &lastLogin
	f.attrs["u-admin"].GroupPrincipals = map[string]v3.Principals{providerA: {Items: []v3.Principal{groupAPrincipal}}}

	// system: a system user that also holds a provider A principal. In no group.
	f.users["u-system"] = &v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "u-system"},
		PrincipalIDs: []string{"system://local", userPrincipalName(providerA, "u12")},
	}
	f.attrs["u-system"] = attr("u-system", map[string]map[string][]string{providerA: extras(providerA, "u12", "system.user", "ext-u12")})

	return f
}

func (f *ownershipFixtures) userCache(ctrl *gomock.Controller) *fake.MockNonNamespacedCacheInterface[*v3.User] {
	userCache := fake.NewMockNonNamespacedCacheInterface[*v3.User](ctrl)
	userCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.User, error) {
		if user, ok := f.users[name]; ok {
			return user, nil
		}
		return nil, apierrors.NewNotFound(v3.Resource("user"), name)
	}).AnyTimes()
	userCache.EXPECT().List(labels.Everything()).DoAndReturn(func(labels.Selector) ([]*v3.User, error) {
		list := make([]*v3.User, 0, len(f.users))
		for _, user := range f.users {
			list = append(list, user)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		return list, nil
	}).AnyTimes()

	return userCache
}

func (f *ownershipFixtures) userAttributeCache(ctrl *gomock.Controller) *fake.MockNonNamespacedCacheInterface[*v3.UserAttribute] {
	userAttributeCache := fake.NewMockNonNamespacedCacheInterface[*v3.UserAttribute](ctrl)
	userAttributeCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.UserAttribute, error) {
		if attr, ok := f.attrs[name]; ok {
			return attr, nil
		}
		return nil, apierrors.NewNotFound(v3.Resource("userattribute"), name)
	}).AnyTimes()

	return userAttributeCache
}

func (f *ownershipFixtures) groupsCache(ctrl *gomock.Controller) *fake.MockNonNamespacedCacheInterface[*v3.Group] {
	groupsCache := fake.NewMockNonNamespacedCacheInterface[*v3.Group](ctrl)
	groupsCache.EXPECT().Get(gomock.Any()).DoAndReturn(func(name string) (*v3.Group, error) {
		if group, ok := f.groups[name]; ok {
			return group, nil
		}
		return nil, apierrors.NewNotFound(v3.Resource("group"), name)
	}).AnyTimes()
	groupsCache.EXPECT().List(gomock.Any()).DoAndReturn(func(selector labels.Selector) ([]*v3.Group, error) {
		var list []*v3.Group
		for _, group := range f.groups {
			if selector.Matches(labels.Set(group.Labels)) {
				list = append(list, group)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
		return list, nil
	}).AnyTimes()

	return groupsCache
}

// ensureAndGetUserAttribute stands in for userMGR.EnsureAndGetUserAttribute and reads the fixtures.
func (f *ownershipFixtures) ensureAndGetUserAttribute(name string) (*v3.UserAttribute, bool, error) {
	if attr, ok := f.attrs[name]; ok {
		return attr, false, nil
	}
	return &v3.UserAttribute{
		ObjectMeta:      metav1.ObjectMeta{Name: name},
		GroupPrincipals: map[string]v3.Principals{},
		ExtraByProvider: map[string]map[string][]string{},
	}, true, nil
}

// ownershipServer is a SCIMServer backed by the fixtures.
// The clients and the user manager have no expectations, so any write fails the test unless the
// test adds one.
type ownershipServer struct {
	*SCIMServer
	users          *fake.MockNonNamespacedClientInterface[*v3.User, *v3.UserList]
	userAttributes *fake.MockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList]
	groups         *fake.MockNonNamespacedClientInterface[*v3.Group, *v3.GroupList]
	userMGR        *mocks.MockManager
}

func (f *ownershipFixtures) server(ctrl *gomock.Controller) *ownershipServer {
	srv := &ownershipServer{
		users:          fake.NewMockNonNamespacedClientInterface[*v3.User, *v3.UserList](ctrl),
		userAttributes: fake.NewMockNonNamespacedClientInterface[*v3.UserAttribute, *v3.UserAttributeList](ctrl),
		groups:         fake.NewMockNonNamespacedClientInterface[*v3.Group, *v3.GroupList](ctrl),
		userMGR:        mocks.NewMockManager(ctrl),
	}
	srv.SCIMServer = &SCIMServer{
		groups:             srv.groups,
		groupsCache:        f.groupsCache(ctrl),
		userCache:          f.userCache(ctrl),
		users:              srv.users,
		userAttributeCache: f.userAttributeCache(ctrl),
		userAttributes:     srv.userAttributes,
		userMGR:            srv.userMGR,
		getConfig:          testDefaultGetConfig,
	}

	return srv
}

// newSCIMRequest builds a SCIM request for the provider.
// The id path value is set when id isn't empty.
func newSCIMRequest(t *testing.T, method, provider, endpoint, id string, body any) *http.Request {
	t.Helper()

	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		require.NoError(t, err)
	}

	path := "/v1-scim/" + provider + "/" + endpoint
	if id != "" {
		path += "/" + id
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.SetPathValue("provider", provider)
	if id != "" {
		r.SetPathValue("id", id)
	}

	return r
}

var externalIDUserConfig = func(string) scimconfig.Config {
	return scimconfig.Config{UserIDAttribute: scimconfig.UserIDExternalID}
}

var externalIDGroupConfig = func(string) scimconfig.Config {
	return scimconfig.Config{GroupIDAttribute: scimconfig.GroupIDExternalID}
}

func TestUserOwnedBy(t *testing.T) {
	tests := []struct {
		user     string
		provider string
		want     bool
	}{
		{user: "u-owned", provider: providerA, want: true},
		{user: "u-loginowned", provider: providerA, want: true},
		{user: "u-multi", provider: providerA, want: true},
		{user: "u-multi", provider: providerB, want: true},
		{user: "u-locallinked", provider: providerA, want: true},
		{user: "u-locallinkednm", provider: providerA, want: true},
		{user: "u-passwordfield", provider: providerA, want: true},
		{user: "u-usernameonly", provider: providerA, want: true},
		{user: "u-loginpassword", provider: providerA, want: true},
		{user: "u-other", provider: providerA, want: false},
		{user: "u-other", provider: providerB, want: true},
		{user: "u-othermember", provider: providerA, want: false},
		{user: "u-stale", provider: providerA, want: false},
		{user: "u-local", provider: providerA, want: false},
		{user: "u-admin", provider: providerA, want: false},
		{user: "u-system", provider: providerA, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.user+" for "+tt.provider, func(t *testing.T) {
			f := newOwnershipFixtures()
			assert.Equal(t, tt.want, userOwnedBy(tt.provider, f.users[tt.user]))
		})
	}
}

func TestUserOwnedByProviderNamePrefix(t *testing.T) {
	// githubapp starts with github, but its principals don't belong to github.
	user := &v3.User{
		ObjectMeta:   metav1.ObjectMeta{Name: "u-githubapp"},
		PrincipalIDs: []string{"local://u-githubapp", userPrincipalName("githubapp", "u13")},
	}

	assert.False(t, userOwnedBy("github", user))
}

func TestGroupOwnedBy(t *testing.T) {
	f := newOwnershipFixtures()

	assert.True(t, groupOwnedBy(providerA, f.groups["grp-a"]))
	assert.False(t, groupOwnedBy(providerA, f.groups["grp-b"]))
	assert.True(t, groupOwnedBy(providerB, f.groups["grp-b"]))
	assert.False(t, groupOwnedBy(providerA, &v3.Group{ObjectMeta: metav1.ObjectMeta{Name: "grp-nolabel"}, Provider: providerA}))
}

func TestGetOwnedUser(t *testing.T) {
	for _, id := range []string{"u-owned", "u-locallinked"} {
		t.Run("returns owned user "+id, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			srv := newOwnershipFixtures().server(ctrl)

			user, err := srv.getOwnedUser(providerA, id)
			require.NoError(t, err)
			assert.Equal(t, id, user.Name)
		})
	}

	for _, id := range []string{"u-admin", "u-other", "u-system", "u-missing"} {
		t.Run("returns not found for "+id, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			srv := newOwnershipFixtures().server(ctrl)

			user, err := srv.getOwnedUser(providerA, id)
			require.Error(t, err)
			assert.True(t, apierrors.IsNotFound(err))
			assert.Nil(t, user)
		})
	}
}

func TestGetOwnedGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	srv := newOwnershipFixtures().server(ctrl)

	group, err := srv.getOwnedGroup(providerA, "grp-a")
	require.NoError(t, err)
	assert.Equal(t, "grp-a", group.Name)

	for _, id := range []string{"grp-b", "grp-missing"} {
		group, err := srv.getOwnedGroup(providerA, id)
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err), id)
		assert.Nil(t, group)
	}
}
