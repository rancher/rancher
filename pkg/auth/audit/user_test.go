package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

var basicLoginJSON = []byte(`{
	"type": "basic",
	"username": "alice",
	"password": "secret",
	"ttl": 3600
}`)

type usernameExtractor func([]byte) string

func testUsernameExtractor(t *testing.T, extract usernameExtractor) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "valid basic login",
			body: basicLoginJSON,
			want: "alice",
		},
		{
			name: "missing username",
			body: []byte(`{"password":"x"}`),
			want: "",
		},
		{
			name: "invalid json",
			body: []byte(`{invalid json`),
			want: "",
		},
		{
			name: "empty body",
			body: []byte(``),
			want: "",
		},
		{
			name: "extra fields ignored",
			body: []byte(`{
				"username":"bob",
				"password":"x",
				"extra":"field"
			}`),
			want: "bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extract(tt.body); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_getUserNameForBasicLogin(t *testing.T) {
	testUsernameExtractor(t, getUserNameForBasicLogin)
}

func TestUsernameFromExtra(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string][]string
		want  string
	}{
		{name: "single username", extra: map[string][]string{"username": {"admin"}}, want: "admin"},
		{name: "smallest value wins", extra: map[string][]string{"username": {"user@example.com", "admin"}}, want: "admin"},
		{name: "order does not matter", extra: map[string][]string{"username": {"admin", "user@example.com"}}, want: "admin"},
		{name: "empty values are skipped", extra: map[string][]string{"username": {"", "admin"}}, want: "admin"},
		{name: "only empty values", extra: map[string][]string{"username": {"", ""}}, want: ""},
		{name: "no username extra", extra: map[string][]string{"principalid": {"local://user-abc"}}, want: ""},
		{name: "empty username list", extra: map[string][]string{"username": {}}, want: ""},
		{name: "nil extra", extra: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsernameFromExtra(tt.extra); got != tt.want {
				t.Fatalf("UsernameFromExtra() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetUserInfoIdentity(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := request.WithUser(req.Context(), &user.DefaultInfo{
		Name:   "user-abc",
		Groups: []string{"system:authenticated"},
		Extra:  map[string][]string{"username": {"admin"}},
	})

	got := getUserInfo(req.WithContext(ctx), true)

	assert.Equal(t, "user-abc", got.Name)
	assert.Equal(t, &Identity{Username: "admin"}, got.Identity, "extras only give the username, not a provider")
}

func TestIdentityFromInfo(t *testing.T) {
	extra := map[string][]string{"username": {"from-extra"}, "principalid": {"github_user://1"}}
	base := &user.DefaultInfo{Name: "u-1", UID: "u-1", Groups: []string{"g"}, Extra: extra}
	explicit := Identity{Username: "explicit", Provider: "keycloakoidc", PrincipalID: "keycloakoidc_user://abc"}

	tests := []struct {
		name string
		info user.Info
		want Identity
	}{
		{name: "explicit identity wins over extras", info: WithIdentity(base, explicit), want: explicit},
		{name: "explicit identity without username is not mixed with extras", info: WithIdentity(base, Identity{Provider: "github"}), want: Identity{Provider: "github"}},
		{name: "empty explicit identity falls back to extras", info: WithIdentity(base, Identity{}), want: Identity{Username: "from-extra"}},
		{name: "plain info uses only the username extra", info: base, want: Identity{Username: "from-extra"}},
		{name: "plain info without extras", info: &user.DefaultInfo{Name: "u-2"}, want: Identity{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IdentityFromInfo(tt.info))
		})
	}
}

func TestWithIdentityKeepsUserInfo(t *testing.T) {
	base := &user.DefaultInfo{Name: "u-1", UID: "uid-1", Groups: []string{"g"}, Extra: map[string][]string{"k": {"v"}}}
	identity := Identity{Username: "explicit", Provider: "local"}

	got := WithIdentity(base, identity)

	assert.Equal(t, "u-1", got.GetName())
	assert.Equal(t, "uid-1", got.GetUID())
	assert.Equal(t, []string{"g"}, got.GetGroups())
	assert.Equal(t, map[string][]string{"k": {"v"}}, got.GetExtra())
	assert.Same(t, base, WithIdentity(base, Identity{}), "an empty identity must return the info unchanged")
	assert.Nil(t, WithIdentity(nil, identity))
}

