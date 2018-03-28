package hosts

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	DockerDialerTimeout = 30
)

type DialerFactory func(h *Host) (func(network, address string) (net.Conn, error), error)

type dialer struct {
	signer          ssh.Signer
	sshKeyString    string
	sshAddress      string
	sshPassphrase   []byte
	username        string
	netConn         string
	dockerSocket    string
	useSSHAgentAuth bool
}

func newDialer(h *Host, kind string) (*dialer, error) {
	dialer := &dialer{
		sshAddress:      fmt.Sprintf("%s:%s", h.Address, h.Port),
		username:        h.User,
		dockerSocket:    h.DockerSocket,
		sshKeyString:    h.SSHKey,
		netConn:         "unix",
		sshPassphrase:   []byte(h.SavedKeyPhrase),
		useSSHAgentAuth: h.SSHAgentAuth,
	}

	if dialer.sshKeyString == "" {
		dialer.sshKeyString = privateKeyPath(h.SSHKeyPath)
	}

	switch kind {
	case "network", "health":
		dialer.netConn = "tcp"
	}

	if len(dialer.dockerSocket) == 0 {
		dialer.dockerSocket = "/var/run/docker.sock"
	}

	return dialer, nil
}

func SSHFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	dialer, err := newDialer(h, "docker")
	return dialer.Dial, err
}

func LocalConnFactory(h *Host) (func(network, address string) (net.Conn, error), error) {
	dialer, err := newDialer(h, "network")
	return dialer.Dial, err
}

func (d *dialer) DialDocker(network, addr string) (net.Conn, error) {
	return d.Dial(network, addr)
}

func (d *dialer) DialLocalConn(network, addr string) (net.Conn, error) {
	return d.Dial(network, addr)
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	conn, err := d.getSSHTunnelConnection()
	if err != nil {
		return nil, fmt.Errorf("Failed to dial ssh using address [%s]: %v", d.sshAddress, err)
	}

	// Docker Socket....
	if d.netConn == "unix" {
		addr = d.dockerSocket
		network = d.netConn
	}

	remote, err := conn.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial to %s: %v", addr, err)
	}
	return remote, err
}

func (d *dialer) getSSHTunnelConnection() (*ssh.Client, error) {
	cfg, err := getSSHConfig(d.username, d.sshKeyString, d.sshPassphrase, d.useSSHAgentAuth)
	if err != nil {
		return nil, fmt.Errorf("Error configuring SSH: %v", err)
	}

	// Establish connection with SSH server
	return ssh.Dial("tcp", d.sshAddress, cfg)
}

func (h *Host) newHTTPClient(dialerFactory DialerFactory) (*http.Client, error) {
	factory := dialerFactory
	if factory == nil {
		factory = SSHFactory
	}

	dialer, err := factory(h)
	if err != nil {
		return nil, err
	}
	dockerDialerTimeout := time.Second * DockerDialerTimeout
	return &http.Client{
		Transport: &http.Transport{
			Dial:                  dialer,
			TLSHandshakeTimeout:   dockerDialerTimeout,
			IdleConnTimeout:       dockerDialerTimeout,
			ResponseHeaderTimeout: dockerDialerTimeout,
		},
	}, nil
}
