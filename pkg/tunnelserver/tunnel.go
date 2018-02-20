package tunnelserver

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	crtKeyIndex    = "crtKeyIndex"
	importedDriver = "imported"

	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

type cluster struct {
	Address string `json:"address"`
	Token   string `json:"token"`
	CACert  string `json:"caCert"`
}

type input struct {
	Node    *client.Node `json:"node"`
	Cluster *cluster     `json:"cluster"`
}

func NewTunnelServer(context *config.ScaledContext) *remotedialer.Server {
	auth := newAuthorizer(context)
	ready := func() bool {
		return context.Leader
	}
	return remotedialer.New(auth, func(rw http.ResponseWriter, req *http.Request, code int, err error) {
		rw.WriteHeader(code)
		rw.Write([]byte(err.Error()))
	}, ready)
}

func newAuthorizer(context *config.ScaledContext) remotedialer.Authorizer {
	auth := &Authorizer{
		crtIndexer:    context.Management.ClusterRegistrationTokens("").Controller().Informer().GetIndexer(),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		machineLister: context.Management.Nodes("").Controller().Lister(),
		machines:      context.Management.Nodes(""),
		clusters:      context.Management.Clusters(""),
	}
	context.Management.ClusterRegistrationTokens("").Controller().Informer().AddIndexers(map[string]cache.IndexFunc{
		crtKeyIndex: auth.crtIndex,
	})
	return auth.authorize
}

type Authorizer struct {
	crtIndexer    cache.Indexer
	clusterLister v3.ClusterLister
	machineLister v3.NodeLister
	machines      v3.NodeInterface
	clusters      v3.ClusterInterface
}

func (t *Authorizer) authorize(req *http.Request) (string, bool, error) {
	token := req.Header.Get(Token)
	if token == "" {
		return "", false, nil
	}

	cluster, err := t.getClusterByToken(token)
	if err != nil || cluster == nil {
		return "", false, err
	}

	input, err := t.readInput(cluster, req)
	if err != nil {
		return "", false, err
	}

	if input.Node != nil {
		return t.authorizeNode(cluster, input.Node, req)
	}

	if input.Cluster != nil {
		return t.authorizeCluster(cluster, input.Cluster, req)
	}

	return "", false, nil
}

func (t *Authorizer) authorizeNode(cluster *v3.Cluster, inNode *client.Node, req *http.Request) (string, bool, error) {
	machineName := machineName(inNode)

	machine, err := t.machineLister.Get(cluster.Name, machineName)
	if apierrors.IsNotFound(err) {
		machine, err = t.createNode(inNode, cluster, req)
		if err != nil {
			return "", false, err
		}
	} else if err != nil && machine == nil {
		return "", false, err
	}

	return machine.Name, true, nil
}

func (t *Authorizer) createNode(inNode *client.Node, cluster *v3.Cluster, req *http.Request) (*v3.Node, error) {
	customConfig := t.toCustomConfig(inNode)
	if customConfig == nil {
		return nil, errors.New("invalid input, missing custom config")
	}

	if customConfig.Address == "" {
		return nil, errors.New("invalid input, address empty")
	}

	name := machineName(inNode)

	machine := &v3.Node{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Name,
		},
		Spec: v3.NodeSpec{
			Etcd:              inNode.Etcd,
			ControlPlane:      inNode.ControlPlane,
			Worker:            inNode.Worker,
			ClusterName:       cluster.Name,
			RequestedHostname: inNode.RequestedHostname,
			CustomConfig:      customConfig,
			Imported:          true,
		},
	}

	return t.machines.Create(machine)
}

func (t *Authorizer) authorizeCluster(cluster *v3.Cluster, inCluster *cluster, req *http.Request) (string, bool, error) {
	var (
		err error
	)

	if cluster.Status.Driver != importedDriver && cluster.Status.Driver != "" {
		return cluster.Name, true, nil
	}

	changed := false
	if cluster.Status.Driver == "" {
		cluster.Status.Driver = importedDriver
		changed = true
	}

	apiEndpoint := "https://" + inCluster.Address
	token := inCluster.Token
	caCert := inCluster.CACert

	if cluster.Status.APIEndpoint != apiEndpoint ||
		cluster.Status.ServiceAccountToken != token ||
		cluster.Status.CACert != caCert {
		cluster.Status.APIEndpoint = apiEndpoint
		cluster.Status.ServiceAccountToken = token
		cluster.Status.CACert = caCert
		changed = true
	}

	if changed {
		_, err = t.clusters.Update(cluster)
	}

	return cluster.Name, true, err
}

func (t *Authorizer) readInput(cluster *v3.Cluster, req *http.Request) (*input, error) {
	params := req.Header.Get(Params)
	var input input

	bytes, err := base64.StdEncoding.DecodeString(params)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, &input); err != nil {
		return nil, err
	}

	if input.Node == nil && input.Cluster == nil {
		return nil, errors.New("missing node or cluster registration info")
	}

	if input.Node != nil && input.Node.RequestedHostname == "" {
		return nil, errors.New("invalid input, hostname empty")
	}

	if input.Cluster != nil && input.Cluster.Address == "" {
		return nil, errors.New("invalid input, address empty")
	}

	if input.Cluster != nil && input.Cluster.Token == "" {
		return nil, errors.New("invalid input, token empty")
	}

	if input.Cluster != nil && input.Cluster.CACert == "" {
		return nil, errors.New("invalid input, caCert empty")
	}

	return &input, nil
}

func machineName(machine *client.Node) string {
	digest := md5.Sum([]byte(machine.RequestedHostname))
	return "m-" + hex.EncodeToString(digest[:])[:12]
}

func (t *Authorizer) getClusterByToken(token string) (*v3.Cluster, error) {
	keys, err := t.crtIndexer.ByIndex(crtKeyIndex, token)
	if err != nil {
		return nil, err
	}

	for _, obj := range keys {
		crt := obj.(*v3.ClusterRegistrationToken)
		return t.clusterLister.Get("", crt.Spec.ClusterName)
	}

	return nil, errors.New("cluster not found")
}

func (t *Authorizer) crtIndex(obj interface{}) ([]string, error) {
	crt := obj.(*v3.ClusterRegistrationToken)
	return []string{crt.Status.Token}, nil
}

func (t *Authorizer) toCustomConfig(machine *client.Node) *v3.CustomConfig {
	if machine == nil || machine.CustomConfig == nil {
		return nil
	}

	result := &v3.CustomConfig{}
	if err := convert.ToObj(machine.CustomConfig, result); err != nil {
		return nil
	}
	return result
}
