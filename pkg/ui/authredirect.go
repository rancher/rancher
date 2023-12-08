package ui

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

var (
	authToTarget = map[string]string{
		"vue":   "/dashboard/auth/verify",
		"ember": "/verify",
	}
)

func redirectAuth(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	bytes, err := base64.RawURLEncoding.DecodeString(vars["state"])
	if err != nil {
		emberIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	input := struct {
		To        string `json:"to,omitempty"`
		PublicKey string `json:"publicKey"`
		RequestID string `json:"requestId"`
	}{}
	if err := json.Unmarshal(bytes, &input); err != nil || authToTarget[input.To] == "" {
		emberIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	http.SetCookie(rw, &http.Cookie{
		Name:     "oauth_Rancher_PublicKey",
		Value:    input.PublicKey,
		MaxAge:   90,
		HttpOnly: true,
		Secure:   true,
		// Path: "/"
	})
	http.SetCookie(rw, &http.Cookie{
		Name:     "oauth_Rancher_RequestId",
		Value:    input.RequestID,
		MaxAge:   90,
		HttpOnly: true,
		Secure:   true,
		// Path: "/"
	})

	u := url.URL{
		Path:     authToTarget[input.To],
		RawQuery: req.URL.RawQuery,
	}
	http.Redirect(rw, req, u.String(), http.StatusFound)
}
