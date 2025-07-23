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
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type sshClient struct {
	secrets         corecontrollers.SecretClient
	machines        capicontrollers.MachineClient
	wranglerContext *wrangler.Context
	capiInitOnce    sync.Once
	capiInitialized bool
}

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
	Subprotocols:     []string{"base64.channel.k8s.io"},
	Error:            onError,
}

func onError(rw http.ResponseWriter, _ *http.Request, code int, err error) {
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

// ensureCAPIInitialized ensures CAPI factory is initialized, doing so only once
func (s *sshClient) ensureCAPIInitialized(ctx context.Context) error {
	if s.capiInitialized {
		return nil
	}

	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if !s.wranglerContext.WaitForCAPICRDs(initCtx) {
		return fmt.Errorf("CAPI CRDs not yet available")
	}

	var initErr error
	s.capiInitOnce.Do(func() {
		logrus.Debugf("[ssh] Initializing CAPI factory on demand")

		if err := s.wranglerContext.InitializeCAPIFactory(ctx); err != nil {
			initErr = fmt.Errorf("failed to initialize CAPI factory: %w", err)
			return
		}

		if s.wranglerContext.CAPI != nil {
			s.machines = s.wranglerContext.CAPI.Machine()
			s.capiInitialized = true
			logrus.Infof("[ssh] CAPI factory initialized successfully")
		} else {
			initErr = fmt.Errorf("CAPI factory is nil after initialization")
		}
	})

	return initErr
}

func (s *sshClient) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !s.capiInitialized {
		if err := s.ensureCAPIInitialized(req.Context()); err != nil {
			logrus.Debugf("[ssh] CAPI not ready yet: %v", err)
			http.Error(rw, "Machine SSH service not ready, please retry", http.StatusServiceUnavailable)
			rw.Header().Set("Retry-After", "5")
			return
		}
	}

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

	addr := fmt.Sprintf("%s:%d", machineInfo.Driver.IPAddress, machineInfo.Driver.SSHPort)
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: machineInfo.Driver.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return err
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

		data, err := ioutil.ReadAll(tar)
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
