package machine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/utils"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type sshClient struct {
	secrets  corecontrollers.SecretClient
	machines capicontrollers.MachineClient
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
	Subprotocols:     []string{"base64.channel.k8s.io"},
	Error:            onError,
}

// Define a short timeout for the connectivity check.
const dialTimeout = 5 * time.Second

func onError(rw http.ResponseWriter, _ *http.Request, code int, err error) {
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

func (s *sshClient) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())
	if err := apiRequest.AccessControl.CanUpdate(apiRequest, types.APIObject{}, apiRequest.Schema); err != nil {
		apiRequest.WriteError(err)
		return
	}

	switch apiRequest.Link {
	case "shell":
		if err := s.shell(apiRequest); err != nil {
			apiRequest.WriteError(err)
			return
		}
	case "sshkeys":
		if err := s.download(apiRequest); err != nil {
			apiRequest.WriteError(err)
			return
		}
	}
}

// getAccessibleAddress probes the machine's IPv4 and IPv6 addresses to find one that is reachable on the SSH port.
// It uses a quick TCP dial to check for connectivity before attempting a full SSH handshake.
func getAccessibleAddress(machineInfo *machineInfo) (string, error) {
	ipv4 := machineInfo.Driver.IPAddress
	ipv6 := machineInfo.Driver.IPv6Address
	name := machineInfo.Driver.MachineName

	if ipv4 == "" && ipv6 == "" {
		return "", fmt.Errorf("no IP addresses available for machine %s", name)
	}

	addresses := []string{ipv4, ipv6}

	kubeHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	if kubeHost != "" && utils.IsPlainIPV6(kubeHost) {
		addresses = []string{ipv6, ipv4}
	}

	for _, addr := range addresses {
		if addr == "" {
			continue
		}

		addrWithPort := net.JoinHostPort(addr, strconv.Itoa(machineInfo.Driver.SSHPort))

		conn, err := net.DialTimeout("tcp", addrWithPort, dialTimeout)
		if err == nil {
			_ = conn.Close()
			return addrWithPort, nil
		}
		logrus.Debugf("[ssh] Failed to probe machine %s at address %s: %v", name, addrWithPort, err)
	}

	return "", fmt.Errorf("failed to find an accessible IP address for machine %s", name)
}

// shell handles the "shell" action on a machine object. It establishes an interactive SSH
// session to the target machine and proxies it over a WebSocket connection.
func (s *sshClient) shell(apiRequest *types.APIRequest) error {
	ctx, cancel := context.WithCancel(apiRequest.Context())
	defer cancel()

	req := apiRequest.Request.WithContext(ctx)
	conn, err := upgrader.Upgrade(apiRequest.Response, req, nil)
	if err != nil {
		return err
	}

	defer conn.Close()
	machineInfo, err := s.getSSHKey(apiRequest.Namespace, apiRequest.Name)
	if err != nil {
		return err
	}

	signer, err := ssh.ParsePrivateKey(machineInfo.IDRSA)
	if err != nil {
		return err
	}

	addrWithPort, err := getAccessibleAddress(machineInfo)
	if err != nil {
		return err
	}

	logrus.Debugf("[ssh] Attempting to connect to machine %s via SSH at %s", apiRequest.Name, addrWithPort)
	client, err := ssh.Dial("tcp", addrWithPort, &ssh.ClientConfig{
		User: machineInfo.Driver.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to machine %s via SSH: %w", apiRequest.Name, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	if err := session.RequestPty("xterm", 20, 80, ssh.TerminalModes{}); err != nil {
		return err
	}

	stdIn, err := session.StdinPipe()
	if err != nil {
		return err
	}

	stdOut, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	go func() {
		defer cancel()
		defer conn.Close()
		io.Copy(&writer{conn: conn}, stdOut)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		s := string(data)
		if len(s) == 0 {
			continue
		}
		if s[0:1] == "0" {
			data, err := base64.StdEncoding.DecodeString(s[1:])
			if err != nil {
				return err
			}
			if _, err := stdIn.Write(data); err != nil {
				return err
			}
		} else if s[0:1] == "4" {
			data, err := base64.StdEncoding.DecodeString(s[1:])
			if err != nil {
				return err
			}
			resize := &resizeRequest{}
			if err := json.Unmarshal(data, resize); err != nil {
				return err
			}
			if err := session.WindowChange(resize.Height, resize.Width); err != nil {
				return err
			}
		}
	}
}

type resizeRequest struct {
	Height int
	Width  int
}

type machineInfo struct {
	IDRSA    []byte
	IDRSAPub []byte
	Driver   machineConfig
}

type machineConfig struct {
	IPAddress   string
	IPv6Address string
	SSHUser     string
	SSHPort     int
	MachineName string
}

func (s *sshClient) getSSHKey(machineNamespace, machineName string) (*machineInfo, error) {
	result := &machineInfo{}
	machine, err := s.machines.Get(machineNamespace, machineName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	secretName := capr.MachineStateSecretName(machine.Spec.InfrastructureRef.Name)
	secret, err := s.secrets.Get(machineNamespace, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(bytes.NewReader(secret.Data["extractedConfig"]))
	if err != nil {
		return nil, err
	}

	tar := tar.NewReader(gz)

	for {
		header, err := tar.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(tar)
		if err != nil {
			return nil, err
		}
		switch filepath.Base(header.Name) {
		case "id_rsa":
			result.IDRSA = data
		case "id_rsa.pub":
			result.IDRSAPub = data
		case "config.json":
			err := json.Unmarshal(data, result)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

type writer struct {
	conn *websocket.Conn
}

func (w *writer) Write(buf []byte) (int, error) {
	data := []byte("1" + base64.StdEncoding.EncodeToString(buf))
	m, err := w.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return 0, err
	}
	if _, err := m.Write(data); err != nil {
		return 0, err
	}
	return len(buf), m.Close()
}
