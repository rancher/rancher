package functions

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetKubernetesVersion(hostURL string, clusterID string, token string) string {
	type clusterSpecs struct {
		GitVersion string `json:"gitVersion"`
	}
	type clusterResponse struct {
		Version clusterSpecs `json:"version"`
	}

	url := fmt.Sprintf("%s/v3/clusters/%s", hostURL, clusterID)

	var version clusterResponse

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

	jsonErr := json.NewDecoder(res.Body).Decode(&version)
	if err != nil {
		fmt.Printf("%v", jsonErr)
	}

	return version.Version.GitVersion
}
