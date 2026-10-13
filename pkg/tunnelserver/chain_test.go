package tunnelserver

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorizers_OnAuthorized(t *testing.T) {
	authorizer := func(key string, authed bool, err error) func(*http.Request) (string, bool, error) {
		return func(*http.Request) (string, bool, error) { return key, authed, err }
	}

	tests := []struct {
		name     string
		chain    []func(*http.Request) (string, bool, error)
		expected []string
	}{
		{
			name:     "no authorizers",
			expected: nil,
		},
		{
			name:     "authorized",
			chain:    []func(*http.Request) (string, bool, error){authorizer("stv-cluster-c-m-abc12", true, nil)},
			expected: []string{"stv-cluster-c-m-abc12"},
		},
		{
			name: "unauthorized authorizers do not notify",
			chain: []func(*http.Request) (string, bool, error){
				authorizer("", false, nil),
				authorizer("", false, errors.New("nope")),
			},
			expected: nil,
		},
		{
			name: "only the authorizer that succeeds notifies",
			chain: []func(*http.Request) (string, bool, error){
				authorizer("wrong", false, errors.New("nope")),
				authorizer("c-m-abc12", true, nil),
				authorizer("never-reached", true, nil),
			},
			expected: []string{"c-m-abc12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Authorizers{}
			for _, auth := range tt.chain {
				a.Add(auth)
			}

			var got []string
			a.OnAuthorized(func(clientKey string) { got = append(got, clientKey) })

			req, err := http.NewRequest(http.MethodGet, "http://not-used/v3/connect", nil)
			assert.NoError(t, err)
			a.Authorize(req)

			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAuthorizers_OnAuthorized_allCallbacks(t *testing.T) {
	a := &Authorizers{}
	a.Add(func(*http.Request) (string, bool, error) { return "c-m-abc12", true, nil })

	var first, second string
	a.OnAuthorized(func(clientKey string) { first = clientKey })
	a.OnAuthorized(func(clientKey string) { second = clientKey })

	req, err := http.NewRequest(http.MethodGet, "http://not-used/v3/connect", nil)
	assert.NoError(t, err)
	a.Authorize(req)

	assert.Equal(t, "c-m-abc12", first)
	assert.Equal(t, "c-m-abc12", second)
}
