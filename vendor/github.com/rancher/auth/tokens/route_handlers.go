package tokens

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rancher/auth/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

//login is a handler for route /tokens?action=login and returns the jwt token after authenticating the user
func (server *tokenAPIServer) login(w http.ResponseWriter, r *http.Request) {

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("login failed with error: %v", err)
		util.ReturnHTTPError(w, r, http.StatusBadRequest, fmt.Sprintf("Error reading input json data: %v", err))
		return
	}
	jsonInput := v3.LoginInput{}

	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		log.Errorf("unmarshal failed with error: %v", err)
		util.ReturnHTTPError(w, r, http.StatusBadRequest, fmt.Sprintf("Error unmarshaling input json data: %v", err))
		return
	}

	var token v3.Token
	var status int
	token, status, err = server.createLoginToken(jsonInput)

	if err != nil {
		log.Errorf("Login failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	if jsonInput.ResponseType == "cookie" {

		tokenCookie := &http.Cookie{
			Name:     "rAuthnSessionToken",
			Value:    token.TokenID,
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else {

		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.Encode(token)
	}
}

func (server *tokenAPIServer) deriveToken(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie("rAuthnSessionToken")
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "No valid token cookie")
		return
	}

	log.Infof("token cookie: %v %v", cookie.Name, cookie.Value)

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("GetToken failed with error: %v", err)
	}
	jsonInput := v3.Token{}

	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		log.Errorf("unmarshal failed with error: %v", err)
	}

	var token v3.Token
	var status int

	// create derived token
	token, status, err = server.createDerivedToken(jsonInput, cookie.Value)

	if err != nil {
		log.Errorf("deriveToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(token)

}

func (server *tokenAPIServer) listTokens(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("rAuthnSessionToken")
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	log.Infof("token cookie: %v %v", cookie.Name, cookie.Value)

	//getToken
	tokens, status, err := server.getTokens(cookie.Value)
	if err != nil {
		log.Errorf("GetToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(tokens)

}

func (server *tokenAPIServer) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("rAuthnSessionToken")
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	log.Infof("token cookie: %v %v", cookie.Name, cookie.Value)

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     "rAuthnSessionToken",
		Value:    "",
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
	}
	http.SetCookie(w, tokenCookie)

	//getToken
	status, err := server.deleteToken(cookie.Value)
	if err != nil {
		log.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

}

func (server *tokenAPIServer) listIdentities(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("rAuthnSessionToken")
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	log.Infof("token cookie: %v %v", cookie.Name, cookie.Value)

	//getToken
	identities, status, err := server.getIdentities(cookie.Value)
	if err != nil {
		log.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(identities)
}
