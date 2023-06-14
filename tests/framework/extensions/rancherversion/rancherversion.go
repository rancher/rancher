package rancherversion

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// RequestRancherVersion Requests the rancher version from the rancher server, parses the returned json and returns a
// Config object, or an error.
func RequestRancherVersion(rancherURL string) (*Config, error) {
	var httpURL = "https://" + rancherURL + "/rancherversion"
	req, err := http.Get(httpURL)
	if err != nil {
		return nil, err
	}
	byteObject, err := io.ReadAll(req.Body)
	if err != nil || byteObject == nil {
		return nil, err
	}

	var jsonObject map[string]interface{}
	err = json.Unmarshal(byteObject, &jsonObject)
	if err != nil {
		return nil, err
	}

	configObject := new(Config)
	configObject.IsPrime, _ = strconv.ParseBool(jsonObject["RancherPrime"].(string))
	configObject.RancherVersion = jsonObject["Version"].(string)
	configObject.GitCommit = jsonObject["GitCommit"].(string)

	return configObject, nil
}
