package clustermanager

import (
	"context"
	"crypto/tls"
	"errors"
	"net"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/dialer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Reasons set on the cluster's Ready condition when its controllers can't be started or kept running.
const (
	reasonAPIServerUnreachable      = "APIServerUnreachable"
	reasonUserControllersFailed     = "UserControllersFailed"
	reasonUserControllersRestarting = "UserControllersRestarting"
)

var (
	// errAPIServerUnreachable marks a failure as the downstream API server not answering, regardless of
	// what the underlying error looks like once it has come back through the tunnel.
	errAPIServerUnreachable = errors.New("downstream API server is unreachable")
	// errControllersSyncTimeout is returned when the controllers didn't sync in time. The record cancels
	// itself in that case, so it has to be told apart from a record stopped by someone else.
	errControllersSyncTimeout = errors.New("timeout syncing controllers")
)

// IsClusterUnavailableErr checks if a given error indicates that the requested cluster was not available
func IsClusterUnavailableErr(err error) bool {
	if apiError, ok := err.(*httperror.APIError); ok {
		return apiError.Code == httperror.ClusterUnavailable
	}
	return false
}

// classifyStartError returns the Ready reason to report for err, a failure to start the controllers of
// a record whose context is ctx, and false if it should not be reported.
//
// A missing tunnel session is not reported: clusterconnected already sets Ready to False with reason
// Disconnected, and skips its own update only while that reason is in place, so a second writer with a
// different reason would make the two flip Ready back and forth.
func classifyStartError(ctx context.Context, err error) (string, bool) {
	switch {
	case errors.Is(err, dialer.ErrAgentDisconnected):
		return "", false
	case errors.Is(err, errControllersSyncTimeout):
		return reasonUserControllersFailed, true
	case errors.Is(err, errAPIServerUnreachable):
		return reasonAPIServerUnreachable, true
	case ctx.Err() != nil:
		// The record was stopped while starting, either because it is being replaced or because the
		// cluster no longer needs it.
		return reasonUserControllersRestarting, true
	case isAPIServerUnreachable(err):
		return reasonAPIServerUnreachable, true
	default:
		return reasonUserControllersFailed, true
	}
}

// isAPIServerUnreachable reports whether err looks like the downstream API server could not be reached
// or did not answer, as opposed to answering with an error.
func isAPIServerUnreachable(err error) bool {
	// A certificate that doesn't verify comes back as a net.Error too, but the server did answer.
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err)
}
