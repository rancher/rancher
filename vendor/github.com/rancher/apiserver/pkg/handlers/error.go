package handlers

import (
	"net/http"
	"net/url"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

func ErrorHandler(request *types.APIRequest, err error) {
	if err == validation.ErrComplete {
		return
	}

	if ec, ok := err.(validation.ErrorCode); ok {
		err = apierror.NewAPIError(ec, "")
	}

	var error *apierror.APIError
	if apiError, ok := err.(*apierror.APIError); ok {
		if apiError.Cause != nil {
			url, _ := url.PathUnescape(request.Request.URL.String())
			if url == "" {
				url = request.Request.URL.String()
			}
			logrus.Errorf("API error response %v for %v %v. Cause: %v", apiError.Code.Status, request.Request.Method,
				url, apiError.Cause)
		}
		error = apiError
	} else {
		logrus.Errorf("Unknown error: %v", err)
		error = &apierror.APIError{
			Code:    validation.ServerError,
			Message: err.Error(),
		}
	}

	if error.Code.Status == http.StatusNoContent {
		request.Response.WriteHeader(http.StatusNoContent)
		return
	}

	data := toError(error)
	request.WriteResponse(error.Code.Status, data)
}

func toError(apiError *apierror.APIError) types.APIObject {
	e := map[string]interface{}{
		"type":    "error",
		"status":  apiError.Code.Status,
		"code":    apiError.Code.Code,
		"message": apiError.Message,
	}
	if apiError.FieldName != "" {
		e["fieldName"] = apiError.FieldName
	}

	return types.APIObject{
		Type:   "error",
		Object: e,
	}
}
