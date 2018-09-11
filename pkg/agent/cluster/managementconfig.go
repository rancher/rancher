package cluster

import (
	"net/http"
	"time"

	"encoding/json"

	"crypto/tls"

	"github.com/pkg/errors"
	apptypes "github.com/rancher/rancher/app/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/client-go/rest"
)

const (
	Token = "X-API-Tunnel-Token"
)

var (
	client = &http.Client{
		Timeout: 300 * time.Second,
	}
)

type ManagementConfig struct {
	CfgConfig   *apptypes.Config `json:"cfgConfig"`
	RestConfig  RestConfig       `json:"restConfig"`
	Cluster     *v3.Cluster      `json:"cluster"`
	BearerToken string           `json:"bearerToken"`
}

type RestConfig struct {
	Host            string               `json:"host"`
	BearerToken     string               `json:"bearerToken"`
	TLSClientConfig rest.TLSClientConfig `json:"tlsClientConfig"`
}

func GetManagementConfig(url string, token string) (*ManagementConfig, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	header := map[string][]string{
		Token: {token},
	}

	for k, v := range header {
		req.Header[k] = v
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   300 * time.Second,
		Transport: tr,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("token unauthorized to get management config")
	}
	mc := &ManagementConfig{}
	return mc, json.NewDecoder(resp.Body).Decode(mc)
}
