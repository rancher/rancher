package dialer

import (
	"fmt"
	"net"
	"time"

	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/config/dialer"
	"k8s.io/apimachinery/pkg/labels"
)

func NewFactory(apiContext *config.ScaledContext, tunneler *remotedialer.Server) (dialer.Factory, error) {
	if tunneler == nil {
		tunneler = tunnelserver.NewTunnelServer(apiContext)
	}

	secretStore, err := nodeconfig.NewStore(apiContext.Core.Namespaces(""), apiContext.K8sClient.CoreV1())
	if err != nil {
		return nil, err
	}

	return &Factory{
		clusterLister: apiContext.Management.Clusters("").Controller().Lister(),
		nodeLister:    apiContext.Management.Nodes("").Controller().Lister(),
		TunnelServer:  tunneler,
		store:         secretStore,
	}, nil
}

type Factory struct {
	nodeLister    v3.NodeLister
	clusterLister v3.ClusterLister
	TunnelServer  *remotedialer.Server
	store         *encryptedstore.GenericEncryptedStore
}

func (f *Factory) ClusterDialer(clusterName string) (dialer.Dialer, error) {
	cluster, err := f.clusterLister.Get("", clusterName)
	if err != nil {
		return nil, err
	}

	if cluster.Status.Driver == v3.ClusterDriverImported && (cluster.Spec.ImportedConfig == nil || cluster.Spec.ImportedConfig.KubeConfig == "") {
		return f.TunnelServer.Dialer(cluster.Name, 15*time.Second), nil
	} else if cluster.Status.Driver == v3.ClusterDriverRKE {
		nodes, err := f.nodeLister.List(cluster.Name, labels.Everything())
		if err != nil {
			return nil, err
		}

		for _, node := range nodes {
			if node.Spec.Imported && node.DeletionTimestamp == nil && v3.NodeConditionProvisioned.IsTrue(node) {
				return f.DockerDialer(clusterName, node.Name)
			}
		}
	}

	return net.Dial, nil
}

func (f *Factory) DockerDialer(clusterName, machineName string) (dialer.Dialer, error) {
	machine, err := f.nodeLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if machine.Spec.Imported {
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

	return nil, fmt.Errorf("can not build dailer to %s:%s", clusterName, machineName)

}

func (f *Factory) NodeDialer(clusterName, machineName string) (dialer.Dialer, error) {
	machine, err := f.nodeLister.Get(clusterName, machineName)
	if err != nil {
		return nil, err
	}

	if machine.Spec.Imported {
		d := f.TunnelServer.Dialer(machine.Name, 15*time.Second)
		return dialer.Dialer(d), nil
	}

	if machine.Spec.CustomConfig != nil && machine.Spec.CustomConfig.Address != "" && machine.Spec.CustomConfig.SSHKey != "" {
		return f.sshLocalDialer(machine)
	}

	if machine.Spec.NodeTemplateName != "" {
		return f.sshLocalDialer(machine)
	}

	return nil, fmt.Errorf("can not build dailer to %s:%s", clusterName, machineName)
}
