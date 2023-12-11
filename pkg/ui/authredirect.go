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

	// These were set by the UI, sent to the OAuth service and came back here
	input := struct {
		To string `json:"to,omitempty"`
		// PublicKey is the temporary pub key generated by Rancher CLI to
		// encrypt the token that will be created by Rancher once the OAuth flow
		// is verified.
		PublicKey string `json:"publicKey"`
		// RequestID is the name of the SamlToken that will be created. This is
		// generated by the CLI client.
		RequestID string `json:"requestId"`
		// ResponseType stores the type of token that must be created (eg: which
		// cluster)
		ResponseType string `json:"responseType"`
	}{}
	if err := json.Unmarshal(bytes, &input); err != nil || authToTarget[input.To] == "" {
		emberIndexUnlessAPI().ServeHTTP(rw, req)
		return
	}

	if input.PublicKey != "" {
		http.SetCookie(rw, &http.Cookie{
			Name:     "oauth_Rancher_PublicKey",
			Value:    input.PublicKey,
			MaxAge:   90,
			HttpOnly: true,
			Secure:   true,
			// Path: "/"
		})
	}
	if input.RequestID != "" {
		http.SetCookie(rw, &http.Cookie{
			Name:     "oauth_Rancher_RequestId",
			Value:    input.RequestID,
			MaxAge:   90,
			HttpOnly: true,
			Secure:   true,
			// Path: "/"
		})
	}
	if input.ResponseType != "" {
		http.SetCookie(rw, &http.Cookie{
			Name:     "oauth_Rancher_ResponseType",
			Value:    input.ResponseType,
			MaxAge:   90,
			HttpOnly: true,
			Secure:   true,
			// Path: "/"
		})
	}

	u := url.URL{
		Path:     authToTarget[input.To],
		RawQuery: req.URL.RawQuery,
	}
	http.Redirect(rw, req, u.String(), http.StatusFound)
}
