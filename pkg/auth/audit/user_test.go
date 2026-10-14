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
