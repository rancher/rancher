package nodes

import (
	"io/ioutil"
	"os/user"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	ExternalNodeConfigConfigurationFileKey = "externalNodes"
	sshPath                                = ".ssh"
)

type Node struct {
	NodeID          string `json:"nodeID" yaml:"nodeID"`
	PublicIPAddress string `json:"publicIPAddress" yaml:"publicIPAddress"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	SSHKeyName      string `json:"sshKeyName" yaml:"sshKeyName"`
	SSHKey          []byte
}

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
