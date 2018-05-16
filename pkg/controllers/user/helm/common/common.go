package common

import (
	"net/url"
	"strings"
)

func ParseExternalID(externalID string) (string, error) {
	values, err := url.Parse(externalID)
	if err != nil {
		return "", err
	}
	catalog := values.Query().Get("catalog")
	template := values.Query().Get("template")
	version := values.Query().Get("version")
	return strings.Join([]string{catalog, template, version}, "-"), nil
}
