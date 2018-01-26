package principals

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"

	"github.com/rancher/auth/tokens"
	"github.com/rancher/auth/util"
)

func (server *principalAPIServer) listPrincipals(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(tokens.CookieName)
	if err != nil {
		logrus.Errorf("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	//getPrincipals
	principals, status, err := server.getPrincipals(cookie.Value)
	if err != nil {
		logrus.Errorf("listPrincipals failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(principals)
}

func (server *principalAPIServer) searchPrincipals(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(tokens.CookieName)
	if err != nil {
		logrus.Errorf("Failed to get token cookie: %v", err)
		util.ReturnHTTPError(w, r, http.StatusUnauthorized, "Invalid token cookie")
		return
	}

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("searchPrincipals failed with error: %v", err)
		util.ReturnHTTPError(w, r, http.StatusBadRequest, fmt.Sprintf("Error reading input json data: %v", err))
		return
	}

	var jsonInput map[string]string
	if len(bytes) > 0 {
		err = json.Unmarshal(bytes, &jsonInput)
		if err != nil {
			logrus.Errorf("searchPrincipals: Error unmarshalling json request body: %v", err)
			util.ReturnHTTPError(w, r, http.StatusBadRequest, fmt.Sprintf("Error reading json request body: %v", err))
			return
		}
	} else {
		util.ReturnHTTPError(w, r, http.StatusBadRequest, fmt.Sprintf("Error reading input json data: %v", err))
		return
	}

	//searchPrincipals
	principals, status, err := server.findPrincipals(cookie.Value, jsonInput["name"])
	if err != nil {
		logrus.Errorf("searchPrincipals failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		util.ReturnHTTPError(w, r, status, fmt.Sprintf("%v", err))
		return
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(principals)
}
