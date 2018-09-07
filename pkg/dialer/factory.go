package dialer

import (
	"fmt"
	"net"
	"time"

	"net/url"
	"strings"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func NewFactory(apiContext *config.ScaledContext) (dialer.Factory, error) {
	authorizer := tunnelserver.NewAuthorizer(apiContext)
	tunneler, err := tunnelserver.NewTunnelServer(apiContext, authorizer)
	if err != nil {
		return nil, err
	}

	secretStore, err := nodeconfig.NewStore(apiContext.Core.Namespaces(""), apiContext.Core)
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

func (f *Factory) translateClusterAddress(cluster *v3.Cluster, clusterHostPort, address string) string {
	if clusterHostPort != address {
		return address
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}

	// Make sure that control plane node we are connecting to is not bad, also use internal address
	nodes, err := f.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return address
	}

	clusterGood := v3.ClusterConditionReady.IsTrue(cluster)
	lastGoodHost := ""
	for _, node := range nodes {
		var (
			publicIP  = node.Status.NodeAnnotations[k8s.ExternalAddressAnnotation]
			privateIP = node.Status.NodeAnnotations[k8s.InternalAddressAnnotation]
		)

		fakeNode := &v1.Node{
			Status: node.Status.InternalNodeStatus,
		}

		nodeGood := v3.NodeConditionRegistered.IsTrue(node) && v3.NodeConditionProvisioned.IsTrue(node) &&
			!v3.NodeConditionReady.IsUnknown(fakeNode) && node.DeletionTimestamp == nil

		if privateIP == "" || !nodeGood {
			continue
		}

		if publicIP == host {
			if clusterGood {
				host = privateIP
				return fmt.Sprintf("%s:%s", host, port)
			}
		} else if node.Status.NodeConfig != nil && slice.ContainsString(node.Status.NodeConfig.Role, services.ControlRole) {
			lastGoodHost = privateIP
		}
	}

	if lastGoodHost != "" {
		return fmt.Sprintf("%s:%s", lastGoodHost, port)
	}

	return address
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
		cd := f.TunnelServer.Dialer(cluster.Name, 15*time.Second)
		return func(network, address string) (net.Conn, error) {
			if cluster.Status.Driver == v3.ClusterDriverRKE {
				address = f.translateClusterAddress(cluster, hostPort, address)
			}
			return cd(network, address)
		}, nil
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

	sessionKey := machineSessionKey(machine)
	if f.TunnelServer.HasSession(sessionKey) {
		d := f.TunnelServer.Dialer(sessionKey, 15*time.Second)
		return func(string, string) (net.Conn, error) {
			return d("unix", "/var/run/docker.sock")
		}, nil
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

	sessionKey := machineSessionKey(machine)
	if f.TunnelServer.HasSession(sessionKey) {
		d := f.TunnelServer.Dialer(sessionKey, 15*time.Second)
		return dialer.Dialer(d), nil
	}

	return nil, fmt.Errorf("can not build dialer to %s:%s", clusterName, machineName)
}

func machineSessionKey(machine *v3.Node) string {
	return fmt.Sprintf("%s/%s", machine.Namespace, machine.Name)
}
