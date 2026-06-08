package clusterregistrationtoken

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr/installer"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
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

	tokenPlaceholder = "{token}"
)

func (h *handler) assignStatus(crt *v32.ClusterRegistrationToken) (v32.ClusterRegistrationTokenStatus, error) {
	if crt.Status.TokenSecretName == "" {
		return crt.Status, nil
	}

	clusterID := convert.ToString(crt.Spec.ClusterName)
	crtStatus := crt.Status.DeepCopy()

	return AssignCommands(crtStatus, clusterID, h.clusters)
}

// AssignCommands populates the command fields in a CRT status using the "{token}" placeholder.
// Substitution of the placeholder with the real token happens at the API layer.
func AssignCommands(crtStatus *v32.ClusterRegistrationTokenStatus, clusterID string, clusters mgmtcontrollers.ClusterCache) (v32.ClusterRegistrationTokenStatus, error) {
	checksum := systemtemplate.CAChecksum()
	ca := ""
	caWindows := ""
	if checksum != "" {
		ca = " --ca-checksum " + checksum
		caWindows = " -CaChecksum " + checksum
	}

	url, err := getURL(tokenPlaceholder, clusterID)
	if err != nil {
		return *crtStatus, err
	}

	if url == "" {
		return *crtStatus, nil
	}

	crtStatus.InsecureCommand = fmt.Sprintf(insecureCommandFormat, url)
	crtStatus.Command = fmt.Sprintf(commandFormat, url)
	crtStatus.ManifestURL = url

	rootURL, err := getRootURL()
	if err != nil {
		return *crtStatus, err
	}

	cluster, err := clusters.Get(clusterID)
	if err != nil {
		return *crtStatus, err
	}

	// for linux
	crtStatus.NodeCommand = fmt.Sprintf(provisioningV2NodeCommandFormat,
		AgentEnvVars(cluster, Linux),
		rootURL+installer.SystemAgentInstallPath,
		AgentEnvVars(cluster, Linux),
		rootURL,
		tokenPlaceholder,
		ca)
	crtStatus.InsecureNodeCommand = fmt.Sprintf(provisioningV2InsecureNodeCommandFormat,
		AgentEnvVars(cluster, Linux),
		rootURL+installer.SystemAgentInstallPath,
		AgentEnvVars(cluster, Linux),
		rootURL,
		tokenPlaceholder,
		ca)

	// for windows
	crtStatus.WindowsNodeCommand = fmt.Sprintf(provisioningV2WindowsNodeCommandFormat,
		AgentEnvVars(cluster, PowerShell),
		rootURL+installer.WindowsRke2InstallPath,
		rootURL,
		tokenPlaceholder,
		caWindows)
	crtStatus.InsecureWindowsNodeCommand = fmt.Sprintf(provisioningV2InsecureWindowsNodeCommandFormat,
		AgentEnvVars(cluster, PowerShell),
		rootURL+installer.WindowsRke2InstallPath,
		rootURL,
		tokenPlaceholder,
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
	result := u.String()

	// Unescape the {token} placeholder that was auto-encoded by url.String()
	result = strings.ReplaceAll(result, "%7Btoken%7D", "{token}")
	return result, nil
}
