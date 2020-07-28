package handler

import (
	"errors"
	"net/url"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

func ErrorHandler(request *types.APIContext, err error) {
	error := &httperror.APIError{}
	if errors.As(err, &error) {
		if error.Cause != nil {
			url, _ := url.PathUnescape(request.Request.URL.String())
			if url == "" {
				url = request.Request.URL.String()
			}
			logrus.Errorf("API error response %v for %v %v. Cause: %v", error.Code.Status, request.Request.Method,
				url, error.Cause)
		}
	} else {
		logrus.Errorf("Unknown error: %v", err)
		error = &httperror.APIError{
			Code:    httperror.ServerError,
			Message: err.Error(),
		}
	}

	data := toError(error)
	request.WriteResponse(error.Code.Status, data)
}

func toError(apiError *httperror.APIError) map[string]interface{} {
	e := map[string]interface{}{
		"type":    "/meta/schemas/error",
		"status":  apiError.Code.Status,
		"code":    apiError.Code.Code,
		"message": apiError.Message,
	}
	if apiError.FieldName != "" {
		e["fieldName"] = apiError.FieldName
	}

	return e
}
