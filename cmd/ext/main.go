package main

import (
	"context"
	"os"

	"github.com/gorilla/mux"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/norman/pkg/kwrapper/k8s"
	"github.com/rancher/rancher/pkg/ext"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

const (
	namespace        = "default"
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

	router := mux.NewRouter()
	ext.RegisterSubRoutes(router, wContext)

	err = server.ListenAndServe(ctx, 5555, 0, router, &server.ListenOpts{
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
