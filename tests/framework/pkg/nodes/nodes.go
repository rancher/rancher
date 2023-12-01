package nodes

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/pkg/sftp"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"golang.org/x/crypto/ssh"
)

const (
	// The json/yaml config key for the config of nodes of outside cloud provider e.g. linode or ec2
	ExternalNodeConfigConfigurationFileKey = "externalNodes"
	SSHPathConfigurationKey                = "sshPath"
	defaultSSHPath                         = ".ssh"
)

// SSHPath is the path to the ssh key used in external node functionality. This be used if the ssh keys exists
// in a location not in /.ssh
type SSHPath struct {
	SSHPath string `json:"sshPath" yaml:"sshPath"`
}

// Node is a configuration of node that is from an outside cloud provider
type Node struct {
	NodeID           string            `json:"nodeID" yaml:"nodeID"`
	NodeLabels       map[string]string `json:"nodeLabels" yaml:"nodeLabels"`
	PublicIPAddress  string            `json:"publicIPAddress" yaml:"publicIPAddress"`
	PrivateIPAddress string            `json:"privateIPAddress" yaml:"privateIPAddress"`
	SSHUser          string            `json:"sshUser" yaml:"sshUser"`
	SSHKeyName       string            `json:"sshKeyName" yaml:"sshKeyName"`
	SSHKey           []byte
}

// ExternalNodeConfig is a struct that is a collection of the node configurations
type ExternalNodeConfig struct {
	Nodes map[int][]*Node `json:"nodes" yaml:"nodes"`
}

// SCPFileToNode copies a file from the local machine to the specific node created.
func (n *Node) SCPFileToNode(localPath, remotePath string) error {
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
	defer client.Close()

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	remoteFile, err := sftp.Create(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	if _, err := remoteFile.ReadFrom(localFile); err != nil {
		return err
	}

	return err
}

// ExecuteCommand executes `command` in the specific node created.
func (n *Node) ExecuteCommand(command string) (string, error) {
	signer, err := ssh.ParsePrivateKey(n.SSHKey)
	var output []byte
	var outputString string

	if err != nil {
		return outputString, err
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
		return outputString, err
	}

	session, err := client.NewSession()
	if err != nil {
		return outputString, err
	}

	output, err = session.Output(command)
	outputString = string(output)
	return outputString, err
}

// GetSSHKey reads in the ssh file from the .ssh directory, returns the key in []byte format
func GetSSHKey(sshKeyname string) ([]byte, error) {
	var keyPath string

	sshPathConfig := new(SSHPath)

	config.LoadConfig(SSHPathConfigurationKey, sshPathConfig)
	if sshPathConfig.SSHPath == "" {
		user, err := user.Current()
		if err != nil {
			return nil, err
		}

		keyPath = filepath.Join(user.HomeDir, defaultSSHPath, sshKeyname)
	} else {
		keyPath = filepath.Join(sshPathConfig.SSHPath, sshKeyname)
	}
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}
