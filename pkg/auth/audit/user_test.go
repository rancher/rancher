package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
