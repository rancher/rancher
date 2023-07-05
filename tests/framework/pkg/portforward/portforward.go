package portforward

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// ForwardPorts spawns a goroutine that does the equivalent of
// "kubectl port-forward -n <namespace> <podName> [portMapping]".
// The connection will remain open until stopChan is closed. Use errChan for receiving errors from the port-forward
// goroutine.
//
// Example:
//
//	stopCh := make(chan struct{}, 1)
//	errCh := make(chan error)
//	if err = ForwardPorts(conf, "my-ns", "my-pod", []string{"5000:5000"}, stopCh, errCh, time.Minute); err != nil {
//	    return err
//	}
//	defer func() {
//	    close(stopCh)
//	    close(errCh)
//	}()
func ForwardPorts(
	conf *rest.Config,
	namespace string,
	podName string,
	portMapping []string,
	stopChan <-chan struct{},
	errChan chan error,
	timeout time.Duration,
) error {
	transport, upgrader, err := spdy.RoundTripperFor(conf)
	if err != nil {
		return fmt.Errorf("error creating roundtripper: %w", err)
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{
			Scheme: "https",
			Path:   fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName),
			Host:   strings.TrimLeft(conf.Host, "htps:/"),
		},
	)

	// Create a new port-forwarder with localhost as the listen address. Standard output from the forwarder will be
	// discarded, but errors will go to stderr.
	readyChan := make(chan struct{})
	fw, err := portforward.New(dialer, portMapping, stopChan, readyChan, io.Discard, os.Stderr)
	if err != nil {
		return fmt.Errorf("error creating port-forwarder: %w", err)
	}

	// Start the port-forward
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			errChan <- err
		}
	}()

	// Wait for the port-forward to be ready for use before returning
	select {
	case <-readyChan:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %s waiting for port-forward to be ready", timeout)
	case err = <-errChan:
		return fmt.Errorf("error from port-forwarder: %w", err)
	}
}
