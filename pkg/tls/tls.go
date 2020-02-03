package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/server"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	corev1controllers "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/data"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	rancherCertFile    = "/etc/rancher/ssl/cert.pem"
	rancherKeyFile     = "/etc/rancher/ssl/key.pem"
	rancherCACertsFile = "/etc/rancher/ssl/cacerts.pem"
)

func migrateCA(restConfig *rest.Config) (*core.Factory, error) {
	core, err := core.NewFactoryFromConfig(restConfig)
	if err != nil {
		return nil, err
	}

	if _, err := core.Core().V1().Secret().Get("cattle-system", "serving-ca", metav1.GetOptions{}); err == nil {
		return core, nil
	}

	dc, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	listenClient := dc.Resource(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "listenconfigs",
	})
	obj, err := listenClient.Get("cli-config", metav1.GetOptions{})
	if err != nil {
		return core, nil
	}

	caCert := data.Object(obj.Object).String("caCert")
	caKey := data.Object(obj.Object).String("caKey")

	if len(caCert) == 0 || len(caKey) == 0 {
		return core, nil
	}

	_, err = core.Core().V1().Secret().Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "serving-ca",
			Namespace: "cattle-system",
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       []byte(caCert),
			v1.TLSPrivateKeyKey: []byte(caKey),
		},
		StringData: nil,
		Type:       v1.SecretTypeTLS,
	})
	return core, err
}

func ListenAndServe(ctx context.Context, restConfig *rest.Config, handler http.Handler, httpsPort, httpPort int, acmeDomains []string, noCACerts bool) error {
	restConfig = rest.CopyConfig(restConfig)
	restConfig.Timeout = 10 * time.Minute

	core, err := migrateCA(restConfig)
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

func SetupListener(secrets corev1controllers.SecretController, acmeDomains []string, noCACerts bool) (*server.ListenOpts, error) {
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

	caForAgent = strings.TrimSpace(caForAgent)
	if settings.CACerts.Get() != caForAgent {
		if err := settings.CACerts.Set(caForAgent); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func readConfig(secrets corev1controllers.SecretController, acmeDomains []string, noCACerts bool) (string, *server.ListenOpts, error) {
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
