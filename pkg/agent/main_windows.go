package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"path/filepath"

	"github.com/gorilla/websocket"
	"github.com/mattn/go-colorable"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	VERSION = "dev"
)

const (
	Token         = "X-API-Tunnel-Token"
	Params        = "X-API-Tunnel-Params"
	installedPath = `C:\etc\rancher`
	connectedPath = installedPath + `\connected`

	windowsServiceName              = "rancher-agent"
	defaultResetConnectedRetryCount = 3
)

// see https://github.com/docker/engine/blob/8e610b2b55bfd1bfa9436ab110d311f5e8a74dcb/cmd/dockerd/service_windows.go#L64-L149

const (
	// These should match the values in event_messages.mc.
	eventInfo  = 1
	eventWarn  = 1
	eventError = 1
	eventDebug = 2
	eventPanic = 3
	eventFatal = 4

	eventExtraOffset = 10 // Add this to any event to get a string that supports extended data
)

type etwHook struct {
	log *eventlog.Log
}

func (h *etwHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (h *etwHook) Fire(e *logrus.Entry) error {
	var (
		etype uint16
		eid   uint32
	)

	switch e.Level {
	case logrus.PanicLevel:
		etype = windows.EVENTLOG_ERROR_TYPE
		eid = eventPanic
	case logrus.FatalLevel:
		etype = windows.EVENTLOG_ERROR_TYPE
		eid = eventFatal
	case logrus.ErrorLevel:
		etype = windows.EVENTLOG_ERROR_TYPE
		eid = eventError
	case logrus.WarnLevel:
		etype = windows.EVENTLOG_WARNING_TYPE
		eid = eventWarn
	case logrus.InfoLevel:
		etype = windows.EVENTLOG_INFORMATION_TYPE
		eid = eventInfo
	case logrus.DebugLevel:
		etype = windows.EVENTLOG_INFORMATION_TYPE
		eid = eventDebug
	default:
		return errors.New("unknown level")
	}

	// If there is additional data, include it as a second string.
	exts := ""
	if len(e.Data) > 0 {
		fs := bytes.Buffer{}
		for k, v := range e.Data {
			fs.WriteString(k)
			fs.WriteByte('=')
			fmt.Fprint(&fs, v)
			fs.WriteByte(' ')
		}

		exts = fs.String()[:fs.Len()-1]
		eid += eventExtraOffset
	}

	if h.log == nil {
		fmt.Fprintf(os.Stderr, "%s [%s]\n", e.Message, exts)
		return nil
	}

	var (
		ss  [2]*uint16
		err error
	)

	ss[0], err = windows.UTF16PtrFromString(e.Message)
	if err != nil {
		return err
	}

	count := uint16(1)
	if exts != "" {
		ss[1], err = windows.UTF16PtrFromString(exts)
		if err != nil {
			return err
		}

		count++
	}

	return windows.ReportEvent(h.log.Handle, etype, 0, eid, 0, count, 0, &ss[0], nil)
}

var (
	eventLogRegisterCallbackFn = func() error {
		eventlog.InstallAsEventCreate(windowsServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
		return nil
	}
	eventLogUnregisterCallbackFn = func() error {
		eventlog.Remove(windowsServiceName)
		return nil
	}
)

type callbackFn func() error

type agentService struct {
	isStop bool
	done   chan struct{}
}

func (a *agentService) Execute(_ []string, svcChangeRequest <-chan svc.ChangeRequest, svcStatus chan<- svc.Status) (bool, uint32) {
	selfChangeRequest := make(chan svc.ChangeRequest)

	svcStatus <- svc.Status{State: svc.StartPending, Accepts: 0}
	if err := a.start(selfChangeRequest); err != nil {
		logrus.Debugf("Rancher agent version %s aborting due to failure during initialization", VERSION)
		logrus.Errorf("Rancher agent version %s failed: %s", VERSION, err.Error())
		return true, 1
	}

	svcStatus <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop}
	logrus.Debugf("Rancher agent version %s running", VERSION)

loop:
	for {
		select {
		case c := <-svcChangeRequest:
			switch c.Cmd {
			case svc.Interrogate:
				svcStatus <- c.CurrentStatus
			case svc.Stop:
				logrus.Debugf("Rancher agent version %s stopping", VERSION)
				svcStatus <- svc.Status{State: svc.StopPending, Accepts: 0}
				a.stop()
				break loop
			}
		case c := <-selfChangeRequest:
			switch c.Cmd {
			case svc.Stop:
				logrus.Debugf("Rancher agent version %s stopping", VERSION)
				svcStatus <- svc.Status{State: svc.StopPending, Accepts: 0}
				a.stop()
			case svc.Shutdown:
				logrus.Debugf("Rancher agent version %s shutting down", VERSION)
				svcStatus <- svc.Status{State: svc.StartPending, Accepts: 0}
				a.shutdown(func() error {
					return eventLogUnregisterCallbackFn()
				})
			}

			break loop
		}
	}

	return false, 0
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
	_, err := os.Stat(connectedPath)
	return err == nil
}

func resetConnected() {
	os.RemoveAll(connectedPath)
}

func connected() {
	f, _ := os.Create(connectedPath)
	defer f.Close()
}

func loadServerCAs() (*x509.CertPool, error) {
	sslCertPath := os.Getenv("SSL_CERT_DIR")
	if len(sslCertPath) == 0 {
		return nil, errors.New("failed to load CA for server, can't get SSL_CERT_PATH environment variable")
	}

	servercaBytes, err := ioutil.ReadFile(strings.Join([]string{sslCertPath, "serverca"}, string(os.PathSeparator)))
	if err != nil {
		return nil, fmt.Errorf("failed to load CA for server, %v", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(servercaBytes) {
		return nil, errors.New("failed to load CA for server, can't append cert from PEM block")
	}

	return caPool, nil
}

type clientFactory struct {
	serverCAs *x509.CertPool
}

func (cf *clientFactory) http() *http.Client {
	return &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: cf.serverCAs,
			},
		},
	}
}

