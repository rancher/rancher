package tokens

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/auth/model"
	"github.com/rancher/auth/util"
)

const CookieName = "R_SESS"

func TokenActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	logrus.Infof("TokenActionHandler called for action %v", actionName)

	if actionName == "login" {
		return tokenServer.login(actionName, action, request)
	} else if actionName == "logout" {
		return tokenServer.logout(actionName, action, request)
	}
	return nil
}

func TokenCreateHandler(request *types.APIContext) error {
	logrus.Infof("TokenCreateHandler called")
	return tokenServer.deriveToken(request)
}

func TokenListHandler(request *types.APIContext) error {
	logrus.Infof("TokenListHandler called")
	if request.ID != "" {
		return tokenServer.getToken(request)
	}
	return tokenServer.listTokens(request)
}

func TokenDeleteHandler(request *types.APIContext) error {
	logrus.Infof("TokenDeleteHandler called")
	return tokenServer.removeToken(request)
}

//login is a handler for route /tokens?action=login and returns the jwt token after authenticating the user
func (s *tokenAPIServer) login(actionName string, action *types.Action, request *types.APIContext) error {

	r := request.Request
	w := request.Response

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("login failed with error: %v", err)
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("Error reading input json data: %v", err))
	}
	jsonInput := v3.LoginInput{}

	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("Error unmarshaling input json data: %v", err))
	}

	var token v3.Token
	var status int
	token, status, err = s.createLoginToken(jsonInput)

	if err != nil {
		logrus.Errorf("Login failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	if jsonInput.ResponseType == "cookie" {
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    token.ObjectMeta.Name + ":" + token.Token,
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else {
		tokenData, err := convertTokenResource(request, token)
		if err != nil {
			return err
		}
		tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token
		request.WriteResponse(http.StatusCreated, tokenData)
	}

	return nil
}

func (s *tokenAPIServer) deriveToken(request *types.APIContext) error {

	r := request.Request

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		logrus.Infof("Failed to get token cookie: %v", err)
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie")
	}
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("GetToken failed with error: %v", err)
	}
	jsonInput := v3.Token{}

	err = json.Unmarshal(bytes, &jsonInput)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
	}

	var token v3.Token
	var status int

	// create derived token
	token, status, err = s.createDerivedToken(jsonInput, cookie.Value)
	if err != nil {
		logrus.Errorf("deriveToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	tokenData, err := convertTokenResource(request, token)
	if err != nil {
		return err
	}
	tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token

	request.WriteResponse(http.StatusCreated, tokenData)

	return nil
}

func (s *tokenAPIServer) listTokens(request *types.APIContext) error {
	r := request.Request

	// TODO switch to X-API-UserId header
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		logrus.Infof("Failed to get token cookie: %v", err)
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie")
	}
	//getToken
	tokens, status, err := s.getTokens(cookie.Value)
	if err != nil {
		logrus.Errorf("GetToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	tokensFromStore := make([]map[string]interface{}, len(tokens))
	for _, token := range tokens {
		tokenData, err := convertTokenResource(request, token)
		if err != nil {
			return err
		}

		tokensFromStore = append(tokensFromStore, tokenData)
	}

	request.WriteResponse(http.StatusOK, tokensFromStore)
	return nil
}

func (s *tokenAPIServer) logout(actionName string, action *types.Action, request *types.APIContext) error {
	r := request.Request
	w := request.Response

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		logrus.Infof("Failed to get token cookie: %v", err)
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie")
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
		logrus.Errorf("DeleteToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}
	return nil
}

func (s *tokenAPIServer) getToken(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		logrus.Infof("Failed to get token cookie: %v", err)
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie")
	}

	tokenID := request.ID

	//getToken
	token, status, err := s.getTokenByID(cookie.Value, tokenID)
	if err != nil {
		logrus.Errorf("GetToken failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		} else if status == 410 {
			status = http.StatusNotFound
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}
	tokenData, err := convertTokenResource(request, token)
	if err != nil {
		return err
	}
	request.WriteResponse(http.StatusOK, tokenData)
	return nil
}

func (s *tokenAPIServer) removeToken(request *types.APIContext) error {
	// TODO switch to X-API-UserId header
	r := request.Request

	cookie, err := r.Cookie(CookieName)
	if err != nil {
		logrus.Infof("Failed to get token cookie: %v", err)
		return httperror.NewAPIErrorLong(http.StatusUnauthorized, util.GetHTTPErrorCode(http.StatusUnauthorized), "No valid token cookie")
	}
	tokenID := request.ID

	//getToken
	_, status, err := s.getTokenByID(cookie.Value, tokenID)
	if err != nil {
		if status != 410 {
			logrus.Errorf("DeleteToken Failed to fetch the token to delete with error: %v", err)
			if status == 0 {
				status = http.StatusInternalServerError
			}
			return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
		}
	}

	tokenData, err := deleteTokenUsingStore(request, tokenID)
	if err != nil {
		return err
	}
	request.WriteResponse(http.StatusOK, tokenData)
	return nil
}

func getTokenFromStore(request *types.APIContext, tokenID string) (map[string]interface{}, error) {
	store := request.Schema.Store
	if store == nil {
		return nil, errors.New("no token store available")
	}

	tokenData, err := store.ByID(request, request.Schema, tokenID)
	if err != nil {
		return nil, err
	}

	return tokenData, nil
}

func convertTokenResource(request *types.APIContext, token v3.Token) (map[string]interface{}, error) {
	tokenData, err := convert.EncodeToMap(token)
	if err != nil {
		return nil, err
	}
	mapper := request.Schema.Mapper
	if mapper == nil {
		return nil, errors.New("no schema mapper available")
	}
	mapper.FromInternal(tokenData)

	return tokenData, nil
}

func deleteTokenUsingStore(request *types.APIContext, tokenID string) (map[string]interface{}, error) {
	store := request.Schema.Store
	if store == nil {
		return nil, errors.New("no token store available")
	}

	tokenData, err := store.Delete(request, request.Schema, tokenID)
	if err != nil {
		return nil, err
	}
	return tokenData, nil
}

func (s *tokenAPIServer) authConfigs(w http.ResponseWriter, r *http.Request) {

	var authConfigs []model.AuthConfig

	authConfigs = append(authConfigs, model.DefaultGithubConfig())
	authConfigs = append(authConfigs, model.DefaultLocalConfig())

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(authConfigs)

}
