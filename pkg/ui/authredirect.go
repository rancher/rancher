package ui

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
)

const vueAuthVerifyPath = "/dashboard/auth/verify"

func redirectAuth(rw http.ResponseWriter, req *http.Request) {
	state := req.URL.Query().Get("state")
	bytes, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		vueIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	input := struct {
		To string `json:"to,omitempty"`
	}{}
	if err := json.Unmarshal(bytes, &input); err != nil || input.To != "vue" {
		vueIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	u := url.URL{
		Path:     vueAuthVerifyPath,
		RawQuery: req.URL.RawQuery,
	}
	http.Redirect(rw, req, u.String(), http.StatusFound)
}