func (cf *clientFactory) ws() *websocket.Dialer {
	return &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			RootCAs: cf.serverCAs,
		},
	}
}

func newClientFactory() (*clientFactory, error) {
	serverCAs, err := loadServerCAs()
	if err != nil {
		return nil, err
	}

	return &clientFactory{
		serverCAs: serverCAs,
	}, nil
}

func (a *agentService) stop() {
	if a.isStop {
		return
	}

	close(a.done)

	rkeworker.Stop(context.Background())

	logrus.Infof("Rancher agent version %s was stopped", VERSION)

	a.isStop = true
}

func (a *agentService) shutdown(callback callbackFn) {
	a.stop()

	rkeworker.Remove(context.Background(), func() error {
		resetConnected()
		return unregisterService(nil)
	})

	logrus.Debugf("Rancher agent version %s shutdown", VERSION)

	if callback != nil {
		callback()
	}
}

func (a *agentService) start(selfChangeRequest chan<- svc.ChangeRequest) error {
	logrus.Infof("Rancher agent version %s is starting", VERSION)

	clientPool, err := newClientFactory()
	if err != nil {
		return err
	}

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
		if err := rkenodeconfigclient.ConfigClientWhileWindows(
			ctx,
			clientPool.http(),
			connectConfig,
			headers,
			false,
		); err != nil {
			return err
		}

		logrus.Infoln("Starting plan monitor")

	loop:
		for {
			select {
			case <-time.After(2 * time.Minute):
				if err := rkenodeconfigclient.ConfigClientWhileWindows(
					ctx,
					clientPool.http(),
					connectConfig,
					headers,
					false,
				); err != nil {
					if _, ok := err.(*rkenodeconfigclient.ErrNodeOrClusterNotFound); ok {
						return err
					}

					logrus.Errorf("Failed to check plan: %v", err)
				}
			case <-ctx.Done():
				logrus.Infoln("Stopped plan monitor")
				break loop
			}
		}

		return nil
	}

	go func() {
		dialerClose := make(chan int64)
		resetConnectedRetryCount := defaultResetConnectedRetryCount

	loop:
		for {
			connected := isConnected()

			ctx, cancel := context.WithCancel(context.Background())

			wsURL := fmt.Sprintf("wss://%s/v3/connect", serverURL.Host)
			if !connected {
				wsURL += "/register"
			}
			logrus.Infof("Connecting to proxy %s with token %s", wsURL, token)

			go func() {
				dialerClose <- remotedialer.ClientConnectWhileWindows(
					ctx,
					wsURL,
					http.Header(headers),
					clientPool.ws(),
					func(proto, address string) bool {
						switch proto {
						case "tcp":
							return true
						case "npipe":
							return address == `//./pipe/docker_engine`
						}
						return false
					},
					blockingOnConnect,
				)
			}()

			select {
			case status := <-dialerClose:
				cancel()

				switch status {
				case 200:
					// stop
					selfChangeRequest <- svc.ChangeRequest{Cmd: svc.Stop}
					break loop
				case 403:
					// reset connected file, reconnect again
					if resetConnectedRetryCount <= 0 {
						logrus.Warn("Proxy reject this connection, this host was connected to a rancher server before.")
						logrus.Info("Try to reset ", connectedPath)

						resetConnected()
						resetConnectedRetryCount = defaultResetConnectedRetryCount
					}
					resetConnectedRetryCount -= 1
				case 503:
					// shutdown
					selfChangeRequest <- svc.ChangeRequest{Cmd: svc.Shutdown}
					break loop
				}

				time.Sleep(10 * time.Second)
			case <-a.done:
				cancel()
				break loop
			}
		}
	}()

	return nil
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func registerService(callback callbackFn) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	// if a service can open then means it was registered
	s, err := m.OpenService(windowsServiceName)
	if err == nil {
		s.Close()
		return nil
	}

	s, err = m.CreateService(
		windowsServiceName,
		exepath,
		mgr.Config{
			ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
			StartType:    mgr.StartAutomatic,
			ErrorControl: mgr.ErrorNormal,
			DisplayName:  "Rancher Agent",
		},
	)
	if err != nil {
		return err
	}
	s.Close()

	if callback != nil {
		return callback()
	}

	return nil
}

