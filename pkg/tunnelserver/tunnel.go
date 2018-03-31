package tunnelserver

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"strings"

	"fmt"

	"reflect"

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
	nodeKeyIndex   = "nodeKeyIndex"
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

func NewTunnelServer(context *config.ScaledContext, authorizer *Authorizer) *remotedialer.Server {
	ready := func() bool {
		return context.Leader
	}
	return remotedialer.New(authorizer.authorizeTunnel, func(rw http.ResponseWriter, req *http.Request, code int, err error) {
		rw.WriteHeader(code)
		rw.Write([]byte(err.Error()))
	}, ready)
}

func NewAuthorizer(context *config.ScaledContext) *Authorizer {
	auth := &Authorizer{
		crtIndexer:    context.Management.ClusterRegistrationTokens("").Controller().Informer().GetIndexer(),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		nodeIndexer:   context.Management.Nodes("").Controller().Informer().GetIndexer(),
		machineLister: context.Management.Nodes("").Controller().Lister(),
		machines:      context.Management.Nodes(""),
		clusters:      context.Management.Clusters(""),
	}
	context.Management.ClusterRegistrationTokens("").Controller().Informer().AddIndexers(map[string]cache.IndexFunc{
		crtKeyIndex: auth.crtIndex,
	})
	context.Management.Nodes("").Controller().Informer().AddIndexers(map[string]cache.IndexFunc{
		nodeKeyIndex: auth.nodeIndex,
	})
	return auth
}

type Authorizer struct {
	crtIndexer    cache.Indexer
	clusterLister v3.ClusterLister
	nodeIndexer   cache.Indexer
	machineLister v3.NodeLister
	machines      v3.NodeInterface
	clusters      v3.ClusterInterface
}

type Client struct {
	Cluster *v3.Cluster
	Node    *v3.Node
	Token   string
	Server  string
}

func (t *Authorizer) authorizeTunnel(req *http.Request) (string, bool, error) {
	client, ok, err := t.Authorize(req)
	if client != nil && client.Node != nil {
		return client.Node.Name, ok, err
	} else if client != nil && client.Cluster != nil {
		return client.Cluster.Name, ok, err
	}

	return "", false, err
}

func (t *Authorizer) AuthorizeLocalNode(username, password string) (string, []string, *v3.Cluster, bool) {
	cluster, err := t.getClusterByToken(password)
	if err != nil || cluster == nil || cluster.Status.Driver != v3.ClusterDriverLocal {
		return "", nil, nil, false
	}
	if username == "kube-proxy" {
		return "system:kube-proxy", nil, cluster, true
	}
	return "system:node:" + username, []string{"system:nodes"}, cluster, true
}

func (t *Authorizer) Authorize(req *http.Request) (*Client, bool, error) {
	token := req.Header.Get(Token)
	if token == "" {
		return nil, false, nil
	}

	cluster, err := t.getClusterByToken(token)
	if err != nil || cluster == nil {
		return nil, false, err
	}

	input, err := t.readInput(cluster, req)
	if err != nil {
		return nil, false, err
	}

	if input.Node != nil {
		node, ok, err := t.authorizeNode(cluster, input.Node, req)
		if err != nil {
			return nil, false, err
		}
		if node.Status.NodeConfig != nil && input.Node.CustomConfig != nil {
			node = node.DeepCopy()
			node.Status.NodeConfig.Address = input.Node.CustomConfig.Address
			node.Status.NodeConfig.InternalAddress = input.Node.CustomConfig.InternalAddress
		}
		return &Client{
			Cluster: cluster,
			Node:    node,
			Token:   token,
			Server:  req.Host,
		}, ok, err
	}

	if input.Cluster != nil {
		cluster, ok, err := t.authorizeCluster(cluster, input.Cluster, req)
		return &Client{
			Cluster: cluster,
			Token:   token,
			Server:  req.Host,
		}, ok, err
	}

	return nil, false, nil
}

func (t *Authorizer) getMachine(cluster *v3.Cluster, inNode *client.Node) (*v3.Node, error) {
	machineName := machineName(inNode)

	machine, err := t.machineLister.Get(cluster.Name, machineName)
	if apierrors.IsNotFound(err) {
		if objs, err := t.nodeIndexer.ByIndex(nodeKeyIndex, fmt.Sprintf("%s/%s", cluster.Name, inNode.RequestedHostname)); err == nil {
			for _, obj := range objs {
				return obj.(*v3.Node), err
			}
		}

		machine, err := t.machineLister.Get(cluster.Name, inNode.RequestedHostname)
		if err == nil {
			return machine, nil
		}
	}

	return machine, err
}

func (t *Authorizer) authorizeNode(cluster *v3.Cluster, inNode *client.Node, req *http.Request) (*v3.Node, bool, error) {
	register := strings.HasSuffix(req.URL.Path, "/register")

	machine, err := t.getMachine(cluster, inNode)
	if apierrors.IsNotFound(err) {
		if !register {
			return nil, false, err
		}
		machine, err = t.createNode(inNode, cluster, req)
		if err != nil {
			return nil, false, err
		}
	} else if err != nil && machine == nil {
		return nil, false, err
	}

	if register {
		machine, err = t.updateNode(machine, inNode, cluster)
		if err != nil {
			return nil, false, err
		}
	}

	return machine, true, nil
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

func (t *Authorizer) updateNode(machine *v3.Node, inNode *client.Node, cluster *v3.Cluster) (*v3.Node, error) {
	newMachine := machine.DeepCopy()
	newMachine.Spec.Etcd = inNode.Etcd
	newMachine.Spec.ControlPlane = inNode.ControlPlane
	newMachine.Spec.Worker = inNode.Worker
	if !reflect.DeepEqual(machine, newMachine) {
		return t.machines.Update(newMachine)
	}
	return machine, nil
}

func (t *Authorizer) authorizeCluster(cluster *v3.Cluster, inCluster *cluster, req *http.Request) (*v3.Cluster, bool, error) {
	var (
		err error
	)

	if cluster.Status.Driver != importedDriver && cluster.Status.Driver != "" {
		return cluster, true, nil
	}

	changed := false
	if cluster.Status.Driver == "" {
		cluster.Status.Driver = importedDriver
		changed = true
	}

	apiEndpoint := "https://" + inCluster.Address
	token := inCluster.Token
	caCert := inCluster.CACert

	if cluster.Status.Driver == importedDriver {
		if cluster.Status.APIEndpoint != apiEndpoint ||
			cluster.Status.ServiceAccountToken != token ||
			cluster.Status.CACert != caCert {
			cluster.Status.APIEndpoint = apiEndpoint
			cluster.Status.ServiceAccountToken = token
			cluster.Status.CACert = caCert
			changed = true
		}
	}

	if changed {
		_, err = t.clusters.Update(cluster)
	}

	return cluster, true, err
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

func (t *Authorizer) nodeIndex(obj interface{}) ([]string, error) {
	node := obj.(*v3.Node)
	return []string{fmt.Sprintf("%s/%s", node.Namespace, node.Status.NodeName)}, nil
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
