package parse

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"
)

func Icon(url string) (string, string, error) {
	if url == "" {
		return "", "", nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(url, "/")
	iconFilename := parts[len(parts)-1]
	iconData := base64.StdEncoding.EncodeToString(body)

	return iconData, iconFilename, nil
}
