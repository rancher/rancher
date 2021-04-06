package capabilities

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

type errorResponse struct {
	Error string `json:"error"`
}

func handleErr(writer http.ResponseWriter, originalErr error) {
	resp := errorResponse{originalErr.Error()}

	asJSON, err := json.Marshal(resp)

	if err != nil {
		logrus.Error("error while marshalling error message '" + originalErr.Error() + "' error was '" + err.Error() + "'")
		writer.Write([]byte(err.Error()))

		return
	}

	writer.Write(asJSON)
}

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
