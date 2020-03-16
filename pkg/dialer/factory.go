package dialer

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/remotedialer"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/services"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

type nodeNotFound struct {
	error
}

func IsNodeNotFound(err error) bool {
	_, ok := err.(nodeNotFound)
	return ok
}

func NewFactory(apiContext *config.ScaledContext) (*Factory, error) {
	authorizer := tunnelserver.NewAuthorizer(apiContext)
	tunneler := tunnelserver.NewTunnelServer(authorizer)

	return &Factory{
		clusterLister:    apiContext.Management.Clusters("").Controller().Lister(),
		nodeLister:       apiContext.Management.Nodes("").Controller().Lister(),
		TunnelServer:     tunneler,
		TunnelAuthorizer: authorizer,
	}, nil
}

type Factory struct {
	nodeLister       v3.NodeLister
	clusterLister    v3.ClusterLister
	TunnelServer     *remotedialer.Server
	TunnelAuthorizer *tunnelserver.Authorizer
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
	return !cluster.Spec.Internal &&
		cluster.Status.Driver != v3.ClusterDriverImported &&
		cluster.Status.Driver != v3.ClusterDriverRKE &&
		cluster.Status.Driver != v3.ClusterDriverK3s

}

func (f *Factory) translateClusterAddress(cluster *v3.Cluster, clusterHostPort, address string) string {
	if clusterHostPort != address {
		logrus.Debugf("dialerFactory: apiEndpoint clusterHostPort [%s] is not equal to address [%s]", clusterHostPort, address)
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
	logrus.Debugf("dialerFactory: ClusterConditionReady for cluster [%s] is [%t]", cluster.Spec.DisplayName, clusterGood)
	lastGoodHost := ""
	logrus.Debug("dialerFactory: finding a node to tunnel the cluster connection")
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

		if !nodeGood {
			logrus.Debugf("dialerFactory: Skipping node [%s] for tunneling the cluster connection because nodeConditions are not as expected", node.Spec.RequestedHostname)
			logrus.Debugf("dialerFactory: Node conditions for node [%s]: %+v", node.Status.NodeName, node.Status.Conditions)
			continue
		}
		if privateIP == "" {
			logrus.Debugf("dialerFactory: Skipping node [%s] for tunneling the cluster connection because privateIP is empty", node.Status.NodeName)
			continue
		}

		logrus.Debugf("dialerFactory: IP addresses for node [%s]: publicIP [%s], privateIP [%s]", node.Status.NodeName, publicIP, privateIP)

		if publicIP == host {
			logrus.Debugf("dialerFactory: publicIP [%s] for node [%s] matches apiEndpoint host [%s], checking if cluster condition Ready is True", publicIP, node.Status.NodeName, host)
			if clusterGood {
				logrus.Debug("dialerFactory: cluster condition Ready is True")
				host = privateIP
				logrus.Debugf("dialerFactory: Using privateIP [%s] of node [%s] as node to tunnel the cluster connection", privateIP, node.Status.NodeName)
				return fmt.Sprintf("%s:%s", host, port)
			}
			logrus.Debug("dialerFactory: cluster condition Ready is False")
		} else if node.Status.NodeConfig != nil && slice.ContainsString(node.Status.NodeConfig.Role, services.ControlRole) {
			logrus.Debugf("dialerFactory: setting node [%s] with privateIP [%s] as option for the connection as it is a controlplane node", node.Status.NodeName, privateIP)
			lastGoodHost = privateIP
		}
	}

	if lastGoodHost != "" {
		logrus.Debugf("dialerFactory: returning [%s:%s] as last good option to tunnel the cluster connection", lastGoodHost, port)
		return fmt.Sprintf("%s:%s", lastGoodHost, port)
	}

	logrus.Debugf("dialerFactory: returning [%s], as no good option was found (no match with apiEndpoint or a controlplane node with correct conditions", address)
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
	logrus.Debugf("dialerFactory: apiEndpoint hostPort for cluster [%s] is [%s]", clusterName, hostPort)
	if (address == hostPort || isProxyAddress(address)) && isCloudDriver(cluster) {
		// For cloud drivers we just connect directly to the k8s API, not through the tunnel.  All other go through tunnel
		return native()
	}

	if f.TunnelServer.HasSession(cluster.Name) {
		logrus.Debugf("dialerFactory: tunnel session found for cluster [%s]", cluster.Name)
		cd := f.TunnelServer.Dialer(cluster.Name, 15*time.Second)
		return func(network, address string) (net.Conn, error) {
			if cluster.Status.Driver == v3.ClusterDriverRKE {
				address = f.translateClusterAddress(cluster, hostPort, address)
			}
			logrus.Debugf("dialerFactory: returning network [%s] and address [%s] as clusterDialer", network, address)
			return cd(network, address)
		}, nil
	}
	logrus.Debugf("dialerFactory: no tunnel session found for cluster [%s], falling back to nodeDialer", cluster.Name)

	// Try to connect to a node for the cluster dialer
	nodes, err := f.nodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	var localAPIEndpoint bool
	if cluster.Status.Driver == v3.ClusterDriverRKE {
		localAPIEndpoint = true
	}

	for _, node := range nodes {
		if node.DeletionTimestamp == nil && v3.NodeConditionProvisioned.IsTrue(node) {
			logrus.Debugf("dialerFactory: using node [%s]/[%s] for nodeDialer",
				node.Labels["management.cattle.io/nodename"], node.Name)
			if nodeDialer, err := f.nodeDialer(clusterName, node.Name); err == nil {
				return func(network, address string) (net.Conn, error) {
					if address == hostPort && localAPIEndpoint {
						logrus.Debug("dialerFactory: rewriting address/port to 127.0.0.1:6443 as node may not" +
							" have direct kube-api access")
						// The node dialer may not have direct access to kube-api so we hit localhost:6443 instead
						address = "127.0.0.1:6443"
					}
					logrus.Debugf("dialerFactory: Returning network [%s] and address [%s] as nodeDialer", network, address)
					return nodeDialer(network, address)
				}, nil
			}
		}
	}

	logrus.Debugf("No active connection for cluster [%s] will wait for about 30 seconds", cluster.Name)
	for i := 0; i < 4; i++ {
		if f.TunnelServer.HasSession(cluster.Name) {
			logrus.Debugf("Cluster [%s] has reconnected, resuming", cluster.Name)
			logrus.Debugf("dialerFactory: tunnel session found for cluster [%s]", cluster.Name)
			cd := f.TunnelServer.Dialer(cluster.Name, 15*time.Second)
			return func(network, address string) (net.Conn, error) {
				if cluster.Status.Driver == v3.ClusterDriverRKE {
					address = f.translateClusterAddress(cluster, hostPort, address)
				}
				logrus.Debugf("dialerFactory: returning network [%s] and address [%s] as clusterDialer", network, address)
				return cd(network, address)
			}, nil
		}
		time.Sleep(wait.Jitter(5*time.Second, 1))
	}

	return nil, fmt.Errorf("waiting for cluster [%s] agent to connect", cluster.Name)
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
	netDialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return netDialer.Dial, nil
}

