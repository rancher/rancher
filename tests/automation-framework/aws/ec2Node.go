package aws

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

var SSHPath = os.Getenv("SSH_PATH")

type EC2Node struct {
	NodeName         string
	NodeID           string
	PublicIPAdress   string
	PrivateIPAddress string
	SSHUser          string
	SSHName          string
	SSHKey           []byte
	SSHKeyPath       string
}

func (e *EC2Node) ExecuteCommand(command string) error {
	signer, err := ssh.ParsePrivateKey(e.SSHKey)
	if err != nil {
		return err
	}

	auths := []ssh.AuthMethod{ssh.PublicKeys([]ssh.Signer{signer}...)}

	cfg := &ssh.ClientConfig{
		User:            e.SSHUser,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	cfg.SetDefaults()

	client, err := ssh.Dial("tcp", e.PublicIPAdress+":22", cfg)
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	return session.Run(command)
}

func getSSHKeyName(sshKeyName string) string {
	stringSlice := strings.Split(sshKeyName, ".")
	return stringSlice[0]
}

func getSSHKey(sshKeyName string) ([]byte, error) {
	content, err := ioutil.ReadFile(getSSHKeyPath(sshKeyName))
	if err != nil {
		return []byte{}, err
	}

	// contentString := string(content)
	return content, nil
}

func getSSHKeyPath(sshKeyname string) string {
	keyPath := filepath.Join(SSHPath, sshKeyname)
	return keyPath
}
