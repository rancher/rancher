package rkeworker

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"sync"

	"fmt"
	"net"
	"os/exec"

	"github.com/rancher/rancher/pkg/clusterrouter/proxy"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/sirupsen/logrus"
)

const (
	tlsKey  = "/etc/kubernetes/ssl/kube-apiserver-key.pem"
	tlsCert = "/etc/kubernetes/ssl/kube-apiserver.pem"
)

var (
	apiProxy sync.Once
)

func ExecutePlan(ctx context.Context, serverURL string, nodeConfig *NodeConfig) error {
	if nodeConfig.Certs != "" {
		bundle, err := rkecerts.Unmarshal(nodeConfig.Certs)
		if err != nil {
			return err
		}

		if err := bundle.Explode(); err != nil {
			return err
		}
	}

	for name, process := range nodeConfig.Processes {
		if err := runProcess(ctx, name, process); err != nil {
			return err
		}
	}

	if nodeConfig.APIProxyAddress != "" {
		apiProxy.Do(func() {
			if err := startHTTPServer(nodeConfig.APIProxyAddress, serverURL); err != nil {
				logrus.Fatalf("Failed to start API proxy: %v", err)
			}
		})
	}

	return nil
}

func startHTTPServer(address, serverURL string) error {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return err
	}

	proxy, err := proxy.NewSimpleProxy(parsedURL.Host, nil)
	if err != nil {
		return err
	}

	wrapped := func(rw http.ResponseWriter, req *http.Request) {
		req.Header.Set("X-API-K8s-Node-Client", "true")
		req.Host = parsedURL.Host
		proxy.ServeHTTP(rw, req)
	}

	host, _, err := net.SplitHostPort(address)
	if err == nil {
		err := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/32", host), "dev", "lo").Run()
		if err != nil {
			logrus.Warnf("Failed to assign IP %s: %v", host, err)
		}
	}

	go func() {
		err := http.ListenAndServeTLS(address, tlsCert, tlsKey, http.HandlerFunc(wrapped))
		log.Fatal(err)
	}()

	return nil
}
