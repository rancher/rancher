package rkenodeconfigclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/rancher/rancher/pkg/agent/node"

	"github.com/rancher/rancher/pkg/rkeworker"
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

func ConfigClient(ctx context.Context, url string, header http.Header, writeCertOnly bool) (int, error) {
	// try a few more times because there is a delay after registering a new node
	nodeOrClusterNotFoundRetryLimit := 3
	interval := 120
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
			logrus.Debugf("Get agent config: %#v", nc)
			if nc.AgentCheckInterval != 0 {
				interval = nc.AgentCheckInterval
			}

			err := rkeworker.ExecutePlan(ctx, nc, writeCertOnly)
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

		logrus.Infof("Waiting for node to register. Either cluster is not ready for registering or etcd and controlplane node have to be registered first")
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

	if resp.StatusCode == http.StatusNoContent {
		return &rkeworker.NodeConfig{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := ioutil.ReadAll(resp.Body)
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
