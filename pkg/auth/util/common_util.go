package util

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

// WriteError write the error message and the http status code in the ResponseWriter
func WriteError(w http.ResponseWriter, httpStatus int, err error) {
	w.WriteHeader(httpStatus)
	w.Write([]byte(err.Error()))
}

// ReturnHTTPError handles sending out Error response
// TODO Use the Norman API error framework instead
func ReturnHTTPError(w http.ResponseWriter, r *http.Request, httpStatus int, errorMessage string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	err := AuthError{
		Status:  strconv.Itoa(httpStatus),
		Message: errorMessage,
		Type:    "error",
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(err)
}

func GetHTTPErrorCode(httpStatus int) string {
	switch httpStatus {
	case 401:
		return "Unauthorized"
	case 404:
		return "NotFound"
	case 403:
		return "PermissionDenied"
	case 500:
		return "ServerError"
	}

	return "ServerError"
}

func GetHost(req *http.Request) string {
	host := req.Header.Get("X-API-Host")
	if host == "" {
		host = req.Header.Get("X-Forwarded-Host")
	}
	if host == "" {
		host = req.Host
	}
	return host
}

// AuthError structure contains the error resource definition
type AuthError struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// APIErrorResponse defines the HTTP response for an API error.
type APIErrorResponse struct {
	Code     string `json:"code"`
	Status   int    `json:"status"`
	Message  string `json:"message,omitempty"`
	Type     string `json:"type"`
	BaseType string `json:"baseType"`
}

// ReturnAPIError writes a structured error response for the given error.
func ReturnAPIError(w http.ResponseWriter, err error) {
	var (
		status  int
		code    string
		message string
	)

	switch e := err.(type) {
	case *apierror.APIError:
		status = e.Code.Status
		code = e.Code.Code
		message = e.Message
	case *httperror.APIError: // Compatibility with norman.
		status = e.Code.Status
		code = e.Code.Code
		message = e.Message
	default: // Not an APIError.
		status = validation.ServerError.Status
		code = validation.ServerError.Code
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := APIErrorResponse{
		Code:     code,
		Status:   status,
		Type:     "error",
		BaseType: "error",
	}
	if message != "" {
		resp.Message = message
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if eerr := enc.Encode(resp); eerr != nil {
		logrus.Errorf("Writing error response: %s", eerr)
	}
}

// IsAPIError returns true if the err is either [apierror.APIError]
// or [httperror.APIError] for compatibility with norman.
func IsAPIError(err error) bool {
	switch err.(type) {
	case *apierror.APIError, *httperror.APIError:
		return true
	default:
		return false
	}
}