func TestGetUserInfoPrefersExplicitIdentity(t *testing.T) {
	identity := Identity{Username: "explicit", Provider: "github", PrincipalID: "github_user://1"}
	info := WithIdentity(&user.DefaultInfo{
		Name:  "u-1",
		Extra: map[string][]string{"username": {"from-extra"}},
	}, identity)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(request.WithUser(req.Context(), info))

	got := getUserInfo(req, true)

	assert.Equal(t, "u-1", got.Name)
	assert.Equal(t, &identity, got.Identity)
	assert.Equal(t, map[string][]string{"username": {"from-extra"}}, got.Extra, "extras must not be changed")
}

func TestFillIdentity(t *testing.T) {
	type record struct{ username, displayName string }
	users := map[string]record{
		"u-local":    {username: "admin", displayName: "Default Admin"},
		"u-external": {displayName: "Jane Doe"},
		"u-nameless": {},
	}
	resolver := func(userID string) (string, string, bool) {
		r, ok := users[userID]
		return r.username, r.displayName, ok
	}
	provided := &Identity{Username: "from-extra", Provider: "github"}

	tests := []struct {
		name     string
		resolver UserResolver
		user     User
		want     *Identity
	}{
		{name: "username from the user record", resolver: resolver, user: User{Name: "u-local", withIdentity: true}, want: &Identity{Username: "admin"}},
		{name: "display name when the record has no username", resolver: resolver, user: User{Name: "u-external", withIdentity: true}, want: &Identity{Username: "Jane Doe"}},
		{name: "empty when the record has neither", resolver: resolver, user: User{Name: "u-nameless", withIdentity: true}},
		{name: "empty when the user does not exist", resolver: resolver, user: User{Name: "system:unauthenticated", withIdentity: true}},
		{name: "existing identity is kept", resolver: resolver, user: User{Name: "u-local", Identity: provided, withIdentity: true}, want: provided},
		{name: "empty name is not resolved", resolver: resolver, user: User{withIdentity: true}},
		{name: "no resolver", resolver: nil, user: User{Name: "u-local", withIdentity: true}},
		{name: "identities are off", resolver: resolver, user: User{Name: "u-local"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lh := &LoggingHandler{resolveUser: tt.resolver}
			lh.fillIdentity(&tt.user)
			assert.Equal(t, tt.want, tt.user.Identity)
		})
	}
}

func TestUserSetIdentity(t *testing.T) {
	groups := []string{"g1", "g2"}
	extra := map[string][]string{"username": {"alice"}}
	info := &user.DefaultInfo{Name: "u-1", Groups: groups, Extra: extra}

	u := &User{
		Name:           "old",
		Identity:       &Identity{Username: "old-name"},
		Group:          []string{"old"},
		Extra:          map[string][]string{"old": {"x"}},
		ImpersonatedBy: &Impersonator{Name: "keep"},
		RequestUser:    "as-user",
		withIdentity:   true,
	}
	u.SetIdentity(info)

	assert.Equal(t, "u-1", u.Name)
	assert.Equal(t, &Identity{Username: "alice"}, u.Identity)
	assert.Equal(t, groups, u.Group)
	assert.Equal(t, map[string][]string{"old": {"x"}}, u.Extra, "SetIdentity must not touch Extra")
	assert.Equal(t, &Impersonator{Name: "keep"}, u.ImpersonatedBy, "SetIdentity must not touch ImpersonatedBy")
	assert.Equal(t, "as-user", u.RequestUser, "SetIdentity must not touch RequestUser")

	// Clearing the audit user's groups must not modify the memory it shares with the request user.
	u.clearGroups()
	assert.Nil(t, u.Group)
	assert.Equal(t, []string{"g1", "g2"}, info.Groups)
}

func TestUserImpersonate(t *testing.T) {
	realGroups := []string{"real-group"}
	realIdentity := Identity{Username: "alice", Provider: "github", PrincipalID: "github_user://1"}
	real := WithIdentity(&user.DefaultInfo{Name: "u-real", Groups: realGroups, Extra: map[string][]string{"username": {"alice"}, "requesttokenid": {"token-1"}}}, realIdentity)
	effective := &user.DefaultInfo{Name: "u-target", Groups: []string{"system:authenticated"}}

	u := &User{Extra: real.GetExtra(), withIdentity: true}
	u.SetIdentity(real)
	u.Impersonate(real, effective)

	assert.Equal(t, "u-target", u.Name)
	assert.Nil(t, u.Identity, "the caller's identity must not carry over to the impersonated user")
	assert.Equal(t, []string{"system:authenticated"}, u.Group)
	assert.Equal(t, real.GetExtra(), u.Extra, "Extra is captured at entry and keeps its content")
	assert.Equal(t, &Impersonator{Name: "u-real", Identity: &realIdentity, Group: realGroups}, u.ImpersonatedBy)

	u.clearGroups()
	assert.Nil(t, u.Group)
	assert.Nil(t, u.ImpersonatedBy.Group)
	assert.Equal(t, []string{"real-group"}, realGroups, "the caller's groups must not be modified in place")
}

