package error

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	// InvalidRequest the request is missing a required parameter, includes an invalid parameter value, includes a parameter more than once, or is otherwise malformed.
	InvalidRequest = "invalid_request"
	// AccessDenied the resource owner or authorization server denied the request.
	AccessDenied = "access_denied"
	// UnsupportedResponseType the authorization server does not support obtaining an authorization code using this method.
	UnsupportedResponseType = "unsupported_response_type"
	// InvalidScope the requested scope is invalid, unknown, or malformed
	InvalidScope = "invalid_scope"
	// ServerError the authorization server encountered an unexpected condition that prevented it from fulfilling the request.
	ServerError = "server_error"
)

// Error represents an error returned.
type Error struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Write writes the error in the response writer.
func (e *Error) Write(code int, w http.ResponseWriter) {
	WriteError(e.Error, e.ErrorDescription, code, w)
}

// ToString returns a string with the error and description.
func (e *Error) ToString() string {
	return e.Error + ": " + e.ErrorDescription
}

func New(errStr string, errDescription string) *Error {
	return &Error{Error: errStr, ErrorDescription: errDescription}
}

// WriteError writes an error in the response writer.
func WriteError(errStr string, errDescription string, code int, w http.ResponseWriter) {
	oidcErr := Error{
		Error:            errStr,
		ErrorDescription: errDescription,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(oidcErr)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse error response: %v", err), http.StatusInternalServerError)
	}
}

// RedirectWithError redirect to the redirectURI with the error, description and state. This is used in the authorize endpoint as described in the OIDC spec.
func RedirectWithError(redirectURI string, errString string, description string, state string, w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return
	}
	q := url.Values{}
	q.Set("error", errString)
	q.Set("error_description", description)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}
