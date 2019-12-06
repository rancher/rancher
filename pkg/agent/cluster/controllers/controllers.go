package controllers

import (
	"context"
	"io/ioutil"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/rancher/rancher/pkg/auth/providers/common"

	"github.com/rancher/types/config/dialer"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

type controllerAccessor struct {
	apiPort    int
	kubeConfig *api.Config
}

func (c *controllerAccessor) ClusterDialer(clusterName string) (dialer.Dialer, error) {
	return net.Dial, nil
}

func (c *controllerAccessor) DockerDialer(clusterName, machineName string) (dialer.Dialer, error) {
	return net.Dial, nil
}

func (c *controllerAccessor) NodeDialer(clusterName, machineName string) (dialer.Dialer, error) {
	return net.Dial, nil
}

func (c *controllerAccessor) Stop(cluster *v3.Cluster) {
}

func (c *controllerAccessor) KubeConfig(clusterName, token string) *api.Config {
	return c.kubeConfig
}

func (c *controllerAccessor) GetHTTPSPort() int {
	return c.apiPort
}

func StartControllers(ctx context.Context, token, url, namespace string) error {
	cloudCfg, err := getCloudCfg(token, url)
	if err != nil {
		return err
	}

	clusterCfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	scaledContext, err := config.NewScaledContext(*cloudCfg)
	if err != nil {
		return err
	}
	accessor, err := newAccessor(cloudCfg.Host, clusterCfg)
	if err != nil {
		return err
	}

	scaledContext.Dialer = accessor
	scaledContext.UserManager, err = common.NewUserManager(scaledContext)

	userContext, err := config.NewUserContext(scaledContext, *clusterCfg, namespace)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	cluster, err := userContext.Management.Management.Clusters("").Get(namespace, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "getting cluster")
	}

	if err := user.Register(ctx, userContext, cluster, accessor, accessor); err != nil {
		return err
	}

	if err := scaledContext.Start(ctx); err != nil {
		return err
	}

	return userContext.Start(ctx)
}

func newAccessor(apiURL string, inCluster *rest.Config) (*controllerAccessor, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	port := u.Port()
	if port == "" {
		port = "443"
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	token, err := ioutil.ReadFile(inCluster.BearerTokenFile)
	if err != nil {
		return nil, err
	}

	return &controllerAccessor{
		apiPort: portNum,
		kubeConfig: &api.Config{
			CurrentContext: "default",
			APIVersion:     "v1",
			Kind:           "Config",
			Clusters: map[string]*api.Cluster{
				"default": {
					Server:               inCluster.Host,
					CertificateAuthority: rootCAFile,
				},
			},
			Contexts: map[string]*api.Context{
				"default": {
					AuthInfo: "user",
					Cluster:  "default",
				},
			},
			AuthInfos: map[string]*api.AuthInfo{
				"user": {
					Token: strings.TrimSpace(string(token)),
				},
			},
		},
	}, nil
}

func getCloudCfg(token, cloudURL string) (*rest.Config, error) {
	u, err := url.Parse(cloudURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/k8s/clusters/local"

	return &rest.Config{
		Host:        u.String(),
		BearerToken: token,
	}, nil
}
