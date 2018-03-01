package remotedialer

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type ConnectAuthorizer func(proto, address string) bool

func ClientConnect(wsURL string, headers http.Header, tlsConfig *tls.Config, auth ConnectAuthorizer, onConnect func() error) {
	dialer := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}
	for {
		if err := connectToProxy(wsURL, headers, auth, dialer, onConnect); err != nil {
			logrus.WithError(err).Error("Failed to connect to proxy")
			time.Sleep(time.Duration(5) * time.Second)
		}
	}
}

func connectToProxy(proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, onConnect func() error) error {
	logrus.WithField("url", proxyURL).Info("Connecting to proxy")

	if dialer == nil {
		dialer = &websocket.Dialer{}
	}
	ws, _, err := dialer.Dial(proxyURL, headers)
	if err != nil {
		logrus.WithError(err).Error("Failed to connect to proxy")
		return err
	}

	if err := onConnect(); err != nil {
		return err
	}

	session := newClientSession(auth, ws)
	_, err = session.serve()
	session.Close()
	return err
}
