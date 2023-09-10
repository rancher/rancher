package rancherversion

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

/* Requests the rancher version from the rancher server, parses the returned
 * json and returns a Struct object, or an error.
 */
func RequestRancherVersion(rancher_url string) (*Config, error) {
	var http_url = "https://" + rancher_url + "/rancherversion"
	req, err := http.Get(http_url)
	if err != nil {
		return nil, err
	}
	byte_object, err := io.ReadAll(req.Body)
	if err != nil || byte_object == nil {
		return nil, err
	}
	var jsonObject map[string]interface{}
	err = json.Unmarshal(byte_object, &jsonObject)
	if err != nil {
		return nil, err
	}
	config_object := new(Config)
	config_object.IsPrime, _ = strconv.ParseBool(jsonObject["RancherPrime"].(string))
	config_object.RancherVersion = jsonObject["Version"].(string)
	config_object.GitCommit = jsonObject["GitCommit"].(string)
	return config_object, nil
}
