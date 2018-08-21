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
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	normantypes "github.com/rancher/norman/types"
	"github.com/rancher/rancher/app"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/rancher/pkg/agent/node"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/rkenodeconfigclient"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	mngtclient "github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var (
	VERSION = "dev"
)

const (
	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

func main() {
	logserver.StartServerWithDefaults()
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
	defer c.Close()

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

		if strings.Contains(container.Names[0], "share-mnt") {
			continue
		}

		container := container
		go func() {
			time.Sleep(15 * time.Second)
			logrus.Infof("Removing unmanaged agent %s(%s)", container.Names[0], container.ID)
			c.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
				Force: true,
			})
		}()
	}

	return nil
}

func run() error {
	logrus.Infof("Rancher agent version %s is starting", VERSION)
	params, err := getParams()
	if err != nil {
		return err
	}
	writeCertsOnly := os.Getenv("CATTLE_WRITE_CERT_ONLY") == "true"
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

	onConnect := func(ctx context.Context) error {
		connected()
		connectConfig := fmt.Sprintf("https://%s/v3/connect/config", serverURL.Host)
		if err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, writeCertsOnly); err != nil {
			return err
		}

		if isCluster() {
			// if it is cluster agent
			managementConfigUrl := fmt.Sprintf("https://%s/v3/connect/managementConfig", serverURL.Host)
			managementConfig, err := cluster.GetManagementConfig(managementConfigUrl, token)
			if err != nil {
				return err
			}
			kubeConfig := rest.Config{}
			kubeConfig.Host = fmt.Sprintf("https://%s/k8s/clusters/local", serverURL.Host)
			kubeConfig.BearerToken = managementConfig.BearerToken
			kubeConfig.TLSClientConfig.CAFile = "/etc/kubernetes/ssl/certs/serverca"
			scaleContext, clusterManager, err := app.BuildScaledContext(ctx, kubeConfig, managementConfig.CfgConfig)
			if err != nil {
				return err
			}
			if err := setupCRDs(ctx, scaleContext.Schemas); err != nil {
				return err
			}
			// todo: figure out the minimum controllers that need to be running in agent. The reason we need to run resources is because we need interact with management resources in some of user controllers
			if err := scaleContext.Start(context.Background()); err != nil {
				logrus.Errorf("failed to start management controllers %s", err)
			}
			if err := cluster.StartUserController(context.Background(), clusterManager, managementConfig.Cluster); err != nil {
				panic(err)
			}
			return nil
		}

		if err := cleanup(context.Background()); err != nil {
			return err
		}

		go func() {
			logrus.Infof("Starting plan monitor")
			for {
				select {
				case <-time.After(2 * time.Minute):
					err := rkenodeconfigclient.ConfigClient(ctx, connectConfig, headers, writeCertsOnly)
					if err != nil {
						logrus.Errorf("failed to check plan: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()

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

func setupCRDs(ctx context.Context, schemas *normantypes.Schemas) error {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientGetter, err := proxy.NewClientGetterFromConfig(*restConfig)
	if err != nil {
		return err
	}
	factory := &crd.Factory{ClientGetter: clientGetter}
	factory.BatchCreateCRDs(ctx, config.UserStorageContext, schemas, &projectschema.Version,
		projectclient.AppType,
		projectclient.AppRevisionType,
		mngtclient.PipelineExecutionLogType,
		mngtclient.PipelineExecutionType,
		mngtclient.PipelineType,
		mngtclient.ProjectAlertType,
		mngtclient.ProjectLoggingType,
		mngtclient.ProjectNetworkPolicyType,
		mngtclient.ProjectRoleTemplateBindingType,
		mngtclient.SourceCodeCredentialType,
		mngtclient.SourceCodeRepositoryType,
	)
	factory.BatchWait()
	return nil
}
