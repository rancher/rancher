package ui

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
)

var (
	authToTarget = map[string]string{
		"vue":   "/dashboard/auth/verify",
		"ember": "/verify",
	}
)

func redirectAuth(rw http.ResponseWriter, req *http.Request) {
	// Get state from query parameter instead of mux.Vars
	state := req.URL.Query().Get("state")
	bytes, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		emberIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	input := struct {
		To string `json:"to,omitempty"`
	}{}
	if err := json.Unmarshal(bytes, &input); err != nil || authToTarget[input.To] == "" {
		emberIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	u := url.URL{
		Path:     authToTarget[input.To],
		RawQuery: req.URL.RawQuery,
	}
	http.Redirect(rw, req, u.String(), http.StatusFound)
}
