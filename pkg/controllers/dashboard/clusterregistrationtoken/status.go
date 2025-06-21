package clusterregistrationtoken

import (
	"fmt"
	"net/url"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr/installer"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemtemplate"
)

const (
	commandFormat                                  = "kubectl apply -f %s"
	insecureCommandFormat                          = "curl --insecure -sfL %s | kubectl apply -f -"
	provisioningV2NodeCommandFormat                = "%s curl -fL %s | sudo %s sh -s - --server %s --label 'cattle.io/os=linux' --token %s%s"
	provisioningV2WindowsNodeCommandFormat         = `%s curl.exe -fL %s -o install.ps1; Set-ExecutionPolicy Bypass -Scope Process -Force; ./install.ps1 -Server %s -Label 'cattle.io/os=windows' -Token %s -Worker%s`
	provisioningV2InsecureNodeCommandFormat        = "%s curl --insecure -fL %s | sudo %s sh -s - --server %s --label 'cattle.io/os=linux' --token %s%s"
	provisioningV2InsecureWindowsNodeCommandFormat = `%s curl.exe --insecure -fL %s -o install.ps1; Set-ExecutionPolicy Bypass -Scope Process -Force; ./install.ps1 -Server %s -Label 'cattle.io/os=windows' -Token %s -Worker%s`
)

func (h *handler) assignStatus(crt *v32.ClusterRegistrationToken) (v32.ClusterRegistrationTokenStatus, error) {
	checksum := systemtemplate.CAChecksum()
	ca := ""
	caWindows := ""
	if checksum != "" {
		ca = " --ca-checksum " + checksum
		caWindows = " -CaChecksum " + checksum
	}

	token := crt.Status.Token
	clusterID := convert.ToString(crt.Spec.ClusterName)
	if token == "" {
		return crt.Status, nil
	}

	crtStatus := crt.Status.DeepCopy()
	crtStatus.Token = token

	url, err := getURL(token, clusterID)
	if err != nil {
		return crt.Status, err
	}

	if url == "" {
		return *crtStatus, nil
	}

	crtStatus.InsecureCommand = fmt.Sprintf(insecureCommandFormat, url)
	crtStatus.Command = fmt.Sprintf(commandFormat, url)
	crtStatus.ManifestURL = url

	rootURL, err := getRootURL()
	if err != nil {
		return crt.Status, err
	}

	cluster, err := h.clusters.Get(clusterID)
	if err != nil {
		return crt.Status, err
	}

	// for linux
	crtStatus.NodeCommand = fmt.Sprintf(provisioningV2NodeCommandFormat,
		AgentEnvVars(cluster, Linux),
		rootURL+installer.SystemAgentInstallPath,
		AgentEnvVars(cluster, Linux),
		rootURL,
		token,
		ca)
	crtStatus.InsecureNodeCommand = fmt.Sprintf(provisioningV2InsecureNodeCommandFormat,
		AgentEnvVars(cluster, Linux),
		rootURL+installer.SystemAgentInstallPath,
		AgentEnvVars(cluster, Linux),
		rootURL,
		token,
		ca)

	// for windows
	crtStatus.WindowsNodeCommand = fmt.Sprintf(provisioningV2WindowsNodeCommandFormat,
		AgentEnvVars(cluster, PowerShell),
		rootURL+installer.WindowsRke2InstallPath,
		rootURL,
		token,
		caWindows)
	crtStatus.InsecureWindowsNodeCommand = fmt.Sprintf(provisioningV2InsecureWindowsNodeCommandFormat,
		AgentEnvVars(cluster, PowerShell),
		rootURL+installer.WindowsRke2InstallPath,
		rootURL,
		token,
		caWindows)

	return *crtStatus, nil
}

func ShareMntCommand(nodeName, token string, cluster *v3.Cluster) ([]string, error) {
	rootURL, err := getRootURL()
	if err != nil {
		return []string{""}, err
	}

	cmd := []string{
		"--no-register", "--only-write-certs",
		"--node-name", nodeName,
		"--server", rootURL,
		"--token", token,
	}

	ca := systemtemplate.CAChecksum()
	if ca != "" {
		cmd = append(cmd, fmt.Sprintf("--ca-checksum %s", ca))
	}

	return cmd, nil
}

func getRootURL() (string, error) {
	serverURL := settings.ServerURL.Get()
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	u.Path = ""
	return u.String(), nil
}

func getURL(token, clusterID string) (string, error) {
	serverURL := settings.ServerURL.Get()
	if serverURL == "" {
		return "", nil
	}
	path := "/v3/import/" + token + "_" + clusterID + ".yaml"
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	u.Path = path
	serverURL = u.String()
	return serverURL, nil
}
