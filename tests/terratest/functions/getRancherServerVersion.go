package functions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func GetRancherServerVersion(hostURL string, token string) string {
	type nestedData struct {
		Version string `json:"seen-whatsnew"`
	}

	type clusterSpecs struct {
		Data nestedData `json:"data"`
	}

	type clusterResponse struct {
		Data []clusterSpecs `json:"data"`
	}

	url := fmt.Sprintf("%s/v1/userpreferences", hostURL)

	var response clusterResponse

	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("%v", err)
	}

	req.Header = http.Header{
		"Authorization": []string{token},
	}

	res, clientErr := client.Do(req)
	if clientErr != nil {
		fmt.Printf("%v", clientErr)
	}

	defer res.Body.Close()

	jsonErr := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		fmt.Printf("%v", jsonErr)
	}

	version := strings.Trim(response.Data[0].Data.Version, `"`)

	return version
}
