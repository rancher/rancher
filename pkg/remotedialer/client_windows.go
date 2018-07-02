package remotedialer

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func ClientConnectWhileWindows(ctx context.Context, wsURL string, headers http.Header, dialer *websocket.Dialer, auth ConnectAuthorizer, blockingOnConnect func(context.Context) error) int64 {
	if err := connectToProxyWhileWindows(ctx, wsURL, headers, auth, dialer, blockingOnConnect); err != nil {
		errMsg := err.Error()

		switch err {
		case websocket.ErrBadHandshake:
			return 403
		case rkeworker.ErrHyperKubePSScriptAgentRetry:
			logrus.Warn("This connection try to touch proxy again: ", errMsg)
			return 302
		default:
			if e, ok := err.(*rkenodeconfigclient.ErrNodeOrClusterNotFound); ok {
				logrus.Warn("Can't connect to the registered " + e.ErrorOccursType() + ", terminating gracefully")
				return 503
			}

			if strings.HasSuffix(errMsg, "An existing connection was forcibly closed by the remote host.") {
				logrus.Warn("Proxy actively close this connection: ", errMsg)
			} else {
				logrus.Error("Failed to connect to proxy: ", errMsg)
			}
		}

		return 500
	}

	return 200
}

func connectToProxyWhileWindows(rootContext context.Context, proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, blockingOnConnect func(context.Context) error) error {
	if dialer == nil {
		dialer = &websocket.Dialer{}
	}

	ws, _, err := dialer.Dial(proxyURL, headers)
	if err != nil {
		return err
	}
	defer ws.Close()

	eg, ctx := errgroup.WithContext(rootContext)

	if blockingOnConnect != nil {
		eg.Go(func() error {
			return blockingOnConnect(ctx)
		})
	}

	eg.Go(func() error {
		session := newClientSession(auth, ws)
		defer session.Close()

		_, err := session.serveWhileWindows(ctx)
		if err != nil {
			return err
		}

		return nil
	})

	return eg.Wait()
}
