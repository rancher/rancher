package googleoauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

// GClient implements a httpclient for google oauth
type GClient struct {
	httpClient *http.Client
}

func (g *GClient) getUser(accessToken string, config *v32.GoogleOauthConfig) (*Account, error) {
	// userinfo endpoint doesn't require viewType param, non-admins and admins both can query this endpoint
	respBytes, _, err := g.getFromGoogle(accessToken, config.UserInfoEndpoint)
	if err != nil {
		return nil, err
	}

	var goauthAccount Account
	if err = json.Unmarshal(respBytes, &goauthAccount); err != nil {
		return nil, err
	}
	return &goauthAccount, nil
}

func (g *GClient) getFromGoogle(accessToken string, url string) ([]byte, int, error) {
	var statusCode int
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, statusCode, err
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}

	b, err := ioutil.ReadAll(resp.Body)
	return b, statusCode, err
}
