package rkenodeconfigserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"reflect"

	docketypes "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/api/customization/clusterregistrationtokens"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/rkeworker"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/tunnelserver"
	rkecluster "github.com/rancher/rke/cluster"
	rkedocker "github.com/rancher/rke/docker"
	rkehosts "github.com/rancher/rke/hosts"
	rkepki "github.com/rancher/rke/pki"
	rkeservices "github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

var (
	b2Mount = "/mnt/sda1"
)

type RKENodeConfigServer struct {
	auth                 *tunnelserver.Authorizer
	lookup               *rkecerts.BundleLookup
	systemAccountManager *systemaccount.Manager
}

func Handler(auth *tunnelserver.Authorizer, scaledContext *config.ScaledContext) http.Handler {
	return &RKENodeConfigServer{
		auth:                 auth,
		lookup:               rkecerts.NewLookup(scaledContext.Core.Namespaces(""), scaledContext.Core),
		systemAccountManager: systemaccount.NewManagerFromScale(scaledContext),
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
		rw.Write([]byte("Failed to construct node config. Error: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(rw).Encode(nodeConfig); err != nil {
		logrus.Errorf("failed to write nodeConfig to agent: %v", err)
	}
}

func isNonWorkerOnly(role []string) bool {
	if slice.ContainsString(role, rkeservices.ETCDRole) ||
		slice.ContainsString(role, rkeservices.ControlRole) {
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
	rkeConfig.Nodes[0].Role = []string{rkeservices.WorkerRole, rkeservices.ETCDRole, rkeservices.ControlRole}

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
	token, err := n.systemAccountManager.GetOrCreateSystemClusterToken(cluster.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create or get cluster token for share-mnt")
	}
	for _, tempNode := range plan.Nodes {
		if tempNode.Address == node.Status.NodeConfig.Address {
			b2d := strings.Contains(infos[tempNode.Address].OperatingSystem, rkehosts.B2DOS)
			nc.Processes = augmentProcesses(token, tempNode.Processes, false, b2d)
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for non-worker %s", node.Status.NodeConfig.Address)
}

func (n *RKENodeConfigServer) nodeConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node) (*rkeworker.NodeConfig, error) {
	spec := cluster.Status.AppliedSpec.DeepCopy()

	infos, err := librke.GetDockerInfo(node)
	if err != nil {
		return nil, err
	}

	nodeDockerInfo := infos[node.Status.NodeConfig.Address]
	if nodeDockerInfo.OSType == "windows" {
		return n.windowsNodeConfig(ctx, cluster, node, getWindowsReleaseID(&nodeDockerInfo))
	}

	bundle, err := n.lookup.Lookup(cluster)
	if err != nil {
		return nil, err
	}

	bundle = bundle.ForNode(spec.RancherKubernetesEngineConfig, node.Status.NodeConfig.Address)

	certString, err := bundle.Marshal()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshall bundle")
	}

	rkeConfig := spec.RancherKubernetesEngineConfig
	filterHostForSpec(rkeConfig, node)
	logrus.Debugf("The number of nodes sent to the plan: %v", len(rkeConfig.Nodes))
	plan, err := librke.New().GeneratePlan(ctx, rkeConfig, infos)
	if err != nil {
		return nil, err
	}

	nc := &rkeworker.NodeConfig{
		Certs:       certString,
		ClusterName: cluster.Name,
	}
	token, err := n.systemAccountManager.GetOrCreateSystemClusterToken(cluster.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create or get cluster token for share-mnt")
	}
	for _, tempNode := range plan.Nodes {
		if tempNode.Address == node.Status.NodeConfig.Address {
			b2d := strings.Contains(infos[tempNode.Address].OperatingSystem, rkehosts.B2DOS)
			nc.Processes = augmentProcesses(token, tempNode.Processes, true, b2d)
			nc.Files = tempNode.Files
			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for %s", node.Status.NodeConfig.Address)
}

func filterHostForSpec(spec *v3.RancherKubernetesEngineConfig, n *v3.Node) {
	nodeList := make([]v3.RKEConfigNode, 0)
	for _, node := range spec.Nodes {
		if isNonWorkerOnly(node.Role) || node.NodeName == n.Status.NodeConfig.NodeName {
			nodeList = append(nodeList, node)
		}
	}
	spec.Nodes = nodeList
}

func augmentProcesses(token string, processes map[string]v3.Process, worker, b2d bool) map[string]v3.Process {
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
		nodeCommand := clusterregistrationtokens.NodeCommand(token) + " --no-register --only-write-certs"
		args := []string{"--", "share-root.sh", strings.TrimPrefix(nodeCommand, "sudo ")}
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

func (n *RKENodeConfigServer) windowsNodeConfig(ctx context.Context, cluster *v3.Cluster, node *v3.Node, windowsReleaseID string) (*rkeworker.NodeConfig, error) {
	rkeConfig := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig
	if rkeConfig == nil {
		return nil, errors.New("only work on the clusters built with 'custom node'")
	}

	for _, tempNode := range rkeConfig.Nodes {
		if tempNode.Address == node.Status.NodeConfig.Address {
			bundle, err := n.lookup.Lookup(cluster)
			if err != nil {
				return nil, err
			}

			bundle = bundle.ForWindowsNode(rkeConfig, node.Status.NodeConfig.Address, windowsReleaseID)
			certString, err := bundle.Marshal()
			if err != nil {
				return nil, errors.Wrapf(err, "failed to marshall cert bundle for %s", node.Status.NodeConfig.Address)
			}

			systemImages := formatSystemImages(v3.K8sVersionWindowsSystemImages[rkeConfig.Version], windowsReleaseID)
			process, err := createWindowsProcesses(rkeConfig, node.Status.NodeConfig, systemImages)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create windows node plan for %s", node.Status.NodeConfig.Address)
			}

			nc := &rkeworker.NodeConfig{
				Certs:       certString,
				ClusterName: cluster.Name,
				Processes:   process,
			}

			return nc, nil
		}
	}

	return nil, fmt.Errorf("failed to find plan for %s", node.Status.NodeConfig.Address)
}

func createWindowsProcesses(rkeConfig *v3.RancherKubernetesEngineConfig, configNode *v3.RKEConfigNode, systemImages v3.WindowsSystemImages) (map[string]v3.Process, error) {
	cniBinariesImage := ""
	kubernetesBinariesImage := systemImages.KubernetesBinaries
	nginxProxyImage := systemImages.NginxProxy
	kubeletPauseImage := systemImages.KubeletPause

	// network
	cniComponent := strings.ToLower(rkeConfig.Network.Plugin)
	switch cniComponent {
	case "flannel", "canal":
		cniBinariesImage = systemImages.FlannelCNIBinaries
	default:
		return nil, fmt.Errorf("windows node can't support %s network plugin", rkeConfig.Network.Plugin)
	}
	cniMode := "overlay"
	if len(rkeConfig.Network.Options) > 0 {
		for _, backendTypeName := range []string{rkecluster.FlannelBackendType, rkecluster.CanalFlannelBackendType} {
			if flannelBackendType, ok := rkeConfig.Network.Options[backendTypeName]; ok {
				flannelBackendType = strings.ToLower(flannelBackendType)
				if flannelBackendType == "host-gw" {
					cniMode = "l2bridge"
				}
				break
			}
		}
	}

	// get kubernetes masters
	controlPlanes := make([]string, 0)
	for _, host := range rkeConfig.Nodes {
		for _, role := range host.Role {
			if role == rkeservices.ControlRole {
				address := host.InternalAddress
				if len(address) == 0 {
					address = host.Address
				}
				controlPlanes = append(controlPlanes, address)
			}
		}
	}

	// get private registeries
	privateRegistriesMap := make(map[string]v3.PrivateRegistry)
	for _, pr := range rkeConfig.PrivateRegistries {
		if pr.URL == "" {
			pr.URL = rkedocker.DockerRegistryURL
		}
		privateRegistriesMap[pr.URL] = pr
	}
	registryAuthConfig := ""

	result := make(map[string]v3.Process)

	registryAuthConfig, _, _ = rkedocker.GetImageRegistryConfig(kubernetesBinariesImage, privateRegistriesMap)
	result["pre-run-docker-kubernetes-binaries"] = v3.Process{
		Name:  "kubernetes-binaries",
		Image: kubernetesBinariesImage,
		Command: []string{
			"pwsh.exe",
		},
		Args: []string{
			"-f",
			"c:\\Program Files\\runtime\\copy.ps1",
		},
		Binds: []string{
			"c:\\etc\\kubernetes:c:\\kubernetes",
		},
		ImageRegistryAuthConfig: registryAuthConfig,
	}

	registryAuthConfig, _, _ = rkedocker.GetImageRegistryConfig(cniBinariesImage, privateRegistriesMap)
	result["pre-run-docker-cni-binaries"] = v3.Process{
		Name:  "cni-binaries",
		Image: cniBinariesImage,
		Env: []string{
			fmt.Sprintf("MODE=%s", cniMode),
		},
		Command: []string{
			"pwsh.exe",
		},
		Args: []string{
			"-f",
			"c:\\Program Files\\runtime\\copy.ps1",
		},
		Binds: []string{
			"c:\\etc\\cni:c:\\cni",
		},
		ImageRegistryAuthConfig: registryAuthConfig,
	}

	registryAuthConfig, _, _ = rkedocker.GetImageRegistryConfig(nginxProxyImage, privateRegistriesMap)
	result["post-run-docker-nginx-proxy"] = v3.Process{
		Name:  "nginx-proxy",
		Image: nginxProxyImage,
		Env: []string{
			fmt.Sprintf("%s=%s", rkeservices.NginxProxyEnvName, strings.Join(controlPlanes, ",")),
		},
		Command: []string{
			"pwsh.exe",
		},
		RestartPolicy: "always",
		Args: []string{
			"-f",
			"c:\\Program Files\\runtime\\start.ps1",
		},
		Publish: []string{
			"6443:6443",
		},
		ImageRegistryAuthConfig: registryAuthConfig,
	}

	// hyperkube fake process
	clusterCIDR := rkeConfig.Services.KubeController.ClusterCIDR
	if len(clusterCIDR) == 0 {
		clusterCIDR = rkecluster.DefaultClusterCIDR
	}
	clusterDomain := rkeConfig.Services.Kubelet.ClusterDomain
	if len(clusterDomain) == 0 {
		clusterDomain = rkecluster.DefaultClusterDomain
	}
	serviceCIDR := rkeConfig.Services.KubeController.ServiceClusterIPRange
	if len(serviceCIDR) == 0 {
		serviceCIDR = rkecluster.DefaultServiceClusterIPRange
	}
	dnsServiceIP := rkeConfig.Services.Kubelet.ClusterDNSServer
	if len(dnsServiceIP) == 0 {
		dnsServiceIP = rkecluster.DefaultClusterDNSService
	}
	clusterMajorVersion := getTagMajorVersion(rkeConfig.Version)
	serviceOptions := v3.K8sVersionWindowsServiceOptions[clusterMajorVersion]

	extendingKubeletOptions := extendMap(map[string]string{
		"v": "2",
		"pod-infra-container-image":    kubeletPauseImage,
		"allow-privileged":             "true",
		"anonymous-auth":               "false",
		"image-pull-progress-deadline": "20m",
		"register-with-taints":         "beta.kubernetes.io/os=windows:PreferNoSchedule",
		"client-ca-file":               "c:" + rkepki.GetCertPath(rkepki.CACertName),
		"kubeconfig":                   "c:" + rkepki.GetConfigPath(rkepki.KubeNodeCertName),
		"hostname-override":            configNode.HostnameOverride,
		"cluster-domain":               clusterDomain,
		"cluster-dns":                  dnsServiceIP,
		"node-ip":                      configNode.InternalAddress,
	}, serviceOptions.Kubelet)
	extendingKubeproxyOptions := extendMap(map[string]string{
		"v":                 "2",
		"proxy-mode":        "userspace",
		"kubeconfig":        "c:" + rkepki.GetConfigPath(rkepki.KubeProxyCertName),
		"hostname-override": configNode.HostnameOverride,
	}, serviceOptions.Kubeproxy)

	result["hyperkube"] = v3.Process{
		Name: "hyperkube",
		Command: []string{
			"powershell.exe",
		},
		Args: []string{
			"-KubeClusterCIDR", clusterCIDR,
			"-KubeClusterDomain", clusterDomain,
			"-KubeServiceCIDR", serviceCIDR,
			"-KubeDnsServiceIP", dnsServiceIP,
			"-KubeCNIComponent", cniComponent,
			"-KubeCNIMode", cniMode,
			"-KubeletOptions", translateMapToTuples(extendingKubeletOptions),
			"-KubeProxyOptions", translateMapToTuples(extendingKubeproxyOptions),
			"-NodeIP", configNode.InternalAddress,
			"-NodeName", configNode.HostnameOverride,
		},
	}

	return result, nil
}

func getTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
}

func extendMap(sourceMap, targetMap map[string]string) map[string]string {
	if len(targetMap) != 0 {
		for key, val := range targetMap {
			sourceMap[key] = val
		}
	}
	return sourceMap
}

func translateMapToTuples(options map[string]string) string {
	result := ""
	if len(options) != 0 {
		kvPairs := make([]string, 0, len(options))
		for key, val := range options {
			kvPairs = append(kvPairs, fmt.Sprintf(`"--%s=%v"`, key, val))
		}
		result = strings.Join(kvPairs, ";")
	}
	return result
}

func getWindowsReleaseID(nodeDockerInfo *docketypes.Info) string {
	windowsBuildNumber := "17134"
	windowsReleaseID := "1803"

	// get build number of windows
	// e.g.: 10.0 16299 (16299.15.amd64fre.rs3_release.170928-1534)
	kernelVersionSplits := strings.Split(nodeDockerInfo.KernelVersion, " ")
	if len(kernelVersionSplits) == 3 {
		windowsBuildNumber = kernelVersionSplits[1]
	}

	// translate build number to release id
	// ref: https://www.microsoft.com/en-us/itpro/windows-10/release-information
	switch windowsBuildNumber {
	case "16299":
		windowsReleaseID = "1709"
	}

	return windowsReleaseID
}

func formatSystemImages(originSystemImages v3.WindowsSystemImages, windowsReleaseID string) v3.WindowsSystemImages {
	origin := reflect.ValueOf(originSystemImages)
	shadow := reflect.New(origin.Type()).Elem()
	for i := 0; i < origin.NumField(); i++ {
		originVal := origin.Field(i).String()
		dealVal := strings.Replace(originVal, "1803", windowsReleaseID, -1)
		shadow.Field(i).SetString(dealVal)
	}

	return shadow.Interface().(v3.WindowsSystemImages)
}