func TestUserImpersonatedByJSON(t *testing.T) {
	plain, err := json.Marshal(&User{Name: "u-1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"u-1"}`, string(plain), "impersonatedBy must be omitted when there is no impersonation")

	imp, err := json.Marshal(&User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real", Identity: &Identity{Username: "alice"}, Group: []string{"g"}}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"u-target","impersonatedBy":{"name":"u-real","identity":{"username":"alice"},"group":["g"]}}`, string(imp))
}

// TestUserJSONRoundTrip pins the wire format of the audit User. Log consumers parse these keys, so
// existing ones must not change and new ones must stay additive and omitted when empty.
func TestUserJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		user User
		want string
	}{
		{
			name: "empty",
			user: User{},
			want: `{}`,
		},
		{
			name: "identity is additive to the existing keys",
			user: User{
				Name:     "user-h9q7h",
				Identity: &Identity{Username: "alice", Provider: "github", PrincipalID: "github_user://12345"},
				Group:    []string{"g1"}, Extra: map[string][]string{"username": {"alice"}},
			},
			want: `{"name":"user-h9q7h","identity":{"username":"alice","provider":"github","principalId":"github_user://12345"},` +
				`"group":["g1"],"extra":{"username":["alice"]}}`,
		},
		{
			name: "identity with only some of its fields",
			user: User{Name: "u-1", Identity: &Identity{Provider: "local"}},
			want: `{"name":"u-1","identity":{"provider":"local"}}`,
		},
		{
			name: "no identity keeps the previous shape",
			user: User{Name: "system:unauthenticated", Group: []string{"system:unauthenticated"}},
			want: `{"name":"system:unauthenticated","group":["system:unauthenticated"]}`,
		},
		{
			name: "impersonation",
			user: User{
				Name: "u-target", Identity: &Identity{Username: "target"}, Group: []string{"g-target"},
				Extra:          map[string][]string{"principalid": {"p"}},
				ImpersonatedBy: &Impersonator{Name: "u-real", Identity: &Identity{Username: "alice", Provider: "local"}, Group: []string{"g-real"}},
				RequestUser:    "u-target", RequestGroups: []string{"g-target"},
			},
			want: `{"name":"u-target","identity":{"username":"target"},"group":["g-target"],"extra":{"principalid":["p"]},` +
				`"impersonatedBy":{"name":"u-real","identity":{"username":"alice","provider":"local"},"group":["g-real"]},` +
				`"requestUser":"u-target","requestGroups":["g-target"]}`,
		},
		{
			name: "impersonator without identity or groups",
			user: User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real"}},
			want: `{"name":"u-target","impersonatedBy":{"name":"u-real"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(&tt.user)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(data))

			var got User
			require.NoError(t, json.Unmarshal(data, &got))
			assert.Equal(t, tt.user, got)
		})
	}

	t.Run("entries written before identity existed still parse", func(t *testing.T) {
		var got User
		require.NoError(t, json.Unmarshal([]byte(`{"name":"user-abc","group":["g"],"extra":{"k":["v"]}}`), &got))
		assert.Equal(t, User{Name: "user-abc", Group: []string{"g"}, Extra: map[string][]string{"k": {"v"}}}, got)
	})
}

func TestFillIdentityImpersonator(t *testing.T) {
	resolver := func(id string) (string, string, bool) {
		switch id {
		case "u-real":
			return "", "Real Person", true
		case "u-target":
			return "target", "", true
		}
		return "", "", false
	}
	lh := &LoggingHandler{resolveUser: resolver}

	u := &User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real"}, withIdentity: true}
	lh.fillIdentity(u)
	assert.Equal(t, &Identity{Username: "target"}, u.Identity)
	assert.Equal(t, &Identity{Username: "Real Person"}, u.ImpersonatedBy.Identity)

	// An impersonator that already has an identity keeps it.
	alice := &Identity{Username: "alice", Provider: "github"}
	u = &User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real", Identity: alice}, withIdentity: true}
	lh.fillIdentity(u)
	assert.Equal(t, alice, u.ImpersonatedBy.Identity)
}

func TestMiddlewareFillsIdentityFromResolver(t *testing.T) {
	resolver := func(userID string) (string, string, bool) {
		if userID == "u-external" {
			return "", "Jane Doe", true
		}
		return "", "", false
	}

	tests := []struct {
		name         string
		userName     string
		extra        map[string][]string
		wantUsername string
	}{
		{name: "empty username is resolved", userName: "u-external", wantUsername: "Jane Doe"},
		{name: "extra username wins over the record", userName: "u-external", extra: map[string][]string{"username": {"jane@example.com"}}, wantUsername: "jane@example.com"},
		{name: "unknown user stays empty", userName: "u-unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, out := newTestAuditWriter(auditlogv1.LevelNull)
			handler := NewAuditLogMiddleware(writer, WithUserResolver(resolver), WithIncludeUsername(func() bool { return true }))(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

			req := httptest.NewRequest(http.MethodGet, "/v3/clusters", nil)
			req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{
				Name:  tt.userName,
				Extra: tt.extra,
			}))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			var entry struct {
				User map[string]any `json:"user"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
			if tt.wantUsername == "" {
				assert.NotContains(t, entry.User, "identity")
			} else {
				assert.Equal(t, map[string]any{"username": tt.wantUsername}, entry.User["identity"])
			}
		})
	}
}

