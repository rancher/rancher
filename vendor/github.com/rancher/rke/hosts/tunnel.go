package hosts

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	DockerAPIVersion = "1.24"
	K8sVersion       = "1.8"
)

func (h *Host) TunnelUp(ctx context.Context, dialerFactory DialerFactory) error {
	if h.DClient != nil {
		return nil
	}
	log.Infof(ctx, "[dialer] Setup tunnel for host [%s]", h.Address)
	httpClient, err := h.newHTTPClient(dialerFactory)
	if err != nil {
		return fmt.Errorf("Can't establish dialer connection: %v", err)
	}
	// set Docker client
	logrus.Debugf("Connecting to Docker API for host [%s]", h.Address)
	h.DClient, err = client.NewClient("unix:///var/run/docker.sock", DockerAPIVersion, httpClient, nil)
	if err != nil {
		return fmt.Errorf("Can't initiate NewClient: %v", err)
	}
	return checkDockerVersion(ctx, h)
}

func (h *Host) TunnelUpLocal(ctx context.Context) error {
	var err error
	if h.DClient != nil {
		return nil
	}
	// set Docker client
	logrus.Debugf("Connecting to Docker API for host [%s]", h.Address)
	h.DClient, err = client.NewEnvClient()
	if err != nil {
		return fmt.Errorf("Can't initiate NewClient: %v", err)
	}
	return checkDockerVersion(ctx, h)
}

func checkDockerVersion(ctx context.Context, h *Host) error {
	info, err := h.DClient.Info(ctx)
	if err != nil {
		return fmt.Errorf("Can't retrieve Docker Info: %v", err)
	}
	logrus.Debugf("Docker Info found: %#v", info)
	isvalid, err := docker.IsSupportedDockerVersion(info, K8sVersion)
	if err != nil {
		return fmt.Errorf("Error while determining supported Docker version [%s]: %v", info.ServerVersion, err)
	}

	if !isvalid && h.EnforceDockerVersion {
		return fmt.Errorf("Unsupported Docker version found [%s], supported versions are %v", info.ServerVersion, docker.K8sDockerVersions[K8sVersion])
	} else if !isvalid {
		log.Warnf(ctx, "Unsupported Docker version found [%s], supported versions are %v", info.ServerVersion, docker.K8sDockerVersions[K8sVersion])
	}
	return nil
}

func parsePrivateKey(keyBuff string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(keyBuff))
}

func parsePrivateKeyWithPassPhrase(keyBuff string, passphrase []byte) (ssh.Signer, error) {
	return ssh.ParsePrivateKeyWithPassphrase([]byte(keyBuff), passphrase)
}

func makeSSHConfig(user string, signer ssh.Signer) (*ssh.ClientConfig, error) {
	config := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &config, nil
}

func (h *Host) checkEncryptedKey() (ssh.Signer, error) {
	logrus.Debugf("[ssh] Checking private key")
	var err error
	var key ssh.Signer
	if len(h.SSHKey) > 0 {
		key, err = parsePrivateKey(h.SSHKey)
	} else {
		key, err = parsePrivateKey(privateKeyPath(h.SSHKeyPath))
	}
	if err == nil {
		return key, nil
	}

	// parse encrypted key
	if strings.Contains(err.Error(), "decode encrypted private keys") {
		var passphrase []byte
		if len(h.SavedKeyPhrase) == 0 {
			fmt.Printf("Passphrase for Private SSH Key: ")
			passphrase, err = terminal.ReadPassword(int(syscall.Stdin))
			fmt.Printf("\n")
			if err != nil {
				return nil, err
			}
			h.SavedKeyPhrase = string(passphrase)
		} else {
			passphrase = []byte(h.SavedKeyPhrase)
		}

		if len(h.SSHKey) > 0 {
			key, err = parsePrivateKeyWithPassPhrase(h.SSHKey, passphrase)
		} else {
			key, err = parsePrivateKeyWithPassPhrase(privateKeyPath(h.SSHKeyPath), passphrase)
		}
		if err != nil {
			return nil, err
		}
	}
	return key, err
}

func privateKeyPath(sshKeyPath string) string {
	if sshKeyPath[:2] == "~/" {
		sshKeyPath = filepath.Join(os.Getenv("HOME"), sshKeyPath[2:])
	}
	buff, _ := ioutil.ReadFile(sshKeyPath)
	return string(buff)
}
