package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/httperror"
	"github.com/rancher/steve/pkg/schemaserver/parse"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

const (
	csrfCookie = "CSRF"
	csrfHeader = "X-API-CSRF"
)

func ValidateAction(request *types.APIRequest) (*schemas.Action, error) {
	if request.Action == "" || request.Link != "" || request.Method != http.MethodPost {
		return nil, nil
	}

	if err := request.AccessControl.CanAction(request, request.Schema, request.Action); err != nil {
		return nil, err
	}

	actions := request.Schema.CollectionActions
	if request.Name != "" {
		actions = request.Schema.ResourceActions
	}

	action, ok := actions[request.Action]
	if !ok {
		return nil, httperror.NewAPIError(validation.InvalidAction, fmt.Sprintf("Invalid action: %s", request.Action))
	}

	return &action, nil
}

func CheckCSRF(apiOp *types.APIRequest) error {
	if !parse.IsBrowser(apiOp.Request, false) {
		return nil
	}

	cookie, err := apiOp.Request.Cookie(csrfCookie)
	if err == http.ErrNoCookie {
		bytes := make([]byte, 5)
		_, err := rand.Read(bytes)
		if err != nil {
			return httperror.WrapAPIError(err, validation.ServerError, "Failed in CSRF processing")
		}

		cookie = &http.Cookie{
			Name:   csrfCookie,
			Value:  hex.EncodeToString(bytes),
			Path:   "/",
			Secure: true,
		}

		http.SetCookie(apiOp.Response, cookie)
	} else if err != nil {
		return httperror.NewAPIError(validation.InvalidCSRFToken, "Failed to parse cookies")
	} else if apiOp.Method != http.MethodGet {
		/*
		 * Very important to use apiOp.Method and not apiOp.Request.Method. The client can override the HTTP method with _method
		 */
		if cookie.Value == apiOp.Request.Header.Get(csrfHeader) {
			// Good
		} else if cookie.Value == apiOp.Request.URL.Query().Get(csrfCookie) {
			// Good
		} else {
			return httperror.NewAPIError(validation.InvalidCSRFToken, "Invalid CSRF token")
		}
	}

	return nil
}
