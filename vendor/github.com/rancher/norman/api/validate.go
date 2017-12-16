package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
)

const (
	csrfCookie = "CSRF"
	csrfHeader = "X-API-CSRF"
)

func ValidateAction(request *types.APIContext) (*types.Action, error) {
	if request.Action == "" || request.Method != http.MethodPost {
		return nil, nil
	}

	actions := request.Schema.CollectionActions
	if request.ID != "" {
		actions = request.Schema.ResourceActions
	}

	action, ok := actions[request.Action]
	if !ok {
		return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Invalid action: %s", request.Action))
	}

	if request.ID != "" && request.ReferenceValidator != nil {
		resource := request.ReferenceValidator.Lookup(request.Type, request.ID)
		if resource == nil {
			return nil, httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("Failed to find type: %s id: %s", request.Type, request.ID))
		}

		if _, ok := resource.Actions[request.Action]; !ok {
			return nil, httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Invalid action: %s", request.Action))
		}
	}

	return &action, nil
}

func CheckCSRF(rw http.ResponseWriter, req *http.Request) error {
	if !parse.IsBrowser(req, false) {
		return nil
	}

	cookie, err := req.Cookie(csrfCookie)
	if err == http.ErrNoCookie {
		bytes := make([]byte, 5)
		_, err := rand.Read(bytes)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.ServerError, "Failed in CSRF processing")
		}

		cookie = &http.Cookie{
			Name:  csrfCookie,
			Value: hex.EncodeToString(bytes),
		}
	} else if err != nil {
		return httperror.NewAPIError(httperror.InvalidCSRFToken, "Failed to parse cookies")
	} else if req.Method != http.MethodGet {
		/*
		 * Very important to use request.getMethod() and not httpRequest.getMethod(). The client can override the HTTP method with _method
		 */
		if cookie.Value == req.Header.Get(csrfHeader) {
			// Good
		} else if cookie.Value == req.URL.Query().Get(csrfCookie) {
			// Good
		} else {
			return httperror.NewAPIError(httperror.InvalidCSRFToken, "Invalid CSRF token")
		}
	}

	cookie.Path = "/"
	http.SetCookie(rw, cookie)
	return nil
}
