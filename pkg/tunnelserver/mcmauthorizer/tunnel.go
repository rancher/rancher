package mcmauthorizer

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/kontainerdriver"
	"github.com/rancher/rancher/pkg/namespace"

	"github.com/rancher/norman/types/convert"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	crtKeyIndex  = "crtKeyIndex"
	nodeKeyIndex = "nodeKeyIndex"

	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

var (
	ErrClusterNotFound = errors.New("cluster not found")
	importDrivers      = map[string]bool{
		v32.ClusterDriverImported: true,
		v32.ClusterDriverK3s:      true,
		v32.ClusterDriverK3os:     true,
		v32.ClusterDriverRancherD: true,
		v32.ClusterDriverRke2:     true,
	}
)

type cluster struct {
	Address string `json:"address"`
	Token   string `json:"token"`
	CACert  string `json:"caCert"`
}

type input struct {
	Node        *client.Node `json:"node"`
	Cluster     *cluster     `json:"cluster"`
	NodeVersion int          `json:"nodeVersion"`
}

func NewAuthorizer(context *config.ScaledContext) *Authorizer {
	auth := &Authorizer{
		crtIndexer:            context.Management.ClusterRegistrationTokens("").Controller().Informer().GetIndexer(),
		clusterLister:         context.Management.Clusters("").Controller().Lister(),
		nodeIndexer:           context.Management.Nodes("").Controller().Informer().GetIndexer(),
		machineLister:         context.Management.Nodes("").Controller().Lister(),
		machines:              context.Management.Nodes(""),
		clusters:              context.Management.Clusters(""),
		KontainerDriverLister: context.Management.KontainerDrivers("").Controller().Lister(),
		Secrets:               context.Core.Secrets(""),
		SecretLister:          context.Core.Secrets("").Controller().Lister(),
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
	crtIndexer            cache.Indexer
	clusterLister         v3.ClusterLister
	nodeIndexer           cache.Indexer
	machineLister         v3.NodeLister
	machines              v3.NodeInterface
	clusters              v3.ClusterInterface
	KontainerDriverLister v3.KontainerDriverLister
	Secrets               corev1.SecretInterface
	SecretLister          corev1.SecretLister
}

type Client struct {
	Cluster     *v3.Cluster
	Node        *v3.Node
	Token       string
	Server      string
	NodeVersion int
}

func (t *Authorizer) AuthorizeTunnel(req *http.Request) (string, bool, error) {
	client, ok, err := t.Authorize(req)
	if client != nil && client.Node != nil {
		return client.Cluster.Name + ":" + client.Node.Name, ok, err
	} else if client != nil && client.Cluster != nil {
		return client.Cluster.Name, ok, err
	}

	return "", false, err
}

func (t *Authorizer) Authorize(req *http.Request) (*Client, bool, error) {
	token := req.Header.Get(Token)
	if token == "" {
		logrus.Debugf("Authorize: Token header [%s] is empty", Token)
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
		register := strings.HasSuffix(req.URL.Path, "/register")

		node, ok, err := t.authorizeNode(register, cluster, input.Node, req)
		if err != nil {
			return nil, false, err
		}
		if register && node.Status.NodeConfig != nil && input.Node.CustomConfig != nil {
			node = node.DeepCopy()
			node.Status.NodeConfig.Address = input.Node.CustomConfig.Address
			node.Status.NodeConfig.InternalAddress = input.Node.CustomConfig.InternalAddress
			node.Status.NodeConfig.Taints = taints.GetRKETaintsFromStrings(input.Node.CustomConfig.Taints)
		}
		return &Client{
			Cluster:     cluster,
			Node:        node,
			Token:       token,
			Server:      req.Host,
			NodeVersion: input.NodeVersion,
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
	logrus.Tracef("getMachine: looking up machine [%s] in cluster [%s]", machineName, cluster.Name)
	machine, err := t.machineLister.Get(cluster.Name, machineName)
	if apierrors.IsNotFound(err) {
		if objs, err := t.nodeIndexer.ByIndex(nodeKeyIndex, fmt.Sprintf("%s/%s", cluster.Name, inNode.RequestedHostname)); err == nil {
			for _, obj := range objs {
				return obj.(*v3.Node), err
			}
		}

		logrus.Tracef("getMachine: looking up [%s] as node name in cluster [%s]", inNode.RequestedHostname, cluster.Name)
		machine, err := t.machineLister.Get(cluster.Name, inNode.RequestedHostname)
		if err == nil {
			logrus.Debugf("Found [%s] as node name in cluster [%s], error: %v", inNode.RequestedHostname, cluster.Name, err)
			return machine, nil
		}

		logrus.Tracef("getMachine: looking up [%s] as RequestedHostname in cluster [%s]", inNode.RequestedHostname, cluster.Name)
		machines, _ := t.machineLister.List(cluster.Name, labels.NewSelector())
		for _, machine := range machines {
			logrus.Tracef("getMachine: comparing machine.Spec.RequestedHostname [%s] to inNode.RequestedHostname [%s]", machine.Spec.RequestedHostname, inNode.RequestedHostname)
			if machine.Spec.RequestedHostname == inNode.RequestedHostname {
				return machine, nil
			}
		}
	}

	return machine, err
}

func (t *Authorizer) authorizeNode(register bool, cluster *v3.Cluster, inNode *client.Node, req *http.Request) (*v3.Node, bool, error) {
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

	logrus.Tracef("updateDockerInfo: cluster [%s] node [%s] dockerInfo [%v]", cluster.Name, machine.Name, inNode.DockerInfo)
	machine, err = t.updateDockerInfo(machine, inNode)
	return machine, true, err
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
		Spec: v32.NodeSpec{
			Etcd:              inNode.Etcd,
			ControlPlane:      inNode.ControlPlane,
			Worker:            inNode.Worker,
			RequestedHostname: inNode.RequestedHostname,
			CustomConfig:      customConfig,
			Imported:          true,
		},
	}

	return t.machines.Create(machine)
}

func (t *Authorizer) updateDockerInfo(machine *v3.Node, inNode *client.Node) (*v3.Node, error) {
	if inNode.DockerInfo == nil {
		return machine, nil
	}

	dockerInfo := &v32.DockerInfo{}
	err := convert.ToObj(inNode.DockerInfo, dockerInfo)
	if err != nil {
		return nil, err
	}

	newMachine := machine.DeepCopy()
	newMachine.Status.DockerInfo = dockerInfo
	if !reflect.DeepEqual(machine, newMachine) {
		return t.machines.Update(newMachine)
	}
	return machine, nil
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

	if !importDrivers[cluster.Status.Driver] && cluster.Status.Driver != "" {
		return cluster, true, nil
	}

	changed := false

	if cluster.Status.Driver == "" {
		driver, err := kontainerdriver.GetDriver(cluster, t.KontainerDriverLister)
		if err != nil {
			return cluster, true, err
		}
		if driver == "" {
			logrus.Tracef("Setting the driver to imported for cluster %v %v", cluster.Name, cluster.Spec.DisplayName)
			cluster.Status.Driver = v32.ClusterDriverImported
			changed = true
		}
	}

	apiEndpoint := "https://" + inCluster.Address
	token := inCluster.Token
	caCert := inCluster.CACert

	var currentSecret *corev1.Secret
	migrator := secretmigrator.NewMigrator(t.SecretLister, t.Secrets)
	if importDrivers[cluster.Status.Driver] {
		currentSecret, _ := t.SecretLister.Get(namespace.GlobalNamespace, cluster.Status.ServiceAccountTokenSecret)
		if cluster.Status.APIEndpoint != apiEndpoint ||
			cluster.Status.CACert != caCert ||
			cluster.Status.ServiceAccountTokenSecret == "" ||
			tokenChanged(currentSecret, token) {
			secret, err := migrator.CreateOrUpdateServiceAccountTokenSecret("", token, cluster)
			if err != nil {
				return cluster, true, err
			}
			if currentSecret != nil && secret.GetResourceVersion() != currentSecret.GetResourceVersion() {
				logrus.Infof("updated service account token for cluster %s (%s)", cluster.Name, cluster.Spec.DisplayName)
			}
			cluster.Status.APIEndpoint = apiEndpoint
			cluster.Status.ServiceAccountTokenSecret = secret.Name
			cluster.Status.ServiceAccountToken = ""
			cluster.Status.CACert = caCert
			changed = true
		}
	}

	if changed {
		_, err = t.clusters.Update(cluster)
		if currentSecret != nil {
			migrator.CleanupKnownSecrets([]*corev1.Secret{currentSecret})
		}
	}

	return cluster, true, err
}

func tokenChanged(secret *corev1.Secret, token string) bool {
	return secret != nil && string(secret.Data[secretmigrator.SecretKey]) != token
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
	machineNameMD5 := fmt.Sprintf("m-%s", hex.EncodeToString(digest[:])[:12])
	logrus.Tracef("machineName: returning [%s] for node with RequestedHostname [%s]", machineNameMD5, machine.RequestedHostname)
	return machineNameMD5
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

	return nil, ErrClusterNotFound
}

func (t *Authorizer) crtIndex(obj interface{}) ([]string, error) {
	crt := obj.(*v3.ClusterRegistrationToken)
	if crt.Status.Token == "" {
		return nil, nil
	}
	return []string{crt.Status.Token}, nil
}

func (t *Authorizer) nodeIndex(obj interface{}) ([]string, error) {
	node := obj.(*v3.Node)
	return []string{fmt.Sprintf("%s/%s", node.Namespace, node.Status.NodeName)}, nil
}

func (t *Authorizer) toCustomConfig(machine *client.Node) *v32.CustomConfig {
	if machine == nil || machine.CustomConfig == nil {
		return nil
	}

	result := &v32.CustomConfig{}
	if err := convert.ToObj(machine.CustomConfig, result); err != nil {
		return nil
	}
	return result
}
