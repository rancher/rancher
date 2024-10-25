package airgap

import (
	"crypto/x509"
	"errors"
	"os"
	"strings"

	"encoding/pem"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	corralBastionIP  = "bastion_ip"
	corralSSHUser    = "aws_ssh_user"
	corralPrivateKey = "corral_private_key"
	systemRegistry   = "system-default-registry"

	fleetExampleFolderName = "fleet-examples.git"
	rancherGithubURL       = "https://github.com/rancher/"
	fleetDefaultNS         = "fleet-default"

	sshPort = ":22"
	rsa     = "RSA"
	tcp     = "tcp"
)

func setupAirgapFleetResources(bastionUser, bastionIP, privateKey string) (string, error) {
	var key ssh.Signer
	var err error

	if strings.Contains(privateKey, rsa) {

		keyBlock, _ := pem.Decode([]byte(privateKey))
		if keyBlock == nil {
			return "", errors.New("unable to use privateKey")
		}

		rsaKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return "", err
		}

		key, err = ssh.NewSignerFromKey(rsaKey)
	} else {
		key, err = ssh.ParsePrivateKey([]byte(privateKey))
	}
	if err != nil {
		return "", err
	}

	return updateLocalFleetRepo(bastionUser, bastionIP, key)

}

// updateLocalFleetRepo clones fleet-examples repo onto the bastion node via ssh
func updateLocalFleetRepo(bastionUser, bastionIP string, privateKey ssh.Signer) (string, error) {
	config := &ssh.ClientConfig{
		User: bastionUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial(tcp, bastionIP+sshPort, config)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	gitSession, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer gitSession.Close()

	var b strings.Builder
	gitSession.Stdout = &b
	gitSession.Stderr = &b

	// install git if it isn't on the bastion already
	err = gitSession.Run("git version")
	if err != nil {
		if strings.Contains(b.String(), "no package named") {
			installGitSession, err := conn.NewSession()
			if err != nil {
				return "", err
			}
			defer installGitSession.Close()

			osOutput, err := installGitSession.CombinedOutput("hostnamectl")
			if err != nil {
				return "", err
			}

			osString := string(osOutput)

			if strings.Contains(osString, "Ubuntu") {
				err = executeCommand(conn, "sudo apt-get update && sudo apt-get install -y git")
			} else if strings.Contains(osString, "SUSE") {
				err = executeCommand(conn, "sudo zypper refresh && sudo zypper install git -y")
			}
			if err != nil {
				return "", err
			}

		} else {
			return "", err
		}
	}

	err = executeCommand(conn, "cd ~/ && git clone --mirror "+rancherGithubURL+fleetExampleFolderName)
	if err != nil {
		return "", err
	}

	err = executeCommand(conn, "sudo git config --system --add safe.directory '*'")
	if err != nil {
		return "", err
	}

	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	internalByteIP, err := session.CombinedOutput("hostname -I | awk '{print $1}'")

	// output ends in a newline, which we don't need
	return string(internalByteIP)[:len(internalByteIP)-1], err
}

// executeCommand executes a command via ssh. Each executed command needs its own session. See ssh package for more details.
func executeCommand(client *ssh.Client, command string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(command)
}

// createSSHSecret is a helper to create a secret using wrangler client for ssh in fleet-default
func createSSHSecret(client *rancher.Client, data map[string][]byte) (*corev1.Secret, error) {
	var err error

	ctx := client.WranglerContext

	secretName := namegenerator.AppendRandomString("fleet-ssh")
	secretTemplate := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: fleetDefaultNS,
		},
		Data: data,
		Type: corev1.SecretTypeSSHAuth,
	}

	createdSecret, err := ctx.Core.Secret().Create(&secretTemplate)

	return createdSecret, err
}

// createFleetSSHSecret creates an sshSecret in fleet-default namespace for use with fleet resources
func createFleetSSHSecret(client *rancher.Client, privateKey string) (string, error) {
	key, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", err
	}

	keyData := map[string][]byte{
		"ssh-publickey":  []byte(key.PublicKey().Marshal()),
		"ssh-privatekey": []byte(privateKey),
	}

	secretResp, err := createSSHSecret(client, keyData)
	if err != nil {
		return "", err
	}

	return secretResp.Name, nil
}
