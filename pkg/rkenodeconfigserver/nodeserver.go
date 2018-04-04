package rkenodeconfigserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

type RKENodeConfigServer struct {
	auth   *tunnelserver.Authorizer
	lookup *rkecerts.BundleLookup
}

func Handler(auth *tunnelserver.Authorizer, scaledContext *config.ScaledContext) http.Handler {
	return &RKENodeConfigServer{
		auth:   auth,
		lookup: rkecerts.NewLookup(scaledContext.Core.Namespaces(""), scaledContext.K8sClient.CoreV1()),
	}
}

func (n *RKENodeConfigServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// 404 tells the client to continue without plan
	// 5xx tells the client to try again later for plan

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
		return
	}

	if client.Cluster.Status.Driver != v3.ClusterDriverRKE {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if client.Node.Status.NodeConfig == nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if slice.ContainsString(client.Node.Status.NodeConfig.Role, services.ETCDRole) ||
		slice.ContainsString(client.Node.Status.NodeConfig.Role, services.ControlRole) {
		rw.WriteHeader(http.StatusNotFound)
		return
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
	spec := cluster.Status.AppliedSpec

	bundle, err := n.lookup.Lookup(cluster)
	if err != nil {
		return nil, err
	}

	bundle = bundle.ForNode(spec.RancherKubernetesEngineConfig, node.Status.NodeConfig.Address)

	certString, err := bundle.Marshal()
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
		if tempNode.Address == node.Status.NodeConfig.Address {
			nc.Processes = tempNode.Processes
			nc.Files = tempNode.Files
			delete(nc.Processes, "etcd")
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for %s", node.Status.NodeConfig.Address)
}
