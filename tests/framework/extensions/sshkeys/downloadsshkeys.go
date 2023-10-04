package sshkeys

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
)

const (
	privateKeySSHKeyRegExPattern              = `-----BEGIN RSA PRIVATE KEY-{3,}\n([\s\S]*?)\n-{3,}END RSA PRIVATE KEY-----`
	ClusterMachineConstraintResourceSteveType = "cluster.x-k8s.io.machine"
)

// DownloadSSHKeys is a helper function that takes a client, the machinePoolNodeName to download
// the ssh key for a particular node.
func DownloadSSHKeys(client *rancher.Client, machinePoolNodeName string) ([]byte, error) {
	machinePoolNodeNameName := fmt.Sprintf("fleet-default/%s", machinePoolNodeName)
	machine, err := client.Steve.SteveType(ClusterMachineConstraintResourceSteveType).ByID(machinePoolNodeNameName)
	if err != nil {
		return nil, err
	}

	sshKeyLink := machine.Links["sshkeys"]

	req, err := http.NewRequest("GET", sshKeyLink, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+client.RancherConfig.AdminToken)

	resp, err := client.Management.APIBaseClient.Ops.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	privateSSHKeyRegEx := regexp.MustCompile(privateKeySSHKeyRegExPattern)
	privateSSHKey := privateSSHKeyRegEx.FindString(string(bodyBytes))

	return []byte(privateSSHKey), err
}
