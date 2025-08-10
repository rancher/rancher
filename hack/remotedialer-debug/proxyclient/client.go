package proxyclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/remotedialer"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultServerAddr        = "wss://127.0.0.1"
	defaultServerPort        = 5555
	defaultServerPath        = "/connect"
	retryTimeout             = 1 * time.Second
	certificateWatchInterval = 10 * time.Second
	getSecretRetryTimeout    = 5 * time.Second
)

type PortForwarder interface {
	Start() error
	Stop()
}

type ProxyClientOpt func(*ProxyClient)

type ProxyClient struct {
	forwarder           PortForwarder
	serverUrl           string
	serverConnectSecret string

	dialer    *websocket.Dialer
	dialerMtx sync.Mutex

	secretController v1.SecretController
	namespace        string
	certSecretName   string
	certServerName   string

	onConnect func(ctx context.Context, session *remotedialer.Session) error
}

func New(ctx context.Context, serverSharedSecret, namespace, certSecretName, certServerName string, secretController v1.SecretController, forwarder PortForwarder, opts ...ProxyClientOpt) (*ProxyClient, error) {
	if secretController == nil {
		return nil, fmt.Errorf("SecretController required")
	}

	if forwarder == nil {
		return nil, fmt.Errorf("a PortForwarder must be provided")
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace required")
	}

	if certSecretName == "" {
		return nil, fmt.Errorf("certSecretName required")
	}

	if serverSharedSecret == "" {
		return nil, fmt.Errorf("server shared secret must be provided")
	}

	serverUrl := fmt.Sprintf("%s:%d%s", defaultServerAddr, defaultServerPort, defaultServerPath)

	client := &ProxyClient{
		serverUrl:           serverUrl,
		forwarder:           forwarder,
		serverConnectSecret: serverSharedSecret,
		certSecretName:      certSecretName,
		certServerName:      certServerName,
		namespace:           namespace,
	}

	client.setUpBuildDialerCallback(ctx, certSecretName, secretController)

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

func (c *ProxyClient) setUpBuildDialerCallback(ctx context.Context, certSecretName string, secretController v1.SecretController) {
	secretController.OnChange(ctx, certSecretName, func(_ string, newSecret *corev1.Secret) (*corev1.Secret, error) {
		if newSecret == nil {
			return nil, nil
		}

		if newSecret.Name == c.certSecretName && newSecret.Namespace == c.namespace {
			rootCAs, err := buildCertFromSecret(c.namespace, c.certSecretName, newSecret)
			if err != nil {
				logrus.Errorf("RDPClient: build certificate failed: %s", err.Error())
				return nil, err
			}

			c.dialerMtx.Lock()
			c.dialer = &websocket.Dialer{
				TLSClientConfig: &tls.Config{
					RootCAs:    rootCAs,
					ServerName: c.certServerName,
				},
			}
			c.dialerMtx.Unlock()
			logrus.Infof("RDPClient: certificate updated successfully")
		}

		return newSecret, nil
	})
}

func buildCertFromSecret(namespace, certSecretName string, secret *corev1.Secret) (*x509.CertPool, error) {
	crtData, exists := secret.Data["tls.crt"]
	if !exists {
		return nil, fmt.Errorf("secret %s/%s missing tls.crt field", namespace, certSecretName)
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(crtData); !ok {
		return nil, fmt.Errorf("failed to parse tls.crt from secret into a CA pool")
	}

	return rootCAs, nil
}

func (c *ProxyClient) Run(ctx context.Context) {
	go func() {
	LookForDialer:
		for {
			select {
			case <-ctx.Done():
				logrus.Infof("RDPClient: Received stop signal.")
				return

			default:
				logrus.Info("RDPClient: Checking if dialer is built...")

				c.dialerMtx.Lock()
				dialer := c.dialer
				c.dialerMtx.Unlock()

				if dialer != nil {
					logrus.Info("RDPClient: Dialer is built. Ready to start.")
					break LookForDialer
				}

				logrus.Infof("RDPClient: Dialer is not built yet, waiting %d secs to re-check.", getSecretRetryTimeout/time.Second)
				time.Sleep(getSecretRetryTimeout)
			}
		}

		for {
			select {
			case <-ctx.Done():
				logrus.Infof("RDPClient: Received signal to stop.")
				return

			default:
				if err := c.forwarder.Start(); err != nil {
					logrus.Errorf("RDPClient: %s ", err)
					time.Sleep(retryTimeout)
					continue
				}

				logrus.Infof("RDPClient: connecting to %s", c.serverUrl)

				headers := http.Header{}
				headers.Set("X-API-Tunnel-Secret", c.serverConnectSecret)

				onConnectAuth := func(proto, address string) bool { return true }
				onConnect := func(sessionCtx context.Context, session *remotedialer.Session) error {
					logrus.Infoln("RDPClient: remotedialer session connected!")
					if c.onConnect != nil {
						return c.onConnect(sessionCtx, session)
					}
					return nil
				}

				c.dialerMtx.Lock()
				dialer := c.dialer
				c.dialerMtx.Unlock()

				if err := remotedialer.ClientConnect(ctx, c.serverUrl, headers, dialer, onConnectAuth, onConnect); err != nil {
					logrus.Errorf("RDPClient: remotedialer.ClientConnect error: %s", err.Error())
					c.forwarder.Stop()
					time.Sleep(retryTimeout)
				}
			}
		}
	}()
}

func (c *ProxyClient) Stop() {
	if c.forwarder != nil {
		c.forwarder.Stop()
		logrus.Infoln("RDPClient: port-forward stopped.")
	}
}

func WithServerURL(serverUrl string) ProxyClientOpt {
	return func(pc *ProxyClient) {
		pc.serverUrl = serverUrl
	}
}

func WithOnConnectCallback(onConnect func(ctx context.Context, session *remotedialer.Session) error) ProxyClientOpt {
	return func(pc *ProxyClient) {
		pc.onConnect = onConnect
	}
}

func WithCustomDialer(dialer *websocket.Dialer) ProxyClientOpt {
	return func(pc *ProxyClient) {
		pc.dialer = dialer
	}
}
