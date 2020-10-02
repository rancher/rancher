package rkenodeconfigserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/api/customization/clusterregistrationtokens"
	util "github.com/rancher/rancher/pkg/cluster"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/librke"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/taints"
	"github.com/rancher/rancher/pkg/tunnelserver"
	rkepki "github.com/rancher/rke/pki"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	DefaultAgentCheckInterval       = 120
	AgentCheckIntervalDuringUpgrade = 35
	AgentCheckIntervalDuringCreate  = 15
)

var (
	b2Mount = "/mnt/sda1"
)

type RKENodeConfigServer struct {
	auth                 *tunnelserver.Authorizer
	lookup               *BundleLookup
	systemAccountManager *systemaccount.Manager
	serviceOptionsLister v3.RKEK8sServiceOptionLister
	serviceOptions       v3.RKEK8sServiceOptionInterface
	sysImagesLister      v3.RKEK8sSystemImageLister
	sysImages            v3.RKEK8sSystemImageInterface
	nodes                v3.NodeInterface
}

func Handler(auth *tunnelserver.Authorizer, scaledContext *config.ScaledContext) http.Handler {
	return &RKENodeConfigServer{
		auth:                 auth,
		lookup:               NewLookup(scaledContext.Core.Namespaces(""), scaledContext.Core),
		systemAccountManager: systemaccount.NewManagerFromScale(scaledContext),
		serviceOptionsLister: scaledContext.Management.RKEK8sServiceOptions("").Controller().Lister(),
		serviceOptions:       scaledContext.Management.RKEK8sServiceOptions(""),
		sysImagesLister:      scaledContext.Management.RKEK8sSystemImages("").Controller().Lister(),
		sysImages:            scaledContext.Management.RKEK8sSystemImages(""),
		nodes:                scaledContext.Management.Nodes(""),
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
	if nodehelper.IsNonWorker(client.Node.Status.NodeConfig) {
		nodeConfig, err = n.nonWorkerConfig(req.Context(), client.Cluster, client.Node)
	} else {
		if client.Cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if client.NodeVersion != 0 {
			logrus.Debugf("cluster [%s] worker-upgrade: received node-version [%v] for node [%s]", client.Cluster.Name,
				client.NodeVersion, client.Node.Name)

			if client.Node.Status.AppliedNodeVersion != client.NodeVersion {
				nodeCopy := client.Node.DeepCopy()
				logrus.Infof("cluster [%s] worker-upgrade: updating node [%s] with node-version %v", client.Cluster.Name,
					client.Node.Name, client.NodeVersion)

				nodeCopy.Status.AppliedNodeVersion = client.NodeVersion

				_, err = n.nodes.Update(nodeCopy)
				if err != nil {
					logrus.Infof("cluster [%s] worker-upgrade: error updating node [%s] with node-version [%v]: %v", client.Cluster.Name,
						client.Node.Name, client.NodeVersion, err)
				}
			}
		}

		nodeConfig, err = n.nodeConfig(req.Context(), client.Cluster, client.Node)
	}

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Failed to construct node config. Error: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(rw).Encode(nodeConfig); err != nil {
		logrus.Errorf("failed to write nodeConfig to agent: %v", err)
	}
}

func (n *RKENodeConfigServer) nonWorkerConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node) (*rkeworker.NodeConfig, error) {
	nodePlan := node.Status.NodePlan
	nc := &rkeworker.NodeConfig{
		ClusterName: cluster.Name,
	}

	if nodePlan == nil {
		logrus.Tracef("cluster [%s]: node [%s] doesn't have node plan yet", cluster.Name, node.Name)
		nc.AgentCheckInterval = AgentCheckIntervalDuringCreate
		return nc, nil
	}

	nc.Processes = nodePlan.Plan.Processes
	nc.AgentCheckInterval = nodePlan.AgentCheckInterval

	return nc, nil
}

func (n *RKENodeConfigServer) nodeConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node) (*rkeworker.NodeConfig, error) {
	status := cluster.Status.AppliedSpec.DeepCopy()
	rkeConfig := status.RancherKubernetesEngineConfig

	nodePlan := node.Status.NodePlan
	hostAddress := node.Status.NodeConfig.Address

	nc := &rkeworker.NodeConfig{
		ClusterName: cluster.Name,
	}

	if nodePlan == nil {
		logrus.Tracef("cluster [%s]: node [%s] %s doesn't have node plan yet", cluster.Name, node.Name, hostAddress)
		nc.AgentCheckInterval = AgentCheckIntervalDuringCreate
		return nc, nil
	}

	infos, err := librke.GetDockerInfo(node)
	if err != nil {
		return nil, err
	}

	bundle, err := n.lookup.Lookup(cluster)
	if err != nil {
		return nil, err
	}

	hostDockerInfo := infos[hostAddress]
	if hostDockerInfo.OSType == "windows" { // compatible with Windows
		bundle = bundle.ForWindowsNode(rkeConfig, hostAddress)
	} else {
		bundle = bundle.ForNode(rkeConfig, hostAddress)
	}

	if rkepki.IsKubeletGenerateServingCertificateEnabledinConfig(rkeConfig) {
		logrus.Debugf("nodeConfig: VerifyKubeletCAEnabled is true, generating kubelet certificate for [%s]", hostAddress)
		err := rkepki.GenerateKubeletCertificate(ctx, bundle.Certs(), *rkeConfig, "", "", false)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate kubelet certificate")
		}
	}
	certString, err := bundle.SafeMarshal()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal certificates bundle")
	}
	nc.Certs = certString
	np := nodePlan.Plan
	nc.Processes = np.Processes
	nc.Files = np.Files
	if nodePlan.AgentCheckInterval != 0 {
		nc.AgentCheckInterval = nodePlan.AgentCheckInterval
	}

	if node.Status.AppliedNodeVersion != cluster.Status.NodeVersion {
		if nodePlan.Version == cluster.Status.NodeVersion {
			nc.NodeVersion = cluster.Status.NodeVersion
			logrus.Infof("cluster [%s] worker-upgrade: sending node-version for node [%s] version %v", cluster.Name, node.Status.NodeName, nc.NodeVersion)
		} else if v3.ClusterConditionUpgraded.IsUnknown(cluster) {
			if nc.AgentCheckInterval != AgentCheckIntervalDuringUpgrade {
				nodeCopy := node.DeepCopy()
				nodeCopy.Status.NodePlan.AgentCheckInterval = AgentCheckIntervalDuringUpgrade

				n.nodes.Update(nodeCopy)

				logrus.Infof("cluster [%s] worker-upgrade: updating [%s] with agent-interval [%v]", cluster.Name, node.Status.NodeName, AgentCheckIntervalDuringUpgrade)
				nc.AgentCheckInterval = AgentCheckIntervalDuringUpgrade
			}
		}
	}
	return nc, nil
}

