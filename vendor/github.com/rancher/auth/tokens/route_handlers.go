package tokens

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rancher/auth/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const CookieName = "R_SESS"

//login is a handler for route /tokens?action=login and returns the jwt token after authenticating the user
func (s *tokenAPIServer) login(w http.ResponseWriter, r *http.Request) {

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
	token, status, err = s.createLoginToken(jsonInput)

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
			Name:     CookieName,
			Value:    token.Name,
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

func (s *tokenAPIServer) deriveToken(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "No valid token cookie")
		return
	}
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
	token, status, err = s.createDerivedToken(jsonInput, cookie.Value)
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

func (s *tokenAPIServer) listTokens(w http.ResponseWriter, r *http.Request) {
	// TODO switch to X-API-UserId header
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}
	//getToken
	tokens, status, err := s.getTokens(cookie.Value)
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

func (s *tokenAPIServer) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
	}
	http.SetCookie(w, tokenCookie)

	//getToken
	status, err := s.deleteToken(cookie.Value)
	if err != nil {
		log.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

}

func (s *tokenAPIServer) getToken(w http.ResponseWriter, r *http.Request) {
	// TODO switch to X-API-UserId header
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	vars := mux.Vars(r)
	tokenID := vars["tokenId"]

	//getToken
	tokens, status, err := s.getDerivedToken(cookie.Value, tokenID)
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

func (s *tokenAPIServer) removeToken(w http.ResponseWriter, r *http.Request) {
	// TODO switch to X-API-UserId header
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		log.Info("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}
	vars := mux.Vars(r)
	tokenID := vars["tokenId"]

	//deleteToken
	status, err := s.deleteDerivedToken(cookie.Value, tokenID)
	if err != nil {
		log.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}
}
