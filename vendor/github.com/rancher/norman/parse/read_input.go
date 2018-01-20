package parse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rancher/norman/httperror"
)

const reqMaxSize = (2 * 1 << 20) + 1

var bodyMethods = map[string]bool{
	http.MethodPut:  true,
	http.MethodPost: true,
}

func ReadBody(req *http.Request) (map[string]interface{}, error) {
	if !bodyMethods[req.Method] {
		return nil, nil
	}

	dec := json.NewDecoder(io.LimitReader(req.Body, reqMaxSize))
	dec.UseNumber()

	data := map[string]interface{}{}
	if err := dec.Decode(&data); err != nil {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	return data, nil
}
