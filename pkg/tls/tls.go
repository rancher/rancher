package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"k8s.io/client-go/rest"
)

const (
	rancherCertFile    = "/etc/rancher/ssl/cert.pem"
	rancherKeyFile     = "/etc/rancher/ssl/key.pem"
	rancherCACertsFile = "/etc/rancher/ssl/cacerts.pem"
)

func ListenAndServe(ctx context.Context, restConfig *rest.Config, handler http.Handler, httpsPort, httpPort int, acmeDomains []string, noCACerts bool) error {
	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		return err
	}

	opts, err := SetupListener(core.Core().V1().Secret(), acmeDomains, noCACerts)
	if err != nil {
		return err
	}

	if err := server.ListenAndServe(ctx, httpsPort, httpPort, handler, opts); err != nil {
		return err
	}

	if err := core.Start(ctx, 5); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()

}

func SetupListener(secrets v1.SecretController, acmeDomains []string, noCACerts bool) (*server.ListenOpts, error) {
	caForAgent, opts, err := readConfig(secrets, acmeDomains, noCACerts)
	if err != nil {
		return nil, err
	}

	if noCACerts {
		caForAgent = ""
	} else if caForAgent == "" {
		caCert, caKey, err := kubernetes.LoadOrGenCA(secrets, opts.CANamespace, opts.CAName)
		if err != nil {
			return nil, err
		}
		caForAgent = string(cert.EncodeCertPEM(caCert))
		opts.CA = caCert
		opts.CAKey = caKey
	}

	if settings.CACerts.Get() != caForAgent {
		if err := settings.CACerts.Set(caForAgent); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func readConfig(secrets v1.SecretController, acmeDomains []string, noCACerts bool) (string, *server.ListenOpts, error) {
	var (
		ca  string
		err error
	)

	tlsConfig, err := BaseTLSConfig()
	if err != nil {
		return "", nil, err
	}

	expiration, err := strconv.Atoi(settings.RotateCertsIfExpiringInDays.Get())
	if err != nil {
		return "", nil, errors.Wrapf(err, "parsing %s", settings.RotateCertsIfExpiringInDays.Get())
	}

	opts := &server.ListenOpts{
		Secrets:       secrets,
		CAName:        "serving-ca",
		CANamespace:   "cattle-system",
		CertNamespace: "cattle-system",
		AcmeDomains:   acmeDomains,
		TLSListenerConfig: dynamiclistener.Config{
			TLSConfig:           tlsConfig,
			ExpirationDaysCheck: expiration,
		},
	}

	// ACME / Let's Encrypt
	// If --acme-domain is set, configure and return
	if len(acmeDomains) > 0 {
		return "", opts, nil
	}

	// Mounted certificates
	// If certificate file/key are set
	certFileExists := fileExists(rancherCertFile)
	keyFileExists := fileExists(rancherKeyFile)

	// If certificate file exists but not certificate key, or other way around, error out
	if (certFileExists && !keyFileExists) || (!certFileExists && keyFileExists) {
		return "", nil, fmt.Errorf("invalid SSL configuration found, please set both certificate file and certificate key file (one is missing)")
	}

	caFileExists := fileExists(rancherCACertsFile)

	// If certificate file and certificate key file exists, load files into listenConfig
	if certFileExists && keyFileExists {
		cert, err := tls.LoadX509KeyPair(rancherCertFile, rancherKeyFile)
		if err != nil {
			return "", nil, err
		}
		opts.TLSListenerConfig.TLSConfig.Certificates = []tls.Certificate{cert}

		// Selfsigned needs cacerts, recognized CA needs --no-cacerts but can't be used together
		if (caFileExists && noCACerts) || (!caFileExists && !noCACerts) {
			return "", nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		// Load cacerts if exists
		if caFileExists {
			ca, err = readPEM(rancherCACertsFile)
			if err != nil {
				return "", nil, err
			}
		}
		return ca, opts, nil
	}

	// External termination
	// We need to check if cacerts is passed or if --no-cacerts is used (when not providing certificate file and key)
	// If cacerts is passed
	if caFileExists {
		// We can't have --no-cacerts
		if noCACerts {
			return "", nil, fmt.Errorf("invalid SSL configuration found, please set cacerts when using self signed certificates or use --no-cacerts when using certificates from a recognized Certificate Authority, do not use both at the same time")
		}
		ca, err = readPEM(rancherCACertsFile)
		if err != nil {
			return "", nil, err
		}
	}

	// No certificates mounted or only --no-cacerts used
	return ca, opts, nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func readPEM(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
