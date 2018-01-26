package util

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rancher/rancher/pkg/auth/model"
)

//ReturnHTTPError handles sending out Error response
func ReturnHTTPError(w http.ResponseWriter, r *http.Request, httpStatus int, errorMessage string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	err := model.AuthError{
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
