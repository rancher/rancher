package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/norman/pkg/kwrapper/k8s"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
)

const (
	namespace = "cattle-system"
	tlsName   = "apiserver-poc.default.svc"
	certName  = "cattle-apiextension-tls"
	caName    = "cattle-apiextension-ca"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func runProxyListener(ctx context.Context, dialer remotedialer.Dialer) error {
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", ext.APIPort))
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print("proxy accept:", err)
			continue
		}

		peerAddr := fmt.Sprintf(":%d", ext.APIPort)
		clientConn, err := dialer(ctx, "tcp", peerAddr)
		if err != nil {
			log.Printf("proxy dial %s: %s", peerAddr, err)
			continue
		}

		// XXX: This works but should really be tested with failures,
		// etc I doubt it's robust
		pipe := func(a, b net.Conn) {
			defer a.Close()
			defer b.Close()
			io.Copy(a, b)
		}

		go pipe(conn, clientConn)
		go pipe(clientConn, conn)
	}
}

func main() {
	ctx := context.Background()

	_, clientConfig, err := k8s.GetConfig(ctx, "auto", os.Getenv("KUBECONFIG"))
	must(err)

	restConfig, err := clientConfig.ClientConfig()
	must(err)

	wContext, err := wrangler.NewContext(ctx, clientConfig, restConfig)
	must(err)

	wContext.Start(ctx)

	router := mux.NewRouter()
	authorizer := func(req *http.Request) (string, bool, error) {
		// XXX: Actually do authorization here with a shared Secret
		return "my-id", true, nil
	}
	remoteDialerServer := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)

	router.Handle("/connect", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		remoteDialerServer.ServeHTTP(w, req)
	}))

	go func() {
		if err := runProxyListener(ctx, remoteDialerServer.Dialer("my-id")); err != nil {
			panic(err)
		}
	}()

	err = server.ListenAndServe(ctx, ext.TunnelPort, 0, router, &server.ListenOpts{
		Secrets:       wContext.Core.Secret(),
		CAName:        caName,
		CANamespace:   namespace,
		CertName:      certName,
		CertNamespace: namespace,
		TLSListenerConfig: dynamiclistener.Config{
			SANs: []string{tlsName},
			FilterCN: func(cns ...string) []string {
				return []string{tlsName}
			},
		},
	})
	if err != nil {
		logrus.Errorf("extension server exited with: %s", err.Error())
	}
	<-ctx.Done()
}
