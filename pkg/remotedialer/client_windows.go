package remotedialer

import (
	"context"
	"net/http"
	"strings"
	"time"

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

			logrus.Error("Failed to connect to proxy: ", errMsg)
		}

		return 500
	}

	return 200
}

func connectToProxyWhileWindows(rootContext context.Context, proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, blockingOnConnect func(context.Context) error) error {
	if dialer == nil {
		dialer = &websocket.Dialer{}
	}

	eg, ctx := errgroup.WithContext(rootContext)

	if blockingOnConnect != nil {
		eg.Go(func() error {
			return blockingOnConnect(ctx)
		})
	}

	eg.Go(func() error {
		reconnectCount := 0

		for {
			err := func() error {
				ws, _, err := dialer.Dial(proxyURL, headers)
				if err != nil {
					return err
				}
				defer ws.Close()

				session := newClientSession(auth, ws)
				_, err = session.serveWhileWindows(ctx)
				session.Close()
				return err
			}()
			if err != nil {
				if reconnectCount < 10 {
					errMsg := err.Error()
					if strings.HasSuffix(errMsg, "An existing connection was forcibly closed by the remote host.") ||
						strings.HasSuffix(errMsg, "An established connection was aborted by the software in your host machine.") ||
						strings.HasSuffix(errMsg, "A socket operation was attempted to an unreachable network.") {
						time.Sleep(5 * time.Second)

						reconnectCount += 1
						continue
					}
				}
			}

			return err
		}
	})

	return eg.Wait()
}
