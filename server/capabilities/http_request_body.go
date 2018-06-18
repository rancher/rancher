package capabilities

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func extractRequestBody(writer http.ResponseWriter, req *http.Request, body interface{}) error {
	raw, err := ioutil.ReadAll(req.Body)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("cannot read request body: " + err.Error())
	}

	err = json.Unmarshal(raw, &body)

	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("cannot parse request body: " + err.Error())
	}

	return nil
}
