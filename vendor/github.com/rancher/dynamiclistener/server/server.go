package server

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/dynamiclistener/storage/file"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/dynamiclistener/storage/memory"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
)

type ListenOpts struct {
	CA                *x509.Certificate
	CAKey             crypto.Signer
	Storage           dynamiclistener.TLSStorage
	Secrets           v1.SecretController
	CertNamespace     string
	CertName          string
	CANamespace       string
	CAName            string
	CertBackup        string
	AcmeDomains       []string
	BindHost          string
	NoRedirect        bool
	TLSListenerConfig dynamiclistener.Config
}

func ListenAndServe(ctx context.Context, httpsPort, httpPort int, handler http.Handler, opts *ListenOpts) error {
	if opts == nil {
		opts = &ListenOpts{}
	}

	if opts.TLSListenerConfig.TLSConfig == nil {
		opts.TLSListenerConfig.TLSConfig = &tls.Config{}
	}

	logger := logrus.StandardLogger()
	errorLog := log.New(logger.WriterLevel(logrus.DebugLevel), "", log.LstdFlags)

	if httpsPort > 0 {
		tlsTCPListener, err := dynamiclistener.NewTCPListener(opts.BindHost, httpsPort)
		if err != nil {
			return err
		}

		tlsTCPListener, handler, err = getTLSListener(ctx, tlsTCPListener, handler, *opts)
		if err != nil {
			return err
		}

		if !opts.NoRedirect {
			handler = dynamiclistener.HTTPRedirect(handler)
		}

		tlsServer := http.Server{
			Handler:  handler,
			ErrorLog: errorLog,
		}

		go func() {
			logrus.Infof("Listening on %s:%d", opts.BindHost, httpsPort)
			err := tlsServer.Serve(tlsTCPListener)
			if err != http.ErrServerClosed && err != nil {
				logrus.Fatalf("https server failed: %v", err)
			}
		}()
		go func() {
			<-ctx.Done()
			tlsServer.Shutdown(context.Background())
		}()
	}

	if httpPort > 0 {
		httpServer := http.Server{
			Addr:     fmt.Sprintf("%s:%d", opts.BindHost, httpPort),
			Handler:  handler,
			ErrorLog: errorLog,
		}
		go func() {
			logrus.Infof("Listening on %s:%d", opts.BindHost, httpPort)
			err := httpServer.ListenAndServe()
			if err != http.ErrServerClosed && err != nil {
				logrus.Fatalf("http server failed: %v", err)
			}
		}()
		go func() {
			<-ctx.Done()
			httpServer.Shutdown(context.Background())
		}()
	}

	return nil
}

func getTLSListener(ctx context.Context, tcp net.Listener, handler http.Handler, opts ListenOpts) (net.Listener, http.Handler, error) {
	if len(opts.TLSListenerConfig.TLSConfig.NextProtos) == 0 {
		opts.TLSListenerConfig.TLSConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	if len(opts.TLSListenerConfig.TLSConfig.Certificates) > 0 {
		return tls.NewListener(tcp, opts.TLSListenerConfig.TLSConfig), handler, nil
	}

	if len(opts.AcmeDomains) > 0 {
		return acmeListener(tcp, handler, opts)
	}

	storage := opts.Storage
	if storage == nil {
		storage = newStorage(ctx, opts)
	}

	caCert, caKey, err := getCA(opts)
	if err != nil {
		return nil, nil, err
	}

	listener, dynHandler, err := dynamiclistener.NewListener(tcp, storage, caCert, caKey, opts.TLSListenerConfig)
	if err != nil {
		return nil, nil, err
	}

	return listener, wrapHandler(dynHandler, handler), nil
}

func getCA(opts ListenOpts) (*x509.Certificate, crypto.Signer, error) {
	if opts.CA != nil && opts.CAKey != nil {
		return opts.CA, opts.CAKey, nil
	}

	if opts.Secrets == nil {
		return factory.LoadOrGenCA()
	}

	if opts.CAName == "" {
		opts.CAName = "serving-ca"
	}

	if opts.CANamespace == "" {
		opts.CANamespace = opts.CertNamespace
	}

	if opts.CANamespace == "" {
		opts.CANamespace = "kube-system"
	}

	return kubernetes.LoadOrGenCA(opts.Secrets, opts.CANamespace, opts.CAName)
}

func newStorage(ctx context.Context, opts ListenOpts) dynamiclistener.TLSStorage {
	var result dynamiclistener.TLSStorage
	if opts.CertBackup == "" {
		result = memory.New()
	} else {
		result = memory.NewBacked(file.New(opts.CertBackup))
	}

	if opts.Secrets == nil {
		return result
	}

	if opts.CertName == "" {
		opts.CertName = "serving-cert"
	}

	if opts.CertNamespace == "" {
		opts.CertNamespace = "kube-system"
	}

	return kubernetes.Load(ctx, opts.Secrets, opts.CertNamespace, opts.CertName, result)
}

func wrapHandler(handler http.Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(rw, req)
		next.ServeHTTP(rw, req)
	})
}

func acmeListener(tcp net.Listener, handler http.Handler, opts ListenOpts) (net.Listener, http.Handler, error) {
	hosts := map[string]bool{}
	for _, domain := range opts.AcmeDomains {
		hosts[domain] = true
	}

	manager := autocert.Manager{
		Cache: autocert.DirCache("certs-cache"),
		Prompt: func(tosURL string) bool {
			return true
		},
		HostPolicy: func(ctx context.Context, host string) error {
			if !hosts[host] {
				return fmt.Errorf("host %s is not configured", host)
			}
			return nil
		},
	}

	opts.TLSListenerConfig.TLSConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName == "localhost" || hello.ServerName == "" {
			newHello := *hello
			newHello.ServerName = opts.AcmeDomains[0]
			return manager.GetCertificate(&newHello)
		}
		return manager.GetCertificate(hello)
	}

	return tls.NewListener(tcp, opts.TLSListenerConfig.TLSConfig), manager.HTTPHandler(handler), nil
}