// TestMiddlewareOmitsIdentityByDefault checks that the entry format only grows when WithIncludeUsername
// reports true, and that no user lookup is made otherwise.
func TestMiddlewareOmitsIdentityByDefault(t *testing.T) {
	lookups := 0
	resolver := func(string) (string, string, bool) {
		lookups++
		return "resolved", "", true
	}

	writer, out := newTestAuditWriter(auditlogv1.LevelNull)
	handler := NewAuditLogMiddleware(writer, WithUserResolver(resolver))(
		http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			u, ok := FromContext(req.Context())
			require.True(t, ok)
			u.Impersonate(
				WithIdentity(&user.DefaultInfo{Name: "u-real"}, Identity{Username: "alice", Provider: "local"}),
				&user.DefaultInfo{Name: "u-target", Extra: map[string][]string{"username": {"target"}}},
			)
		}))

	req := httptest.NewRequest(http.MethodGet, "/v3/clusters", nil)
	req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "u-real"}))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	var entry struct {
		User map[string]any `json:"user"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
	assert.Equal(t, map[string]any{"name": "u-target"}, entry.User)
	assert.Zero(t, lookups)
}

// TestMiddlewareIdentityFollowsToggle checks that the option is read for every entry, so that a
// dynamic feature flag takes effect without a restart.
func TestMiddlewareUsernameFollowsToggle(t *testing.T) {
	enabled := false
	writer, out := newTestAuditWriter(auditlogv1.LevelNull)
	handler := NewAuditLogMiddleware(writer, WithIncludeUsername(func() bool { return enabled }))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	serve := func() map[string]any {
		out.Reset()
		req := httptest.NewRequest(http.MethodGet, "/v3/clusters", nil)
		req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{
			Name: "u-1", Extra: map[string][]string{"username": {"alice"}},
		}))
		handler.ServeHTTP(httptest.NewRecorder(), req)

		var entry struct {
			User map[string]any `json:"user"`
		}
		require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
		return entry.User
	}

	assert.NotContains(t, serve(), "identity")
	enabled = true
	assert.Equal(t, map[string]any{"username": "alice"}, serve()["identity"])
	enabled = false
	assert.NotContains(t, serve(), "identity")
}

// TestUserWithoutIdentityAssignsNothing checks that when identities are off the name and groups are
// still set, but no identity or impersonator is derived or assigned.
func TestUserWithoutIdentityAssignsNothing(t *testing.T) {
	real := WithIdentity(&user.DefaultInfo{Name: "u-real", Extra: map[string][]string{"username": {"alice"}}}, Identity{Username: "alice", Provider: "local"})
	effective := &user.DefaultInfo{Name: "u-target", Groups: []string{"g"}, Extra: map[string][]string{"username": {"target"}}}

	u := getUserInfo(httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		request.WithUser(context.Background(), real)), false)
	u.Impersonate(real, effective)
	(&LoggingHandler{resolveUser: func(string) (string, string, bool) {
		t.Error("the resolver must not be called")
		return "", "", false
	}}).fillIdentity(u)

	assert.Equal(t, &User{Name: "u-target", Group: []string{"g"}, Extra: real.GetExtra()}, u)
}
