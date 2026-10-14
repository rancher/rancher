package audit

import (
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

func TestGetUserInfoUsername(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := request.WithUser(req.Context(), &user.DefaultInfo{
		Name:   "user-abc",
		Groups: []string{"system:authenticated"},
		Extra:  map[string][]string{"username": {"admin"}},
	})

	got := getUserInfo(req.WithContext(ctx))

	if got.Name != "user-abc" {
		t.Fatalf("Name = %q, want user-abc", got.Name)
	}
	if got.Username != "admin" {
		t.Fatalf("Username = %q, want admin", got.Username)
	}
}

func TestUsernameFromInfo(t *testing.T) {
	extra := map[string][]string{"username": {"from-extra"}}
	base := &user.DefaultInfo{Name: "u-1", UID: "u-1", Groups: []string{"g"}, Extra: extra}

	tests := []struct {
		name string
		info user.Info
		want string
	}{
		{name: "explicit name wins over extras", info: WithUsername(base, "explicit"), want: "explicit"},
		{name: "empty explicit name falls back to extras", info: WithUsername(base, ""), want: "from-extra"},
		{name: "plain info uses extras", info: base, want: "from-extra"},
		{name: "plain info without extras", info: &user.DefaultInfo{Name: "u-2"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UsernameFromInfo(tt.info))
		})
	}
}

func TestWithUsernameKeepsUserInfo(t *testing.T) {
	base := &user.DefaultInfo{Name: "u-1", UID: "uid-1", Groups: []string{"g"}, Extra: map[string][]string{"k": {"v"}}}

	got := WithUsername(base, "explicit")

	assert.Equal(t, "u-1", got.GetName())
	assert.Equal(t, "uid-1", got.GetUID())
	assert.Equal(t, []string{"g"}, got.GetGroups())
	assert.Equal(t, map[string][]string{"k": {"v"}}, got.GetExtra())
	assert.Same(t, base, WithUsername(base, ""), "an empty name must return the info unchanged")
	assert.Nil(t, WithUsername(nil, "explicit"))
}

func TestGetUserInfoPrefersExplicitUsername(t *testing.T) {
	info := WithUsername(&user.DefaultInfo{
		Name:  "u-1",
		Extra: map[string][]string{"username": {"from-extra"}},
	}, "explicit")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(request.WithUser(req.Context(), info))

	got := getUserInfo(req)

	assert.Equal(t, "u-1", got.Name)
	assert.Equal(t, "explicit", got.Username)
	assert.Equal(t, map[string][]string{"username": {"from-extra"}}, got.Extra, "extras must not be changed")
}

func TestFillUsername(t *testing.T) {
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

	tests := []struct {
		name     string
		resolver UserResolver
		user     User
		want     string
	}{
		{name: "username from the user record", resolver: resolver, user: User{Name: "u-local"}, want: "admin"},
		{name: "display name when the record has no username", resolver: resolver, user: User{Name: "u-external"}, want: "Jane Doe"},
		{name: "empty when the record has neither", resolver: resolver, user: User{Name: "u-nameless"}, want: ""},
		{name: "empty when the user does not exist", resolver: resolver, user: User{Name: "system:unauthenticated"}, want: ""},
		{name: "existing username is kept", resolver: resolver, user: User{Name: "u-local", Username: "from-extra"}, want: "from-extra"},
		{name: "empty name is not resolved", resolver: resolver, user: User{}, want: ""},
		{name: "no resolver", resolver: nil, user: User{Name: "u-local"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lh := &LoggingHandler{resolveUser: tt.resolver}
			lh.fillUsername(&tt.user)
			assert.Equal(t, tt.want, tt.user.Username)
		})
	}
}

func TestUserSetIdentity(t *testing.T) {
	groups := []string{"g1", "g2"}
	extra := map[string][]string{"username": {"alice"}}
	info := &user.DefaultInfo{Name: "u-1", Groups: groups, Extra: extra}

	u := &User{
		Name:           "old",
		Username:       "old-name",
		Group:          []string{"old"},
		Extra:          map[string][]string{"old": {"x"}},
		ImpersonatedBy: &Impersonator{Name: "keep"},
		RequestUser:    "as-user",
	}
	u.SetIdentity(info)

	assert.Equal(t, "u-1", u.Name)
	assert.Equal(t, "alice", u.Username)
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
	real := &user.DefaultInfo{Name: "u-real", Groups: realGroups, Extra: map[string][]string{"username": {"alice"}, "requesttokenid": {"token-1"}}}
	effective := &user.DefaultInfo{Name: "u-target", Groups: []string{"system:authenticated"}}

	u := &User{Extra: real.Extra}
	u.SetIdentity(real)
	u.Impersonate(real, effective)

	assert.Equal(t, "u-target", u.Name)
	assert.Empty(t, u.Username, "the caller's username must not carry over to the impersonated user")
	assert.Equal(t, []string{"system:authenticated"}, u.Group)
	assert.Equal(t, real.Extra, u.Extra, "Extra is captured at entry and keeps its content")
	assert.Equal(t, &Impersonator{Name: "u-real", Username: "alice", Group: realGroups}, u.ImpersonatedBy)

	u.clearGroups()
	assert.Nil(t, u.Group)
	assert.Nil(t, u.ImpersonatedBy.Group)
	assert.Equal(t, []string{"real-group"}, realGroups, "the caller's groups must not be modified in place")
}

func TestUserImpersonatedByJSON(t *testing.T) {
	plain, err := json.Marshal(&User{Name: "u-1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"u-1"}`, string(plain), "impersonatedBy must be omitted when there is no impersonation")

	imp, err := json.Marshal(&User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real", Username: "alice", Group: []string{"g"}}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"u-target","impersonatedBy":{"name":"u-real","username":"alice","group":["g"]}}`, string(imp))
}

func TestFillUsernameImpersonator(t *testing.T) {
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

	u := &User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real"}}
	lh.fillUsername(u)
	assert.Equal(t, "target", u.Username)
	assert.Equal(t, "Real Person", u.ImpersonatedBy.Username)

	// An impersonator that already has a username keeps it.
	u = &User{Name: "u-target", ImpersonatedBy: &Impersonator{Name: "u-real", Username: "alice"}}
	lh.fillUsername(u)
	assert.Equal(t, "alice", u.ImpersonatedBy.Username)
}

func TestMiddlewareFillsUsernameFromResolver(t *testing.T) {
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
			handler := NewAuditLogMiddleware(writer, WithUserResolver(resolver))(
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
				assert.NotContains(t, entry.User, "username")
			} else {
				assert.Equal(t, tt.wantUsername, entry.User["username"])
			}
		})
	}
}
