package rkenodeconfigserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/tunnelserver"
	cluster2 "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

var (
	b2Mount = "/mnt/sda1"
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

	var nodeConfig *rkeworker.NodeConfig
	if isNonWorkerOnly(client.Node.Status.NodeConfig.Role) {
		nodeConfig, err = n.nonWorkerConfig(req.Context(), client.Cluster, client.Node)
	} else {
		if client.Cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		nodeConfig, err = n.nodeConfig(req.Context(), client.Cluster, client.Node)
	}

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

func isNonWorkerOnly(role []string) bool {
	if slice.ContainsString(role, services.ETCDRole) ||
		slice.ContainsString(role, services.ControlRole) {
		return true
	}
	return false
}

func (n *RKENodeConfigServer) nonWorkerConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node) (*rkeworker.NodeConfig, error) {
	rkeConfig := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig
	if rkeConfig == nil {
		rkeConfig = &v3.RancherKubernetesEngineConfig{}
	}

	rkeConfig = rkeConfig.DeepCopy()
	rkeConfig.Nodes = []v3.RKEConfigNode{
		*node.Status.NodeConfig,
	}
	rkeConfig.Nodes[0].Role = []string{services.WorkerRole, services.ETCDRole, services.ControlRole}

	infos, err := librke.GetDockerInfo(node)
	if err != nil {
		return nil, err
	}

	plan, err := librke.New().GeneratePlan(ctx, rkeConfig, infos)
	if err != nil {
		return nil, err
	}

	nc := &rkeworker.NodeConfig{
		ClusterName: cluster.Name,
	}

	for _, tempNode := range plan.Nodes {
		if tempNode.Address == node.Status.NodeConfig.Address {
			b2d := strings.Contains(infos[tempNode.Address].OperatingSystem, cluster2.B2DOS)
			nc.Processes = augmentProcesses(tempNode.Processes, false, b2d)
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for non-worker %s", node.Status.NodeConfig.Address)
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

	infos, err := librke.GetDockerInfo(node)
	if err != nil {
		return nil, err
	}

	plan, err := librke.New().GeneratePlan(ctx, spec.RancherKubernetesEngineConfig, infos)
	if err != nil {
		return nil, err
	}

	nc := &rkeworker.NodeConfig{
		Certs:       certString,
		ClusterName: cluster.Name,
	}

	for _, tempNode := range plan.Nodes {
		if tempNode.Address == node.Status.NodeConfig.Address {
			b2d := strings.Contains(infos[tempNode.Address].OperatingSystem, cluster2.B2DOS)
			nc.Processes = augmentProcesses(tempNode.Processes, true, b2d)
			nc.Files = tempNode.Files
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for %s", node.Status.NodeConfig.Address)
}

func augmentProcesses(processes map[string]v3.Process, worker, b2d bool) map[string]v3.Process {
	var shared []string

	if b2d {
		shared = append(shared, b2Mount)
	}

	for _, process := range processes {
		for _, bind := range process.Binds {
			parts := strings.Split(bind, ":")
			if len(parts) > 2 && strings.Contains(parts[2], "shared") {
				shared = append(shared, parts[0])
			}
		}
	}

	if len(shared) > 0 {
		args := []string{"--", "share-root.sh"}
		args = append(args, shared...)

		processes["share-mnt"] = v3.Process{
			Name:          "share-mnt",
			Args:          args,
			Image:         image.Resolve(settings.AgentImage.Get()),
			Binds:         []string{"/var/run:/var/run"},
			NetworkMode:   "host",
			RestartPolicy: "always",
			PidMode:       "host",
			Privileged:    true,
		}
	}

	if worker {
		// not sure if we really need this anymore
		delete(processes, "etcd")
	} else {
		if p, ok := processes["share-mnt"]; ok {
			processes = map[string]v3.Process{
				"share-mnt": p,
			}
		} else {
			processes = nil
		}
	}

	for _, p := range processes {
		for i, bind := range p.Binds {
			parts := strings.Split(bind, ":")
			if len(parts) > 1 && parts[1] == "/etc/kubernetes" {
				parts[0] = parts[1]
				p.Binds[i] = strings.Join(parts, ":")
			}
		}
	}

	return processes
}
