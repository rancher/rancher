// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattn/go-colorable"
	"github.com/rancher/norman/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
	"k8s.io/client-go/util/cert"
)

func runMockServer(t *testing.T) *httptest.Server {
	caCert, _, err := pki.GenerateCACertAndKey("kube-ca", nil)
	if err != nil {
		t.Fatalf("Failed to generate CA, %v", err)
	}
	caCertPEM := cert.EncodeCertPEM(caCert)
	windowsNodeConfig := &rkeworker.NodeConfig{
		ClusterName: "dummy",
		Certs:       fmt.Sprintf(`{"kube-ca":{"CertPEM":%q,"name":"kube-ca","commonName":"system:kube-ca","ouName":"","envName":"KUBE_CA","path":"/etc/kubernetes/ssl/kube-ca.pem"}}`, string(caCertPEM)),
		Processes:   map[string]v3.Process{},
	}

	connectHandler := remotedialer.New(
		func(req *http.Request) (clientKey string, authed bool, err error) {
			token := req.Header.Get(Token)
			if token != "fake_agent_token" {
				return "", false, nil
			}

			return "fake_client_key", true, nil
		},
		remotedialer.DefaultErrorWriter,
	)

	connectConfigHandler := http.HandlerFunc(
		func(resp http.ResponseWriter, req *http.Request) {
			resp.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(resp).Encode(windowsNodeConfig); err != nil {
				t.Errorf("Failed to write windowsNodeConfig to agent: %v", err)
			}
		},
	)

	route := mux.NewRouter()
	route.UseEncodedPath()
	route.Handle("/v3/connect", connectHandler)
	route.Handle("/v3/connect/register", connectHandler)
	route.Handle("/v3/connect/config", connectConfigHandler)
	srv := httptest.NewTLSServer(route)

	return srv
}

// TestAgentRunning verifies the following process:
// 1. agent starts and registers to server
// 2. agent takes the worker node plan from server
// 3. agent executes the plan
// 4. agent stops gracefully
func TestAgentRunning(t *testing.T) {
	var (
		expectedRunningStates = []svc.State{svc.StartPending, svc.Running, svc.StopPending, svc.Stopped}
		testRunningStates     []svc.State
	)
	defer func() {
		if !reflect.DeepEqual(expectedRunningStates, testRunningStates) {
			t.Fatalf("Unmatched running states, %+v", testRunningStates)
		}
	}()

	dir, err := ioutil.TempDir("", "windows-agent")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// start server
	testSrv := runMockServer(t)

	// import server CA into Windows certificate chain for Windows Agent HTTP client
	serverCrtPath := filepath.Join(dir, "server.crt")
	ioutil.WriteFile(serverCrtPath, cert.EncodeCertPEM(testSrv.Certificate()), 0600)
	importCmd := exec.Command("powershell.exe", "-Sta", "-NoLogo", "-NonInteractive", "-Command", fmt.Sprintf("& {Import-Certificate -CertStoreLocation 'Cert:\\LocalMachine\\Root' -FilePath %s | Out-Null}", serverCrtPath))
	importCmd.Run()

	// set agent bootstrap params
	os.Setenv("CATTLE_INTERNAL_ADDRESS", "10.0.1.2")
	os.Setenv("CATTLE_NODE_NAME", "fake-windows-worker")
	os.Setenv("CATTLE_ADDRESS", "1.2.3.4")
	os.Setenv("CATTLE_ROLE", "worker")
	os.Setenv("CATTLE_SERVER", testSrv.URL)
	os.Setenv("CATTLE_TOKEN", "fake_agent_token")
	os.Setenv("CATTLE_NODE_LABEL", "rke.cattle.io/windows-build=17763,rke.cattle.io/windows-kernel-version=17763.1.amd64fre.rs5_release.180914-1434,rke.cattle.io/windows-major-version=10,rke.cattle.io/windows-minor-version=0,rke.cattle.io/windows-release-id=1809,rke.cattle.io/windows-version=10.0.17763.437")

	// simulate agent running logic
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	logrus.SetOutput(colorable.NewColorableStdout())
	logrus.SetLevel(logrus.DebugLevel)

	svcChangeRequest := make(chan svc.ChangeRequest)
	svcStatus := make(chan svc.Status)
	agent := newAgentService(nil)
	go agent.Execute(nil, svcChangeRequest, svcStatus)

	go func() {
		for {
			// simulate stopping agent gracefully, like: `Stop-Service rancher-agent` on Windows
			if _, err := os.Stat("c:\\etc\\kubernetes\\ssl\\kube-ca.pem"); err == nil {
				svcChangeRequest <- svc.ChangeRequest{Cmd: svc.Stop}
				return
			}
		}
	}()

	for {
		select {
		case c := <-svcStatus:
			testRunningStates = append(testRunningStates, c.State)
			if c.State == svc.Stopped {
				time.Sleep(5 * time.Second)
				return
			}
		case <-time.After(1 * time.Minute):
			t.Fatal("Timeout")
		}
	}
}