func FilterHostForSpec(spec *v3.RancherKubernetesEngineConfig, n *v3.Node) {
	nodeList := make([]v3.RKEConfigNode, 0)
	for _, node := range spec.Nodes {
		if nodehelper.IsNonWorker(&node) || node.NodeName == n.Status.NodeConfig.NodeName {
			nodeList = append(nodeList, node)
		}
	}
	spec.Nodes = nodeList
}

func AugmentProcesses(token string, processes map[string]v3.Process, worker, b2d bool, nodeName string,
	cluster *v3.Cluster) map[string]v3.Process {
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
		agentImage := settings.AgentImage.Get()
		nodeCommand := clusterregistrationtokens.NodeCommand(token, cluster) + " --no-register --only-write-certs --node-name " + nodeName
		args := []string{"--", "share-root.sh", strings.TrimPrefix(nodeCommand, "sudo ")}
		args = append(args, shared...)
		privateRegistryConfig, _ := util.GenerateClusterPrivateRegistryDockerConfig(cluster)
		processes["share-mnt"] = v3.Process{
			Name:                    "share-mnt",
			Args:                    args,
			Image:                   image.ResolveWithCluster(agentImage, cluster),
			Binds:                   []string{"/var/run:/var/run"},
			NetworkMode:             "host",
			RestartPolicy:           "always",
			PidMode:                 "host",
			Privileged:              true,
			ImageRegistryAuthConfig: privateRegistryConfig,
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

func EnhanceWindowsProcesses(processes map[string]v3.Process) map[string]v3.Process {
	newProcesses := make(map[string]v3.Process, len(processes))
	for k, p := range processes {
		p.Binds = append(p.Binds,
			"//./pipe/rancher_wins://./pipe/rancher_wins",
		)
		newProcesses[k] = p
	}

	return newProcesses
}

func AppendTaintsToKubeletArgs(processes map[string]v3.Process, nodeConfigTaints []v3.RKETaint) map[string]v3.Process {
	if kubelet, ok := processes["kubelet"]; ok && len(nodeConfigTaints) != 0 {
		initialTaints := taints.GetTaintsFromStrings(taints.GetStringsFromRKETaint(nodeConfigTaints))
		var currentTaints []v1.Taint
		foundArgs := ""
		for i, arg := range kubelet.Command {
			if strings.HasPrefix(arg, "--register-with-taints=") {
				foundArgs = strings.TrimPrefix(arg, "--register-with-taints=")
				kubelet.Command = append(kubelet.Command[:i], kubelet.Command[i+1:]...)
				break
			}
		}
		if foundArgs != "" {
			currentTaints = taints.GetTaintsFromStrings(strings.Split(foundArgs, ","))
		}

		// The initial taints are from node pool and node template. They should override the taints from kubelet args.
		mergedTaints := taints.MergeTaints(currentTaints, initialTaints)

		taintArgs := fmt.Sprintf("--register-with-taints=%s", strings.Join(taints.GetStringsFromTaint(mergedTaints), ","))
		kubelet.Command = append(kubelet.Command, taintArgs)
		processes["kubelet"] = kubelet
	}
	return processes
}

func (n *RKENodeConfigServer) getServiceOptions(k8sVersion string, osType string) (map[string]interface{}, error) {
	data := map[string]interface{}{}
	svcOptions, err := kd.GetRKEK8sServiceOptions(k8sVersion, n.serviceOptionsLister, n.serviceOptions, n.sysImagesLister, n.sysImages, kd.Linux)
	if err != nil {
		logrus.Errorf("getK8sServiceOptions: k8sVersion %s [%v]", k8sVersion, err)
		return data, err
	}
	if svcOptions != nil {
		data["k8s-service-options"] = svcOptions
	}
	if osType == "windows" {
		svcOptionsWindows, err := kd.GetRKEK8sServiceOptions(k8sVersion, n.serviceOptionsLister, n.serviceOptions, n.sysImagesLister, n.sysImages, kd.Windows)
		if err != nil {
			logrus.Errorf("getK8sServiceOptionsWindows: k8sVersion %s [%v]", k8sVersion, err)
			return data, err
		}
		if svcOptionsWindows != nil {
			data["k8s-windows-service-options"] = svcOptionsWindows
		}
	}
	return data, nil
}
