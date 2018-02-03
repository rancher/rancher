package tunnel

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
	crtKeyIndex = "crtKeyIndex"

	Token  = "X-API-Tunnel-Token"
	Params = "X-API-Tunnel-Params"
)

func NewTunneler(context *config.ManagementContext) *remotedialer.Server {
	auth := newAuthorizer(context)
	return remotedialer.New(auth, func(rw http.ResponseWriter, req *http.Request, code int, err error) {
		rw.WriteHeader(code)
		rw.Write([]byte(err.Error()))
	})

}

func newAuthorizer(context *config.ManagementContext) remotedialer.Authorizer {
	auth := &Authorizer{
		crtIndexer:    context.Management.ClusterRegistrationTokens("").Controller().Informer().GetIndexer(),
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		machineLister: context.Management.Machines("").Controller().Lister(),
		machines:      context.Management.Machines(""),
	}
	context.Management.ClusterRegistrationTokens("").Controller().Informer().AddIndexers(map[string]cache.IndexFunc{
		crtKeyIndex: auth.crtIndex,
	})
	return auth.authorize
}

type Authorizer struct {
	crtIndexer    cache.Indexer
	clusterLister v3.ClusterLister
	machineLister v3.MachineLister
	machines      v3.MachineInterface
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

	inMachine, err := t.readMachine(cluster, req)
	if err != nil {
		return "", false, err
	}

	machineName := machineName(inMachine)

	machine, err := t.machineLister.Get(cluster.Name, machineName)
	if apierrors.IsNotFound(err) {
		machine, err = t.createMachine(inMachine, cluster, req)
		if err != nil {
			return "", false, err
		}
	} else if err != nil && machine == nil {
		return "", false, err
	}

	return machine.Name, true, nil
}

func (t *Authorizer) createMachine(inMachine *client.Machine, cluster *v3.Cluster, req *http.Request) (*v3.Machine, error) {
	customConfig := t.toCustomConfig(inMachine)
	if customConfig == nil {
		return nil, errors.New("invalid input, missing custom config")
	}

	if customConfig.Address == "" {
		return nil, errors.New("invalid input, address empty")
	}

	name := machineName(inMachine)

	machine := &v3.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Name,
		},
		Spec: v3.MachineSpec{
			ClusterName:       cluster.Name,
			RequestedHostname: inMachine.RequestedHostname,
			Role:              inMachine.Role,
			CustomConfig:      customConfig,
			Imported:          true,
		},
	}

	return t.machines.Create(machine)
}

func (t *Authorizer) readMachine(cluster *v3.Cluster, req *http.Request) (*client.Machine, error) {
	params := req.Header.Get(Params)
	var inMachine client.Machine

	bytes, err := base64.StdEncoding.DecodeString(params)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, &inMachine); err != nil {
		return nil, err
	}

	if inMachine.RequestedHostname == "" {
		return nil, errors.New("invalid input, hostname empty")
	}

	return &inMachine, nil
}

func machineName(machine *client.Machine) string {
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

func (t *Authorizer) toCustomConfig(machine *client.Machine) *v3.CustomConfig {
	if machine == nil || machine.CustomConfig == nil {
		return nil
	}

	result := &v3.CustomConfig{}
	if err := convert.ToObj(machine.CustomConfig, result); err != nil {
		return nil
	}
	return result
}
