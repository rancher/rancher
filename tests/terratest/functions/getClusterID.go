package functions

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetClusterID(hostURL string, clusterName string, token string) string {
	type clusterSpecs struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	type clusterResponse struct {
		Clusters []clusterSpecs `json:"data"`
	}

	url := fmt.Sprintf("%s/v3/clusters", hostURL)

	var clusters clusterResponse

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

	jsonErr := json.NewDecoder(res.Body).Decode(&clusters)
	if err != nil {
		fmt.Printf("%v", jsonErr)
	}

	var clusterSpec string

	for _, cluster := range clusters.Clusters {
		if cluster.Name == clusterName {
			clusterSpec = cluster.Id
		}
	}

	return clusterSpec
}
