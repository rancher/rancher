package hosts

import (
	"fmt"
	"net"
	"net/http"

	"golang.org/x/crypto/ssh"
)

type Dialer interface {
	NewHTTPClient() (*http.Client, error)
}

type sshDialer struct {
	host   *Host
	signer ssh.Signer
}

func (d *sshDialer) NewHTTPClient() (*http.Client, error) {
	dialer := &sshDialer{
		host:   d.host,
		signer: d.signer,
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
	}
	return httpClient, nil
}

func (d *sshDialer) Dial(network, addr string) (net.Conn, error) {
	sshAddr := d.host.Address + ":22"
	// Build SSH client configuration
	cfg, err := makeSSHConfig(d.host.User, d.signer)
	if err != nil {
		return nil, fmt.Errorf("Error configuring SSH: %v", err)
	}
	// Establish connection with SSH server
	conn, err := ssh.Dial("tcp", sshAddr, cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial ssh using address [%s]: %v", sshAddr, err)
	}
	if len(d.host.DockerSocket) == 0 {
		d.host.DockerSocket = "/var/run/docker.sock"
	}
	remote, err := conn.Dial("unix", d.host.DockerSocket)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial to Docker socket: %v", err)
	}
	return remote, err
}
