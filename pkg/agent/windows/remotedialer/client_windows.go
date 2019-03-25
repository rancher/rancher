package remotedialer

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rancher/norman/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type ConnectingStatus int

const (
	ConnectingStatusStopped ConnectingStatus = iota
	ConnectingStatusRetry
	ConnectionStatusLost
	ConnectingStatusFailed
)

func ClientConnectWhileWindows(ctx context.Context, wsURL string, headers http.Header, dialer *websocket.Dialer, auth remotedialer.ConnectAuthorizer, blockingOnConnect func(context.Context) error) ConnectingStatus {
	if err := connectToProxyWhileWindows(ctx, wsURL, headers, auth, dialer, blockingOnConnect); err != nil {
		if err == rkeworker.ErrHyperKubePSScriptAgentRetry {
			return ConnectingStatusRetry
		}

		if e, ok := err.(*rkenodeconfigclient.ErrNodeOrClusterNotFound); ok {
			logrus.Warnf("Proxy lost: it seems lost the registered %s: %v", e.ErrorOccursType(), e)
			return ConnectionStatusLost
		}

		logrus.Warnf("Proxy failed: %v", err)
		return ConnectingStatusFailed
	}

	logrus.Debugln("Proxy stopped")
	return ConnectingStatusStopped
}

func connectToProxyWhileWindows(rootContext context.Context, proxyURL string, headers http.Header, auth remotedialer.ConnectAuthorizer, dialer *websocket.Dialer, blockingOnConnect func(context.Context) error) error {
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
		connectRetryLimit := 10

		var err error
		for err == nil {
			err = func() error {
				ws, _, err := dialer.Dial(proxyURL, headers)
				if err != nil {
					return err
				}
				defer ws.Close()

				session := remotedialer.NewClientSession(auth, ws)
				_, err = session.ServeWhileWindows(ctx)
				session.Close()
				return err
			}()

			if err != nil {
				if connectRetryLimit < 1 {
					return errors.Wrap(err, "dialer retry timeout")
				}

				errMsg := err.Error()
				if err == websocket.ErrBadHandshake ||
					strings.HasSuffix(errMsg, "An existing connection was forcibly closed by the remote host.") ||
					strings.HasSuffix(errMsg, "An established connection was aborted by the software in your host machine.") ||
					strings.HasSuffix(errMsg, "A socket operation was attempted to an unreachable network.") {

					logrus.Debugf("Proxy dialer retry to %d times", connectRetryLimit)
					connectRetryLimit--
					time.Sleep(6 * time.Second)
					err = nil
				}
			}
		}

		return err
	})

	return eg.Wait()
}