func (f *Factory) DockerDialer(clusterName, machineName string) (dialer.Dialer, error) {
	machine, err := f.nodeLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	sessionKey := machineSessionKey(machine)
	if f.TunnelServer.HasSession(sessionKey) {
		network, address := "unix", "/var/run/docker.sock"
		if machine.Status.InternalNodeStatus.NodeInfo.OperatingSystem == "windows" {
			network, address = "npipe", "//./pipe/docker_engine"
		}
		d := f.TunnelServer.Dialer(sessionKey, 15*time.Second)
		return func(string, string) (net.Conn, error) {
			return d(network, address)
		}, nil
	}

	return nil, fmt.Errorf("can not build dialer to [%s:%s]", clusterName, machineName)
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

	return nil, nodeNotFound{error: fmt.Errorf("can not build dialer to [%s:%s]", clusterName, machineName)}
}

func machineSessionKey(machine *v3.Node) string {
	return fmt.Sprintf("%s:%s", machine.Namespace, machine.Name)
}

func isProxyAddress(address string) bool {
	proxy := getEnvAny("HTTP_PROXY", "http_proxy")
	if proxy == "" {
		proxy = getEnvAny("HTTPS_PROXY", "https_proxy")
	}

	if proxy == "" {
		return false
	}

	parsed, err := parseProxy(proxy)
	if err != nil {
		logrus.Warnf("Failed to parse http_proxy url %s: %v", proxy, err)
		return false
	}
	return parsed.Host == address
}

func getEnvAny(names ...string) string {
	for _, n := range names {
		if val := os.Getenv(n); val != "" {
			return val
		}
	}
	return ""
}

func parseProxy(proxy string) (*url.URL, error) {
	if proxy == "" {
		return nil, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil ||
		(proxyURL.Scheme != "http" &&
			proxyURL.Scheme != "https" &&
			proxyURL.Scheme != "socks5") {
		// proxy was bogus. Try pre-pending "http://" to it and
		// see if that parses correctly. If not, fall through
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}
	return proxyURL, nil
}
