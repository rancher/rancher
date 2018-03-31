package rkenodeconfigclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
)

func ConfigClient(ctx context.Context, url string, header http.Header) error {
	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	for {
		nc, err := getConfig(client, url, header)
		if err != nil {
			logrus.Infof("Error while getting agent config: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if nc != nil {
			return rkeworker.ExecutePlan(ctx, nc)
		}

		logrus.Infof("waiting for node to register")
		time.Sleep(2 * time.Second)
	}
}

func getConfig(client *http.Client, url string, header http.Header) (*rkeworker.NodeConfig, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		req.Header[k] = v
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return &rkeworker.NodeConfig{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		content, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response %d: %s", resp.StatusCode, string(content))
	}

	nc := &rkeworker.NodeConfig{}
	return nc, json.NewDecoder(resp.Body).Decode(nc)
}
