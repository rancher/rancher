package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	"github.com/sirupsen/logrus"
)

const (
	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

func main() {
	if os.Getenv("CATTLE_DEBUG") == "true" || os.Getenv("RANCHER_DEBUG") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func isCluster() bool {
	return os.Getenv("CATTLE_CLUSTER") == "true"
}

func getParams() (map[string]interface{}, error) {
	if isCluster() {
		return cluster.Params()
	}
	return node.Params(), nil
}

func getTokenAndURL() (string, string, error) {
	token, url, err := node.TokenAndURL()
	if err != nil {
		return "", "", err
	}
	if token == "" {
		return cluster.TokenAndURL()
	}
	return token, url, nil
}

func isConnect() bool {
	if os.Getenv("CATTLE_AGENT_CONNECT") == "true" {
		return true
	}
	_, err := os.Stat("connected")
	return err == nil
}

func connected() {
	f, err := os.Create("connected")
	if err != nil {
		f.Close()
	}
}

func cleanup(ctx context.Context) error {
	if os.Getenv("CATTLE_K8S_MANAGED") != "true" {
		return nil
	}

	c, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	args := filters.NewArgs()
	args.Add("label", "io.cattle.agent=true")

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return err
	}

	for _, container := range containers {
		if _, ok := container.Labels["io.kubernetes.pod.namespace"]; ok {
			continue
		}
		logrus.Infof("Removing unmanaged agent %s(%s)", container.Names[0], container.ID)
		err := c.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func run() error {
	ctx := signal.SigTermCancelContext(context.Background())

	params, err := getParams()
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	token, server, err := getTokenAndURL()
	if err != nil {
		return err
	}

	headers := map[string][]string{
		Token:  {token},
		Params: {base64.StdEncoding.EncodeToString(bytes)},
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return err
	}

	onConnect := func() error {
		connected()
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers)
		if err != nil {
			return err
		}
		if !isCluster() {
			return cleanup(context.Background())
		}
		return nil
	}

	for {
		wsURL := fmt.Sprintf("wss://%s/v3/connect", serverURL.Host)
		if !isConnect() {
			wsURL += "/register"
		}
		logrus.Infof("Connecting to %s with token %s", wsURL, token)
		remotedialer.ClientConnect(wsURL, http.Header(headers), nil, func(proto, address string) bool {
			switch proto {
			case "tcp":
				return true
			case "unix":
				return address == "/var/run/docker.sock"
			}
			return false
		}, onConnect)
		time.Sleep(5 * time.Second)
	}
}
