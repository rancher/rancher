package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rancher/rancher/pkg/agent/node"
	libwindows "github.com/rancher/rancher/pkg/agent/windows"
	"github.com/rancher/rancher/pkg/agent/windows/remotedialer"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc"
)

var (
	VERSION = "dev"
)

const (
	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

type agentService struct {
	stopOnce  sync.Once
	winRunner *libwindows.WinRunner
	ctx       context.Context
	cancel    context.CancelFunc
}

func (a *agentService) Execute(_ []string, svcChangeRequest <-chan svc.ChangeRequest, svcStatus chan<- svc.Status) (bool, uint32) {
	selfChangeRequest := make(chan svc.ChangeRequest)
	defer close(selfChangeRequest)

	svcStatus <- svc.Status{State: svc.StartPending, Accepts: 0}
	if err := a.start(selfChangeRequest); err != nil {
		logrus.Errorf("Agent cannot start, %v", err)
		return true, 1
	}

	svcStatus <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	logrus.Infoln("Agent running")

	for {
		select {
		case c := <-svcChangeRequest:
			switch c.Cmd {
			case svc.Interrogate:
				svcStatus <- c.CurrentStatus
			case svc.Stop:
				a.stop(svcStatus)
				return false, 0
			case svc.Shutdown:
				a.shutdown(svcStatus)
				return false, 0
			}
		case c := <-selfChangeRequest:
			// for now selfChangeRequest only return `svc.Stop` and `svc.Shutdown`
			switch c.Cmd {
			case svc.Shutdown:
				a.shutdown(svcStatus)
			default:
				a.stop(svcStatus)
			}

			return false, 0
		}
	}
}

func isTrue(envVal string) bool {
	if len(envVal) == 0 {
		return false
	}

	return strings.ToLower(envVal) == "true"
}

func getParams() (map[string]interface{}, error) {
	return node.Params(), nil
}

func getTokenAndURL() (string, string, error) {
	return node.TokenAndURL()
}

func isConnected() bool {
	_, err := os.Stat("connected")
	return err == nil
}

func resetConnected() {
	os.RemoveAll("connected")
}

func connected() {
	f, _ := os.Create("connected")
	defer f.Close()
}

func (a *agentService) stop(svcStatus chan<- svc.Status) {
	a.stopOnce.Do(func() {
		logrus.Infoln("Agent stopping")
		svcStatus <- svc.Status{State: svc.StopPending, Accepts: svc.AcceptShutdown}

		a.cancel()

		// stop kubelet, kube-proxy, networking
		if err := rkeworker.Stop(context.Background()); err != nil {
			logrus.Errorln("Agent cannot stop worker", err)
		}

		svcStatus <- svc.Status{State: svc.Stopped, Accepts: svc.AcceptShutdown}
		logrus.Infoln("Agent stopped")
	})
}

func (a *agentService) shutdown(svcStatus chan<- svc.Status) {
	a.stop(svcStatus)

	logrus.Infoln("Agent shutting down")

	// remove kubelet, kube-proxy, networking
	if err := rkeworker.Remove(context.Background()); err != nil {
		logrus.Errorln("Agent cannot remove worker", err)
	}

	logrus.Infoln("Agent shut down")

	// unregister agent service
	if err := a.winRunner.UnRegister(); err != nil {
		logrus.Errorln("Agent cannot be unregistered", err)
	}
}

func (a *agentService) start(selfChangeRequest chan<- svc.ChangeRequest) error {
	logrus.Infoln("Agent starting")

	params, err := getParams()
	if err != nil {
		return err
	}

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	token, server, err := getTokenAndURL()
	if err != nil {
		return err
	}

	headers := map[string][]string{
		Token:  {token},
		Params: {base64.StdEncoding.EncodeToString(paramBytes)},
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	blockingOnConnect := func(ctx context.Context) error {
		connected()
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		if err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, false); err != nil {
			return err
		}

		// windows agent only acts as a node agent

		logrus.Infoln("Starting plan monitor")

		for {
			select {
			case <-time.After(2 * time.Minute):
				err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, false)
				if err != nil {
					// return the error if the node cannot connect to server or remove from a cluster
					if _, ok := err.(*rkenodeconfigclient.ErrNodeOrClusterNotFound); ok {
						return err
					}

					logrus.Errorf("Failed to check plan: %v", err)
				}
			case <-ctx.Done():
				logrus.Infoln("Stopped plan monitor")
				return nil
			}
		}
	}

	go doConnect(a.ctx, serverURL.Host, token, headers, blockingOnConnect, selfChangeRequest)

	return nil
}

