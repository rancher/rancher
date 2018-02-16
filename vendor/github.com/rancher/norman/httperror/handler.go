package httperror

import (
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

func ErrorHandler(request *types.APIContext, err error) {
	var error *APIError
	if apiError, ok := err.(*APIError); ok {
		if apiError.Cause != nil {
			logrus.Errorf("API error response %v for %v %v. Cause: %v", apiError.code.Status, request.Request.Method,
				request.Request.RequestURI, apiError.Cause)
		}
		error = apiError
	} else {
		logrus.Errorf("Unknown error: %v", err)
		error = &APIError{
			code:    ServerError,
			message: err.Error(),
		}
	}

	data := toError(error)
	request.WriteResponse(error.code.Status, data)
}

func toError(apiError *APIError) map[string]interface{} {
	e := map[string]interface{}{
		"type":    "/meta/schemas/error",
		"status":  apiError.code.Status,
		"code":    apiError.code.code,
		"message": apiError.message,
	}
	if apiError.fieldName != "" {
		e["fieldName"] = apiError.fieldName
	}

	return e
}
