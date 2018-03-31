package rkenodeconfigserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/clusteryaml"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

type RKENodeConfigServer struct {
	auth    *tunnelserver.Authorizer
	builder *clusteryaml.Builder
}

func Handler(auth *tunnelserver.Authorizer, scaledContext *config.ScaledContext) http.Handler {
	return &RKENodeConfigServer{
		auth: auth,
		builder: clusteryaml.NewBuilder(scaledContext.Dialer,
			scaledContext.Management.Nodes("").Controller().Lister(),
			scaledContext.K8sClient.CoreV1()),
	}
}

func (n *RKENodeConfigServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	client, ok, err := n.auth.Authorize(req)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}

	if !ok {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if client.Node == nil {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if client.Cluster.Status.Driver == "" {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}

	if client.Cluster.Status.Driver != v3.ClusterDriverLocal && client.Cluster.Status.Driver != v3.ClusterDriverRKE {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if client.Node.Status.NodeConfig == nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}

	nodeConfig, err := n.nodeConfig(req.Context(), client.Cluster, client.Node)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(rw).Encode(nodeConfig); err != nil {
		logrus.Errorf("failed to write nodeConfig to agent: %v", err)
	}
}

func (n *RKENodeConfigServer) nodeConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node) (*rkeworker.NodeConfig, error) {
	spec, err := n.builder.GetSpec(cluster, false)
	if err != nil {
		return nil, err
	}

	rkeCluster, err := n.builder.ParseCluster(cluster.Name, spec)
	if err != nil {
		return nil, err
	}

	var rkeNode *v3.RKEConfigNode
	for i, nodeToCheck := range rkeCluster.Nodes {
		_, nodeNodeName := ref.Parse(nodeToCheck.NodeName)
		if nodeNodeName == node.Name {
			rkeNode = &rkeCluster.Nodes[i]
			break
		}
	}

	if rkeNode == nil {
		return nil, fmt.Errorf("failed to find valid node for %s", node.Name)
	}

	bundle, err := n.builder.GetOrGenerateWithNode(cluster, rkeNode)
	if err != nil {
		return nil, err
	}

	nodeCerts := bundle.ForNode(spec.RancherKubernetesEngineConfig, rkeNode)
	certString, err := nodeCerts.Marshal()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshall bundle")
	}

	plan, err := librke.New().GeneratePlan(ctx, spec.RancherKubernetesEngineConfig)
	if err != nil {
		return nil, err
	}

	nc := &rkeworker.NodeConfig{
		Certs:       certString,
		ClusterName: cluster.Name,
	}

	for _, tempNode := range plan.Nodes {
		if tempNode.Address == rkeNode.Address {
			nc.Processes = tempNode.Processes
			delete(nc.Processes, "etcd")
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for %s", rkeNode.Address)
}
