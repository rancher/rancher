package bootstrap

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

var (
	defaultSystemAgentInstallScript = "https://raw.githubusercontent.com/rancher/system-agent/main/install.sh"
	localAgentInstallScript         = "./install.sh"
)

func InstallScript() ([]byte, error) {
	url := settings.SystemAgentInstallScript.Get()
	if url == "" {
		script, err := ioutil.ReadFile(localAgentInstallScript)
		if !os.IsNotExist(err) {
			return script, err
		}
	}

	if url == "" {
		url = defaultSystemAgentInstallScript
	}

	resp, httpErr := http.Get(url)
	if httpErr != nil {
		return nil, httpErr
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func Bootstrap(token string) ([]byte, error) {
	script, err := InstallScript()
	if err != nil {
		return nil, err
	}

	url, ca := settings.ServerURL.Get(), systemtemplate.CAChecksum()
	return []byte(fmt.Sprintf(`#!/usr/bin/env sh
CATTLE_SERVER="%s"
CATTLE_CA_CHECKSUM="%s"
CATTLE_TOKEN="%s"

%s
`, url, ca, token, script)), nil
}
