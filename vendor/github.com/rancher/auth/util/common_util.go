package util

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rancher/auth/model"
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
