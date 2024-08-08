package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/norman/pkg/kwrapper/k8s"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	namespace        = "cattle-system"
	tlsName          = "apiserver-poc.default.svc"
	certName         = "cattle-apiextension-tls"
	caName           = "cattle-apiextension-ca"
	defaultHTTPSPort = 9443
)

func must(err error) {
	if err != nil {
		panic(err)
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

	var port int

	router := mux.NewRouter()
	if os.Getenv("IS_CLIENT") != "" {
		port = 5555
		ext.RegisterSubRoutes(router, wContext)

		stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
		out, errOut := new(bytes.Buffer), new(bytes.Buffer)

		go func() {
			roundTripper, upgrader, err := spdy.RoundTripperFor(restConfig)
			if err != nil {
				panic(err)
			}

			podName := os.Getenv("POD_NAME")
			path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
			hostIP := strings.TrimPrefix(restConfig.Host, "https://")
			serverURL := url.URL{
				Scheme: "https",
				Path:   path,
				Host:   hostIP,
			}
			dialer := spdy.NewDialer(upgrader, &http.Client{
				Transport: roundTripper,
			}, http.MethodPost, &serverURL)

			forwarder, err := portforward.New(dialer, []string{"5554"}, stopChan, readyChan, out, errOut)
			must(err)

			err = forwarder.ForwardPorts()
			must(err)
		}()

		go func() {
			for range readyChan { // Kubernetes will close this channel when it has something to tell us.
			}
			if len(errOut.String()) != 0 {
				panic(errOut.String())
			} else if len(out.String()) != 0 {
				fmt.Println(out.String())
			}
		}()

		go func() {
			fmt.Println("Waiting for proxy")
			<-readyChan
			fmt.Println("Proxy ready")
			dialer := &websocket.Dialer{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
			remotedialer.ClientConnect(
				ctx,
				"wss://localhost:5554/connect",
				http.Header{},
				dialer,
				func(string, string) bool { return true },
				nil,
			)
		}()

	} else {
		port = 5554
		authorizer := func(req *http.Request) (string, bool, error) {
			return "my-id", true, nil
		}
		remoteDialerServer := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)

		router.Handle("/connect", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			remoteDialerServer.ServeHTTP(w, req)
		}))

		// XXX: We don't need a http reverse proxy. Instead we could
		// have another port that would basically io.Copy() all data.
		// This way we don't MITM the connection and Rancher doesn't
		// need special auth handling.
		dialer := remoteDialerServer.Dialer("my-id")
		reverseProxy := httputil.ReverseProxy{
			Rewrite: func(proxy *httputil.ProxyRequest) {
				url := url.URL{
					Scheme: "https",
					Host:   "localhost:5555",
				}
				proxy.SetURL(&url)
			},
			Transport: &http.Transport{
				DialContext: dialer,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		router.PathPrefix("/").HandlerFunc(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			reverseProxy.ServeHTTP(w, req)
		}))
	}

	err = server.ListenAndServe(ctx, port, 0, router, &server.ListenOpts{
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
