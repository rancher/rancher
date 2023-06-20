package token

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rancher/norman/httperror"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// GenerateUserToken is a helper function that generates a bearer token for a specified user using the
// username and password
func GenerateUserToken(user *management.User, url string) (*management.Token, error) {
	token := &management.Token{}

	bodyContent, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Username: user.Username,
		Password: user.Password,
	})

	if err != nil {
		return nil, err
	}

	err = postAction("/v3-public/localProviders/local", "login", url, bodyContent, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func postAction(endpoint, action, host string, body []byte, output interface{}) error {
	url := "https://" + host + endpoint + "?action=" + action
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return httperror.NewAPIErrorLong(resp.StatusCode, resp.Status, url)
	}

	byteContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if len(byteContent) > 0 {
		err = json.Unmarshal(byteContent, output)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("received empty response")
}
