package nodes

import (
	"io/ioutil"
	"os/user"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	// The json/yaml config key for the config of nodes of outside cloud provider e.g. linode or ec2
	ExternalNodeConfigConfigurationFileKey = "externalNodes"
	sshPath                                = ".ssh"
)

// Node is a configuration of node that is from an oudise cloud provider
type Node struct {
	NodeID          string `json:"nodeID" yaml:"nodeID"`
	PublicIPAddress string `json:"publicIPAddress" yaml:"publicIPAddress"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	SSHKeyName      string `json:"sshKeyName" yaml:"sshKeyName"`
	SSHKey          []byte
}

// ExternalNodeConfig is a struct that is a collection of the node configurations
type ExternalNodeConfig struct {
	Nodes map[int][]*Node `json:"nodes" yaml:"nodes"`
}

// ExecuteCommand executes `command` in the specific node created.
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

// GetSSHKey reads in the ssh file from the .ssh directory, returns the key in []byte format
func GetSSHKey(sshKeyname string) ([]byte, error) {
	user, err := user.Current()
	if err != nil {
		return nil, err
	}

	keyPath := filepath.Join(user.HomeDir, sshPath, sshKeyname)
	content, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}
