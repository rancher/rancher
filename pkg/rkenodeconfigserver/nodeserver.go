package rkenodeconfigserver

import (
	"net/http"

	"encoding/json"

	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

type RKENodeConfigServer struct {
	auth *tunnelserver.Authorizer
}

func Handler(auth *tunnelserver.Authorizer) http.Handler {
	return &RKENodeConfigServer{
		auth: auth,
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

	if !client.Node.Spec.Imported || client.Cluster.Status.Driver != v3.ClusterDriverLocal {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	if client.Node.Status.NodeConfig == nil {
		rw.WriteHeader(http.StatusServiceUnavailable)
	}

	nodeConfig, err := AgentConfig(req.Context(), *client.Node.Status.NodeConfig, client.Server, client.Token)
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
