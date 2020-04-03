package remotedialer

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type ConnectAuthorizer func(proto, address string) bool

func ClientConnect(ctx context.Context, wsURL string, headers http.Header, dialer *websocket.Dialer, auth ConnectAuthorizer, onConnect func(context.Context) error) {
	if err := connectToProxy(ctx, wsURL, headers, auth, dialer, onConnect); err != nil {
		logrus.WithError(err).Error("Remotedialer proxy error")
		time.Sleep(time.Duration(5) * time.Second)
	}
}

func connectToProxy(rootCtx context.Context, proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, onConnect func(context.Context) error) error {
	logrus.WithField("url", proxyURL).Info("Connecting to proxy")

	if dialer == nil {
		dialer = &websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: HandshakeTimeOut}
	}
	ws, resp, err := dialer.DialContext(rootCtx, proxyURL, headers)
	if err != nil {
		if resp == nil {
			logrus.WithError(err).Errorf("Failed to connect to proxy. Empty dialer response")
		} else {
			rb, err2 := ioutil.ReadAll(resp.Body)
			if err2 != nil {
				logrus.WithError(err).Errorf("Failed to connect to proxy. Response status: %v - %v. Couldn't read response body (err: %v)", resp.StatusCode, resp.Status, err2)
			} else {
				logrus.WithError(err).Errorf("Failed to connect to proxy. Response status: %v - %v. Response body: %s", resp.StatusCode, resp.Status, rb)
			}
		}
		return err
	}
	defer ws.Close()

	result := make(chan error, 2)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	if onConnect != nil {
		go func() {
			if err := onConnect(ctx); err != nil {
				result <- err
			}
		}()
	}

	session := NewClientSession(auth, ws)
	defer session.Close()

	go func() {
		_, err = session.Serve(ctx)
		result <- err
	}()

	select {
	case <-ctx.Done():
		logrus.WithField("url", proxyURL).WithField("err", ctx.Err()).Info("Proxy done")
		return nil
	case err := <-result:
		return err
	}
}
