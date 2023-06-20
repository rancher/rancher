package rkenodeconfigclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/rkenodeconfigserver"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
)

const (
	Params = "X-API-Tunnel-Params"
)

var (
	client = &http.Client{
		Timeout: 300 * time.Second,
	}

	nodeNotFoundRegexp    = regexp.MustCompile(`^node\.management\.cattle\.io.*not found$`)
	clusterNotFoundRegexp = regexp.MustCompile(`^cluster.*not found$`)
)

type ErrNodeOrClusterNotFound struct {
	msg        string
	occursType string
}

func (e *ErrNodeOrClusterNotFound) Error() string {
	return e.msg
}

func (e *ErrNodeOrClusterNotFound) ErrorOccursType() string {
	return e.occursType
}

func newErrNodeOrClusterNotFound(msg, occursType string) *ErrNodeOrClusterNotFound {
	return &ErrNodeOrClusterNotFound{
		msg,
		occursType,
	}
}

// ConfigClient executes a GET request against the rancher servers node-server API in an attempt to get the most recent node-config.
// It continues to do so until a node-config is returned within the response body, or the retry limit of 3 attempts is exceeded. Upon
// receiving a valid node-config this function inspects any kubelet serving certificates present on the host to determine if they need to be refreshed.
// If kubelet certificates need to be regenerated, a second GET request will be made so that the node config holds valid certificates. Once all kubelet certificates
// are deemed valid, ConfigClient will execute the node config, writing files and executing processes as directed. The passed
// url and header values are used when crafting the GET request, and the writeCertOnly parameter is used to denote if the agent should disregard
// all aspects of the received node-config except any delivered certificates. Upon a successful execution of the node config, this function
// will return a polling interval which should be used to query the rancher server for the next node-config and any encountered errors.
func ConfigClient(ctx context.Context, url string, header http.Header, writeCertOnly bool) (int, error) {
	// try a few more times because there is a delay after registering a new node
	nodeOrClusterNotFoundRetryLimit := 3
	interval := 120
	requestedRenewedCert := false
	for {
		nc, err := getConfig(client, url, header)
		if err != nil {
			if _, ok := err.(*ErrNodeOrClusterNotFound); ok {
				if nodeOrClusterNotFoundRetryLimit < 1 {
					// return the error if the node cannot connect to server or remove from a cluster
					return interval, err
				}

				nodeOrClusterNotFoundRetryLimit--
			}

			logrus.Warnf("Error while getting agent config: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if nc != nil {

			// if a cert file and key file are passed to the kubelet
			// then we need to ensure they are valid, otherwise we can safely skip the check
			certFile, keyFile := getKubeletCertificateFilesFromProcess(nc.Processes)
			if certFile != "" && keyFile != "" {
				logrus.Debugf("agent detected certificate arguments within kubelet process, checking kubelet certificate validity")
				// check to see if we need a new kubelet certificate
				kubeletCertNeedsRegen, err := kubeletNeedsNewCertificate(nc)
				if err != nil {
					return interval, err
				}

				if kubeletCertNeedsRegen && !requestedRenewedCert {
					// add to the  header and run getConfig again, so we get a new cert
					// we should only do this at most once per call to ConfigClient
					header.Set(rkenodeconfigserver.RegenerateKubeletCertificate, "true")
					logrus.Debugf("Requesting kubelet certificate regeneration")
					requestedRenewedCert = true
					continue
				}
			}

			header.Set(rkenodeconfigserver.RegenerateKubeletCertificate, "false")
			requestedRenewedCert = false

			// Logging at trace level as NodeConfig may contain sensitive data
			logrus.Tracef("Get agent config: %#v", nc)
			if nc.AgentCheckInterval != 0 {
				interval = nc.AgentCheckInterval
			}

			err = rkeworker.ExecutePlan(ctx, nc, writeCertOnly)
			if err != nil {
				return interval, err
			}

			/* server sends non-zero nodeVersion when node is upgrading (node.Status.AppliedVersion != cluster.Status.NodeVersion)
			ExecutePlan doesn't update processes if writeCertOnly, shouldn't consider this an upgrade */
			if nc.NodeVersion != 0 && !writeCertOnly {
				// reply back with nodeVersion
				params := node.Params()
				params["nodeVersion"] = nc.NodeVersion

				bytes, err := json.Marshal(params)
				if err != nil {
					return interval, err
				}

				headerCopy := http.Header{}
				for k, v := range header {
					headerCopy[k] = v
				}
				headerCopy[Params] = []string{base64.StdEncoding.EncodeToString(bytes)}
				header = headerCopy

				continue
			}

			return interval, err
		}

		logrus.Infof("Waiting for node to register. Either cluster is not ready for registering, cluster is currently provisioning, or etcd, controlplane and worker node have to be registered")
		time.Sleep(2 * time.Second)
	}
}

func getConfig(client *http.Client, url string, header http.Header) (*rkeworker.NodeConfig, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range header {
		req.Header[k] = v
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return &rkeworker.NodeConfig{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("invalid response %d: %s", resp.StatusCode, string(respBytes))

		if nodeNotFoundRegexp.Match(respBytes) {
			return nil, newErrNodeOrClusterNotFound(errMsg, "node")
		} else if clusterNotFoundRegexp.Match(respBytes) {
			return nil, newErrNodeOrClusterNotFound(errMsg, "cluster")
		}

		return nil, errors.New(errMsg)
	}

	nc := &rkeworker.NodeConfig{}
	return nc, json.NewDecoder(resp.Body).Decode(nc)
}

// getKubeletCertificateFilesFromProcess finds the tls-private-key-file and tls-cert-file values from
// the kubelet process so that they may be used to determine the validity of the kubelet certificates stored
// on the host.
func getKubeletCertificateFilesFromProcess(processes map[string]types.Process) (string, string) {
	proc, ok := processes["kubelet"]
	if !ok {
		return "", ""
	}

	return findCommandValue("--tls-private-key-file", proc.Command), findCommandValue("--tls-cert-file", proc.Command)
}

// findCommandValue iterates over a list of process flags and returns the value of
// the specified flag, stripping the flag name and the '=' sign. If the flag
// cannot be found in the list of flags, an empty string is returned.
func findCommandValue(flag string, commandsFlags []string) string {
	for _, cmd := range commandsFlags {
		if strings.HasPrefix(cmd, flag) {
			valueWithEqual := strings.TrimPrefix(cmd, flag)
			value := strings.TrimPrefix(valueWithEqual, "=")
			return value
		}
	}
	return ""
}

// kubeletNeedsNewCertificate will set the
// 'RegenerateKubeletCertificate' header field to true if
// a) the kubelet serving certificate does not exist
// b) the certificate will expire in 72 hours
// c) the certificate does not accurately represent the current IP address and Hostname of the node
//
// While the agent may denote it needs a new kubelet certificate
// in its connection request, a new certificate will only be
// delivered by Rancher if the generate_serving_certificate property
// is set to 'true' for the clusters kubelet service.
func kubeletNeedsNewCertificate(nc *rkeworker.NodeConfig) (bool, error) {
	kubeletCertKeyFile, kubeletCertFile := getKubeletCertificateFilesFromProcess(nc.Processes)
	if kubeletCertFile == "" || kubeletCertKeyFile == "" {
		return true, nil
	}

	cert, err := tls.LoadX509KeyPair(kubeletCertFile, kubeletCertKeyFile)
	if err != nil && !strings.Contains(err.Error(), "no such file") {
		return true, err
	}

	needsRegen, err := kubeletCertificateNeedsRegeneration(os.Getenv("CATTLE_ADDRESS"), os.Getenv("CATTLE_NODE_NAME"), cert, time.Now())
	if err != nil {
		return true, err
	}

	return needsRegen, nil
}
