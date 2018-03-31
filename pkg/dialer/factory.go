package dialer

import (
	"fmt"
	"net"
	"time"

	"net/url"
	"strings"

	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func NewFactory(apiContext *config.ScaledContext) (dialer.Factory, error) {
	authorizer := tunnelserver.NewAuthorizer(apiContext)
	tunneler := tunnelserver.NewTunnelServer(apiContext, authorizer)

	secretStore, err := nodeconfig.NewStore(apiContext.Core.Namespaces(""), apiContext.K8sClient.CoreV1())
	if err != nil {
		return nil, err
	}

	apiContext.Management.Nodes("local").Controller().Informer().AddIndexers(cache.Indexers{
		nodeAccessIndexer: nodeIndexer,
	})

	return &Factory{
		clusterLister:       apiContext.Management.Clusters("").Controller().Lister(),
		localNodeController: apiContext.Management.Nodes("local").Controller(),
		nodeLister:          apiContext.Management.Nodes("").Controller().Lister(),
		TunnelServer:        tunneler,
		TunnelAuthorizer:    authorizer,
		store:               secretStore,
	}, nil
}

type Factory struct {
	localNodeController v3.NodeController
	nodeLister          v3.NodeLister
	clusterLister       v3.ClusterLister
	TunnelServer        *remotedialer.Server
	TunnelAuthorizer    *tunnelserver.Authorizer
	store               *encryptedstore.GenericEncryptedStore
}

func (f *Factory) ClusterDialer(clusterName string) (dialer.Dialer, error) {
	return func(network, address string) (net.Conn, error) {
		d, err := f.clusterDialer(clusterName, address)
		if err != nil {
			return nil, err
		}
		return d(network, address)
	}, nil
}

func isCloudDriver(cluster *v3.Cluster) bool {
	return !cluster.Spec.Internal && cluster.Status.Driver != v3.ClusterDriverImported && cluster.Status.Driver != v3.ClusterDriverRKE
}

func (f *Factory) clusterDialer(clusterName, address string) (dialer.Dialer, error) {
	cluster, err := f.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}

	if cluster.Spec.Internal {
		// For local (embedded, or import) we just assume we can connect directly
		return native()
	}

	hostPort := hostPort(cluster)
	if address == hostPort && isCloudDriver(cluster) {
		// For cloud drivers we just connect directly to the k8s API, not through the tunnel.  All other go through tunnel
		return native()
	}

	if f.TunnelServer.HasSession(cluster.Name) {
		return f.TunnelServer.Dialer(cluster.Name, 15*time.Second), nil
	}

	if cluster.Status.Driver != v3.ClusterDriverRKE {
		return nil, fmt.Errorf("waiting for cluster agent to connect")
	}

	// Only for RKE will we try to connect to a node for the cluster dialer
	nodes, err := f.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		if node.DeletionTimestamp == nil && v3.NodeConditionProvisioned.IsTrue(node) {
			if nodeDialer, err := f.nodeDialer(clusterName, node.Name); err == nil {
				return func(network, address string) (net.Conn, error) {
					if address == hostPort {
						// The node dialer may not have direct access to kube-api so we hit localhost:6443 instead
						address = "127.0.0.1:6443"
					}
					return nodeDialer(network, address)
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("waiting for cluster agent to connect")
}

func hostPort(cluster *v3.Cluster) string {
	u, err := url.Parse(cluster.Status.APIEndpoint)
	if err != nil {
		return ""
	}

	if strings.Contains(u.Host, ":") {
		return u.Host
	}
	return u.Host + ":443"
}

func native() (dialer.Dialer, error) {
	return func(network, address string) (net.Conn, error) {
		return net.DialTimeout(network, address, 30*time.Second)
	}, nil
}

func (f *Factory) DockerDialer(clusterName, machineName string) (dialer.Dialer, error) {
	machine, err := f.nodeLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if f.TunnelServer.HasSession(machine.Name) {
		d := f.TunnelServer.Dialer(machine.Name, 15*time.Second)
		return func(string, string) (net.Conn, error) {
			return d("unix", "/var/run/docker.sock")
		}, nil
	}

	if machine.Spec.CustomConfig != nil && machine.Spec.CustomConfig.Address != "" && machine.Spec.CustomConfig.SSHKey != "" {
		return f.sshDialer(machine)
	}

	if machine.Spec.NodeTemplateName != "" {
		return f.tlsDialer(machine)
	}

	return nil, fmt.Errorf("can not build dialer to %s:%s", clusterName, machineName)
}

func (f *Factory) NodeDialer(clusterName, machineName string) (dialer.Dialer, error) {
	return func(network, address string) (net.Conn, error) {
		d, err := f.nodeDialer(clusterName, machineName)
		if err != nil {
			return nil, err
		}
		return d(network, address)
	}, nil
}

func (f *Factory) nodeDialer(clusterName, machineName string) (dialer.Dialer, error) {
	machine, err := f.nodeLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if f.TunnelServer.HasSession(machine.Name) {
		d := f.TunnelServer.Dialer(machine.Name, 15*time.Second)
		return dialer.Dialer(d), nil
	}

	if machine.Spec.CustomConfig != nil && machine.Spec.CustomConfig.Address != "" && machine.Spec.CustomConfig.SSHKey != "" {
		return f.sshLocalDialer(machine)
	}

	if machine.Spec.NodeTemplateName != "" {
		return f.sshLocalDialer(machine)
	}

	return nil, fmt.Errorf("can not build dialer to %s:%s", clusterName, machineName)
}