func doConnect(ctx context.Context, host, token string, headers map[string][]string, onConnect func(ctx context.Context) error, selfChangeRequest chan<- svc.ChangeRequest) {
	defer resetConnected()

	connectingStatus := make(chan remotedialer.ConnectingStatus)
	defer close(connectingStatus)

	connectionRetryLimit := 3
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wsURL := fmt.Sprintf("wss://%s/v3/connect", host)
		if !isConnected() {
			wsURL += "/register"
		}
		logrus.Infof("Connect to %s with token %s", wsURL, token)

		connectCtx, cancel := context.WithCancel(ctx)
		go func(ctx context.Context) {
			connectingStatus <- remotedialer.ClientConnectWhileWindows(
				ctx,
				wsURL,
				headers,
				nil,
				func(proto, address string) bool {
					switch proto {
					case "tcp":
						return true
					case "npipe":
						return address == `//./pipe/docker_engine`
					}
					return false
				},
				onConnect,
			)
		}(connectCtx)

		select {
		case <-ctx.Done():
			connectingStatus = nil
			cancel()
			return
		case status := <-connectingStatus:
			cancel()
			switch status {
			case remotedialer.ConnectingStatusRetry:
				if connectionRetryLimit < 1 {
					logrus.Warnln("Connection retry timeout")
					selfChangeRequest <- svc.ChangeRequest{Cmd: svc.Stop}
					return
				}

				logrus.Debugf("Connection retry to %d times", connectionRetryLimit)
				connectionRetryLimit--
			case remotedialer.ConnectingStatusStopped:
				logrus.Infoln("Connection has stopped")
				selfChangeRequest <- svc.ChangeRequest{Cmd: svc.Stop}
				return
			case remotedialer.ConnectionStatusLost:
				logrus.Infoln("Connection has lost")
				selfChangeRequest <- svc.ChangeRequest{Cmd: svc.Shutdown}
				return
			}

			time.Sleep(10 * time.Second)
		}
	}
}

func main() {
	flRegisterService := flag.Bool("register-service", false, "Register the service and exit")
	flUnregisterService := flag.Bool("unregister-service", false, "Unregister the service and exit")
	versionPrint := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	logrus.SetOutput(colorable.NewColorableStdout())

	if *versionPrint {
		fmt.Printf("Rancher agent version is %s.\n", VERSION)
		return
	}

	if *flUnregisterService && *flRegisterService {
		logrus.Fatal("--register-service and --unregister-service are mutually exclusive")
	}

	runner := &libwindows.WinRunner{
		ServiceName:        "rancher-agent",
		ServiceDisplayName: "Rancher Agent",
	}
	runner.ServiceHandler = newAgentService(runner)

	if *flUnregisterService {
		if err := runner.UnRegister(); err != nil {
			logrus.Fatal("Failed to unregister service", err)
		}
		return
	}

	if *flRegisterService {
		if err := runner.Register(); err != nil {
			logrus.Fatal("Failed to register service", err)
		}
		return
	}

	if isTrue(os.Getenv("CATTLE_DEBUG")) || isTrue(os.Getenv("RANCHER_DEBUG")) {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := runner.Start(); err != nil {
		logrus.Fatalf("Rancher agent failed: %v", err)
	}
}

func newAgentService(winRunner *libwindows.WinRunner) *agentService {
	ctx, cancel := context.WithCancel(context.Background())

	return &agentService{
		winRunner: winRunner,
		ctx:       ctx,
		cancel:    cancel,
	}
}
