package audit

import (
	"testing"
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
