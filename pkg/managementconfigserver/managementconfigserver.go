package managementconfigserver

import (
	"net/http"

	"encoding/json"

	"github.com/pkg/errors"
	apptypes "github.com/rancher/rancher/app/types"
	"github.com/rancher/rancher/pkg/agent/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

const (
	crtKeyIndex = "crtKeyIndex"
	Token       = "X-API-Tunnel-Token"
)

type ManagementConfigServer struct {
	ScaleContext  *config.ScaledContext
	crtIndexer    cache.Indexer
	clusterLister v3.ClusterLister
	cfg           *apptypes.Config
}

func Handler(scaleContext *config.ScaledContext, cfg *apptypes.Config) http.Handler {
	return &ManagementConfigServer{
		ScaleContext:  scaleContext,
		crtIndexer:    scaleContext.Management.ClusterRegistrationTokens("").Controller().Informer().GetIndexer(),
		clusterLister: scaleContext.Management.Clusters("").Controller().Lister(),
		cfg:           cfg,
	}
}

func (m *ManagementConfigServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mc, err := m.Authorize(req)
	if err != nil {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte(err.Error()))
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(mc); err != nil {
		logrus.Errorf("Failed to write managementConfig to agent: %v", err)
	}
}

func (m *ManagementConfigServer) Authorize(req *http.Request) (*cluster.ManagementConfig, error) {
	token := req.Header.Get(Token)
	if token == "" {
		return nil, errors.New("token is empty")
	}
	c, err := m.getClusterByToken(token)
	if err != nil || c == nil {
		return nil, err
	}
	mc := &cluster.ManagementConfig{}
	mc.Cluster = c
	mc.CfgConfig = m.cfg
	mc.RestConfig.Host = m.ScaleContext.RESTConfig.Host
	mc.RestConfig.TLSClientConfig = m.ScaleContext.RESTConfig.TLSClientConfig
	// todo: we are pushing cluster-admin privilege down to cluster. need to change that
	mc.RestConfig.BearerToken = m.ScaleContext.RESTConfig.BearerToken
	// todo: bearer token to talk to rancher api. Also need to configure privilege
	creatorID := c.Annotations["field.cattle.io/creatorId"]
	bearerToken, err := m.ScaleContext.UserManager.EnsureToken("cluster-agent-token", "token for cluster to talk to local k8s cluster api", creatorID)
	if err != nil {
		return nil, err
	}
	mc.BearerToken = bearerToken
	return mc, nil
}

func (m *ManagementConfigServer) getClusterByToken(token string) (*v3.Cluster, error) {
	keys, err := m.crtIndexer.ByIndex(crtKeyIndex, token)
	if err != nil {
		return nil, err
	}

	for _, obj := range keys {
		crt := obj.(*v3.ClusterRegistrationToken)
		return m.clusterLister.Get("", crt.Spec.ClusterName)
	}

	return nil, errors.New("cluster not found")
}
