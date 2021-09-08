package util

import (
	"encoding/json"
	"net/http"
	"strconv"
)

var (
	RequestKey = struct{}{}
)

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
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusNotFound:
		return "NotFound"
	case http.StatusForbidden:
		return "PermissionDenied"
	default:
		return "ServerError"
	}
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

// AuthError contains the error resource definition
type AuthError struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
