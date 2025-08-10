package proxy

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"

	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/rancher/remotedialer"
)

const (
	listClientsRetryCount = 10
	listClientSleepTime   = 1 * time.Second
)

func runProxyListener(ctx context.Context, cfg *Config, server *remotedialer.Server) error {
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", cfg.ProxyPort)) //this RDP app starts only once and always running
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept() // the client of 6666 is kube-apiserver, according to the APIService object spec, just to this TCP 6666
		if err != nil {
			logrus.Errorf("proxy TCP connection accept failed: %v", err)
			continue
		}

		go func() {
			var retryTimes = 0
			for {
				clients := server.ListClients()
				if len(clients) == 0 {
					retryTimes++
					if retryTimes > listClientsRetryCount {
						conn.Close()
						return
					}

					logrus.Info("proxy TCP connection failed: no clients, retrying in a sec")
					time.Sleep(listClientSleepTime)
				} else {
					client := clients[rand.Intn(len(clients))]
					peerAddr := fmt.Sprintf(":%d", cfg.PeerPort) // rancher's special https server for imperative API
					clientConn, err := server.Dialer(client)(ctx, "tcp", peerAddr)
					if err != nil {
						logrus.Errorf("proxy dialing %s failed: %v", peerAddr, err)
						conn.Close()
						return
					}

					go pipe(conn, clientConn)
					go pipe(clientConn, conn)
					break
				}
			}
		}()
	}
}

func pipe(a, b net.Conn) {
	defer func(a net.Conn) {
		if err := a.Close(); err != nil {
			logrus.Errorf("proxy TCP connection close failed: %v", err)
		}
	}(a)
	defer func(b net.Conn) {
		if err := b.Close(); err != nil {
			logrus.Errorf("proxy TCP connection close failed: %v", err)
		}
	}(b)
	n, err := io.Copy(a, b)
	if err != nil {
		logrus.Errorf("proxy copy failed: %v", err)
		return
	}
	logrus.Debugf("proxy copied %d bytes to %v from %v", n, a.LocalAddr(), b.LocalAddr())
}

func Start(cfg *Config, restConfig *rest.Config) error {
	logrus.SetLevel(logrus.DebugLevel)
	ctx := context.Background()

	// Setting Up Default Authorizer
	authorizer := func(req *http.Request) (string, bool, error) {
		id := req.Header.Get("X-API-Tunnel-Secret")
		if id != cfg.Secret {
			return "", false, fmt.Errorf("X-API-Tunnel-Secret not specified in request header")
		}
		return id, true, nil
	}

	// Initializing Remote Dialer Server
	remoteDialerServer := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)

	router := mux.NewRouter()
	router.Handle("/connect", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logrus.Info("got a connection")
		remoteDialerServer.ServeHTTP(w, req)
	}))

	go func() {
		if err := runProxyListener(ctx, cfg, remoteDialerServer); err != nil {
			logrus.Errorf("proxy listener failed to start in the background: %v", err)
		}
	}()

	// Setting Up Secret Controller
	core, err := core.NewFactoryFromConfigWithOptions(restConfig, nil)
	if err != nil {
		return fmt.Errorf("build secret controller failed w/ err: %w", err)
	}

	if err := core.Start(ctx, 1); err != nil {
		return fmt.Errorf("secretController factory start failed: %w", err)
	}

	secretController := core.Core().V1().Secret()

	// Setting Up Remote Dialer HTTPS Server
	if err := server.ListenAndServe(ctx, cfg.HTTPSPort, 0, router, &server.ListenOpts{
		Secrets:       secretController,
		CAName:        cfg.CAName,
		CertName:      cfg.CertCAName,
		CertNamespace: cfg.CertCANamespace,
		TLSListenerConfig: dynamiclistener.Config{
			SANs: []string{cfg.TLSName},
			FilterCN: func(cns ...string) []string {
				return []string{cfg.TLSName}
			},
			RegenerateCerts: func() bool {
				return true
			},
			ExpirationDaysCheck: 10,
		},
	}); err != nil {
		return fmt.Errorf("extension server exited with an error: %w", err)
	}
	<-ctx.Done()
	return nil
}
