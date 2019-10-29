package hosts

import (
	"fmt"
	"k8s.io/client-go/transport"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/crypto/ssh"
)

const (
	DockerDialerTimeout = 50
)

type DialerFactory func(h *Host) (func(network, address string) (net.Conn, error), error)

type dialer struct {
	signer          ssh.Signer
	sshKeyString    string
	sshCertString   string
	sshAddress      string
	username        string
	netConn         string
	dockerSocket    string
	useSSHAgentAuth bool
	bastionDialer   *dialer
}

type DialersOptions struct {
	DockerDialerFactory    DialerFactory
	LocalConnDialerFactory DialerFactory
	K8sWrapTransport       transport.WrapperFunc
}

func GetDialerOptions(d, l DialerFactory, w transport.WrapperFunc) DialersOptions {
	return DialersOptions{
		DockerDialerFactory:    d,
		LocalConnDialerFactory: l,
		K8sWrapTransport:       w,
	}
}

func newDialer(h *Host, kind string) (*dialer, error) {
	// Check for Bastion host connection
	var bastionDialer *dialer
	if len(h.BastionHost.Address) > 0 {
		bastionDialer = &dialer{
			sshAddress:      fmt.Sprintf("%s:%s", h.BastionHost.Address, h.BastionHost.Port),
			username:        h.BastionHost.User,
			sshKeyString:    h.BastionHost.SSHKey,
			sshCertString:   h.BastionHost.SSHCert,
			netConn:         "tcp",
			useSSHAgentAuth: h.SSHAgentAuth,
		}
		if bastionDialer.sshKeyString == "" && !bastionDialer.useSSHAgentAuth {
			var err error
			bastionDialer.sshKeyString, err = privateKeyPath(h.BastionHost.SSHKeyPath)
			if err != nil {
				return nil, err
			}

			if bastionDialer.sshCertString == "" && len(h.BastionHost.SSHCertPath) > 0 {
				bastionDialer.sshCertString, err = certificatePath(h.BastionHost.SSHCertPath)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	dialer := &dialer{
		sshAddress:      fmt.Sprintf("%s:%s", h.Address, h.Port),
		username:        h.User,
		dockerSocket:    h.DockerSocket,
		sshKeyString:    h.SSHKey,
		sshCertString:   h.SSHCert,
		netConn:         "unix",
		useSSHAgentAuth: h.SSHAgentAuth,
		bastionDialer:   bastionDialer,
	}

	if dialer.sshKeyString == "" && !dialer.useSSHAgentAuth {
		var err error
		dialer.sshKeyString, err = privateKeyPath(h.SSHKeyPath)
		if err != nil {
			return nil, err
		}

		if dialer.sshCertString == "" && len(h.SSHCertPath) > 0 {
			dialer.sshCertString, err = certificatePath(h.SSHCertPath)
			if err != nil {
				return nil, err
			}
		}
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
	var conn *ssh.Client
	var err error
	if d.bastionDialer != nil {
		conn, err = d.getBastionHostTunnelConn()
	} else {
		conn, err = d.getSSHTunnelConnection()
	}
	if err != nil {
		if strings.Contains(err.Error(), "no key found") {
			return nil, fmt.Errorf("Unable to access node with address [%s] using SSH. Please check if the configured key or specified key file is a valid SSH Private Key. Error: %v", d.sshAddress, err)
		} else if strings.Contains(err.Error(), "no supported methods remain") {
			return nil, fmt.Errorf("Unable to access node with address [%s] using SSH. Please check if you are able to SSH to the node using the specified SSH Private Key and if you have configured the correct SSH username. Error: %v", d.sshAddress, err)
		} else if strings.Contains(err.Error(), "cannot decode encrypted private keys") {
			return nil, fmt.Errorf("Unable to access node with address [%s] using SSH. Using encrypted private keys is only supported using ssh-agent. Please configure the option `ssh_agent_auth: true` in the configuration file or use --ssh-agent-auth as a parameter when running RKE. This will use the `SSH_AUTH_SOCK` environment variable. Error: %v", d.sshAddress, err)
		} else if strings.Contains(err.Error(), "operation timed out") {
			return nil, fmt.Errorf("Unable to access node with address [%s] using SSH. Please check if the node is up and is accepting SSH connections or check network policies and firewall rules. Error: %v", d.sshAddress, err)
		}
		return nil, fmt.Errorf("Failed to dial ssh using address [%s]: %v", d.sshAddress, err)
	}

	// Docker Socket....
	if d.netConn == "unix" {
		addr = d.dockerSocket
		network = d.netConn
	}

	remote, err := conn.Dial(network, addr)
	if err != nil {
		if strings.Contains(err.Error(), "connect failed") {
			return nil, fmt.Errorf("Unable to access the service on %s. The service might be still starting up. Error: %v", addr, err)
		} else if strings.Contains(err.Error(), "administratively prohibited") {
			return nil, fmt.Errorf("Unable to access the Docker socket (%s). Please check if the configured user can execute `docker ps` on the node, and if the SSH server version is at least version 6.7 or higher. If you are using RedHat/CentOS, you can't use the user `root`. Please refer to the documentation for more instructions. Error: %v", addr, err)
		}
		return nil, fmt.Errorf("Failed to dial to %s: %v", addr, err)
	}
	return remote, err
}

func (d *dialer) getSSHTunnelConnection() (*ssh.Client, error) {
	cfg, err := getSSHConfig(d.username, d.sshKeyString, d.sshCertString, d.useSSHAgentAuth)
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

func (d *dialer) getBastionHostTunnelConn() (*ssh.Client, error) {
	bastionCfg, err := getSSHConfig(d.bastionDialer.username, d.bastionDialer.sshKeyString, d.bastionDialer.sshCertString, d.bastionDialer.useSSHAgentAuth)
	if err != nil {
		return nil, fmt.Errorf("Error configuring SSH for bastion host [%s]: %v", d.bastionDialer.sshAddress, err)
	}
	bastionClient, err := ssh.Dial("tcp", d.bastionDialer.sshAddress, bastionCfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the bastion host [%s]: %v", d.bastionDialer.sshAddress, err)
	}
	conn, err := bastionClient.Dial(d.bastionDialer.netConn, d.sshAddress)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the host [%s]: %v", d.sshAddress, err)
	}
	cfg, err := getSSHConfig(d.username, d.sshKeyString, d.sshCertString, d.useSSHAgentAuth)
	if err != nil {
		return nil, fmt.Errorf("Error configuring SSH for host [%s]: %v", d.sshAddress, err)
	}
	newClientConn, channels, sshRequest, err := ssh.NewClientConn(conn, d.sshAddress, cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to establish new ssh client conn [%s]: %v", d.sshAddress, err)
	}
	return ssh.NewClient(newClientConn, channels, sshRequest), nil
}

func BastionHostWrapTransport(bastionHost v3.BastionHost) (transport.WrapperFunc, error) {

	bastionDialer := &dialer{
		sshAddress:      fmt.Sprintf("%s:%s", bastionHost.Address, bastionHost.Port),
		username:        bastionHost.User,
		sshKeyString:    bastionHost.SSHKey,
		sshCertString:   bastionHost.SSHCert,
		netConn:         "tcp",
		useSSHAgentAuth: bastionHost.SSHAgentAuth,
	}

	if bastionDialer.sshKeyString == "" && !bastionDialer.useSSHAgentAuth {
		var err error
		bastionDialer.sshKeyString, err = privateKeyPath(bastionHost.SSHKeyPath)
		if err != nil {
			return nil, err
		}

	}

	if bastionDialer.sshCertString == "" && len(bastionHost.SSHCertPath) > 0 {
		var err error
		bastionDialer.sshCertString, err = certificatePath(bastionHost.SSHCertPath)
		if err != nil {
			return nil, err
		}

	}
	return func(rt http.RoundTripper) http.RoundTripper {
		if ht, ok := rt.(*http.Transport); ok {
			ht.DialContext = nil
			ht.DialTLS = nil
			ht.Dial = bastionDialer.Dial
		}
		return rt
	}, nil
}
