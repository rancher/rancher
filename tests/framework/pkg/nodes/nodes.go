package nodes

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"golang.org/x/crypto/ssh"
)

const (
	SSHConfigConfigurationFileKey = "sshConfig"
)

type Node struct {
	NodeID          string `json:"nodeID" yaml:"nodeID"`
	PublicIPAddress string `json:"publicIPAddress" yaml:"publicIPAddress"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	SSHName         string `json:"sshName" yaml:"sshName"`
	SSHKey          []byte
}

type ExternalNodeConfig struct {
	PathToSSHKEY string  `json:"pathToSSHKEY" yaml:"pathToSSHKEY"`
	Nodes        []*Node `json:"nodes" yaml:"nodes"`
}

func (n *Node) ExecuteCommand(command string) error {
	signer, err := ssh.ParsePrivateKey(n.SSHKey)
	if err != nil {
		return err
	}

	auths := []ssh.AuthMethod{ssh.PublicKeys([]ssh.Signer{signer}...)}

	cfg := &ssh.ClientConfig{
		User:            n.SSHUser,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	cfg.SetDefaults()

	client, err := ssh.Dial("tcp", n.PublicIPAddress+":22", cfg)
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	return session.Run(command)
}

func GetSSHKeyName(sshKeyName string) string {
	stringSlice := strings.Split(sshKeyName, ".")
	return stringSlice[0]
}

func GetSSHKey(sshKeyname string) ([]byte, error) {
	var nodeConfig ExternalNodeConfig
	config.LoadConfig(SSHConfigConfigurationFileKey, &nodeConfig)

	keyPath := filepath.Join(nodeConfig.PathToSSHKEY, sshKeyname)
	content, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}