func unregisterService(callback callbackFn) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	// if a service can open then means it was registered
	s, err := m.OpenService(windowsServiceName)
	if err == nil {
		err = s.Delete()
		if err != nil {
			return err
		}
		s.Close()
	}

	if callback != nil {
		return callback()
	}

	return nil
}

var (
	flRegisterService   = flag.Bool("register-service", false, "Register the service and exit")
	flUnregisterService = flag.Bool("unregister-service", false, "Unregister the service and exit")
	versionPrint        = flag.Bool("version", false, "Print version and exit")
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	logrus.SetOutput(colorable.NewColorableStdout())

	flag.Parse()

	if *versionPrint {
		fmt.Printf("Rancher agent version is %s.\n", VERSION)
		return
	}

	if *flUnregisterService {
		if *flRegisterService {
			logrus.Fatal("--register-service and --unregister-service cannot be used together")
		}

		if err := unregisterService(eventLogUnregisterCallbackFn); err != nil {
			logrus.Fatal(err)
		}
		return
	}

	if *flRegisterService {
		if err := registerService(eventLogRegisterCallbackFn); err != nil {
			logrus.Fatal(err)
		}
		return
	}

	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		logrus.Fatal(err)
	}

	if !isInteractive {
		elog, err := eventlog.Open(windowsServiceName)
		if err != nil {
			logrus.Fatal(err)
		}

		logrus.AddHook(&etwHook{elog})
		logrus.SetOutput(ioutil.Discard)
	}

	run := svc.Run
	if isInteractive {
		run = debug.Run
	}

	if isTrue(os.Getenv("CATTLE_DEBUG")) || isTrue(os.Getenv("RANCHER_DEBUG")) {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := run(windowsServiceName, &agentService{isStop: false, done: make(chan struct{})}); err != nil {
		logrus.Fatalf("Rancher agent failed: %v", err)
	}
}
