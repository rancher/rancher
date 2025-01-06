package provisioning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/norman/types"

	"github.com/sirupsen/logrus"

	"github.com/rancher/shepherd/clients/corral"
	"github.com/rancher/shepherd/clients/rancher"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	k3sHardening "github.com/rancher/rancher/tests/v2/actions/hardening/k3s"
	rke1Hardening "github.com/rancher/rancher/tests/v2/actions/hardening/rke1"
	rke2Hardening "github.com/rancher/rancher/tests/v2/actions/hardening/rke2"
	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/rancher/tests/v2/actions/pipeline"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	nodepools "github.com/rancher/rancher/tests/v2/actions/rke1/nodepools"
	"github.com/rancher/rancher/tests/v2/actions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/v2/actions/secrets"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/cloudcredentials/aws"
	"github.com/rancher/shepherd/extensions/cloudcredentials/azure"
	"github.com/rancher/shepherd/extensions/cloudcredentials/google"
	"github.com/rancher/shepherd/extensions/cloudcredentials/vsphere"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/aks"
	"github.com/rancher/shepherd/extensions/clusters/eks"
	"github.com/rancher/shepherd/extensions/clusters/gke"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/etcdsnapshot"
	nodestat "github.com/rancher/shepherd/extensions/nodes"
	"github.com/rancher/shepherd/extensions/tokenregistration"
	"github.com/rancher/shepherd/pkg/environmentflag"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/rancher/shepherd/pkg/wait"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"

	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
)

const (
	active         = "active"
	internalIP     = "alpha.kubernetes.io/provided-node-ip"
	rke1ExternalIP = "rke.cattle.io/external-ip"
	namespace      = "fleet-default"

	rke2k3sAirgapCustomCluster           = "rke2k3sairgapcustomcluster"
	rke2k3sNodeCorralName                = "rke2k3sregisterNode"
	corralPackageAirgapCustomClusterName = "airgapCustomCluster"
	rke1AirgapCustomCluster              = "rke1airgapcustomcluster"
	rke1NodeCorralName                   = "rke1registerNode"
)

// CreateProvisioningCluster provisions a non-rke1 cluster, then runs verify checks
func CreateProvisioningCluster(client *rancher.Client, provider Provider, clustersConfig *clusters.ClusterConfig, hostnameTruncation []machinepools.HostnameTruncation) (*v1.SteveAPIObject, error) {
	credentialSpec := cloudcredentials.LoadCloudCredential(string(provider.Name))
	cloudCredential, err := provider.CloudCredFunc(client, credentialSpec)
	if err != nil {
		return nil, err
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err = clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	clusterName := namegen.AppendRandomString(provider.Name.String())
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfigs := provider.MachinePoolFunc(generatedPoolName, namespace)

	var machinePoolResponses []v1.SteveAPIObject

	for _, machinePoolConfig := range machinePoolConfigs {
		machinePoolConfigResp, err := client.Steve.
			SteveType(provider.MachineConfigPoolResourceSteveType).
			Create(&machinePoolConfig)
		if err != nil {
			return nil, err
		}
		machinePoolResponses = append(machinePoolResponses, *machinePoolConfigResp)
	}

	if clustersConfig.Registries != nil {
		if clustersConfig.Registries.RKE2Registries != nil {
			if clustersConfig.Registries.RKE2Username != "" && clustersConfig.Registries.RKE2Password != "" {
				steveClient, err := client.Steve.ProxyDownstream("local")
				if err != nil {
					return nil, err
				}

				secretName := fmt.Sprintf("priv-reg-sec-%s", clusterName)
				secretTemplate := secrets.NewSecretTemplate(secretName, namespace, map[string][]byte{
					"password": []byte(clustersConfig.Registries.RKE2Password),
					"username": []byte(clustersConfig.Registries.RKE2Username),
				},
					corev1.SecretTypeBasicAuth,
				)

				registrySecret, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
				if err != nil {
					return nil, err
				}

				for registryName, registry := range clustersConfig.Registries.RKE2Registries.Configs {
					registry.AuthConfigSecretName = registrySecret.Name
					clustersConfig.Registries.RKE2Registries.Configs[registryName] = registry
				}
			}
		}
	}

	var machineConfigs []machinepools.MachinePoolConfig
	var pools []machinepools.Pools
	for _, pool := range clustersConfig.MachinePools {
		machineConfigs = append(machineConfigs, pool.MachinePoolConfig)
		pools = append(pools, pool.Pools)
	}

	machinePools := machinepools.
		CreateAllMachinePools(machineConfigs, pools, machinePoolResponses, provider.Roles, hostnameTruncation)

	if clustersConfig.CloudProvider == provisioninginput.VsphereCloudProviderName.String() {

		vcenterCredentials := map[string]interface{}{
			"datacenters": machinePoolConfigs[0].Object["datacenter"],
			"host":        credentialSpec.VmwareVsphereConfig.Vcenter,
			"password":    vsphere.GetVspherePassword(),
			"username":    credentialSpec.VmwareVsphereConfig.Username,
		}
		clustersConfig.AddOnConfig = &provisioninginput.AddOnConfig{
			ChartValues: &rkev1.GenericMap{
				Data: map[string]interface{}{
					"rancher-vsphere-cpi": map[string]interface{}{
						"vCenter": vcenterCredentials,
					},
					"rancher-vsphere-csi": map[string]interface{}{
						"storageClass": map[string]interface{}{
							"datastoreURL": machinePoolConfigs[0].Object["datastoreUrl"],
						},
						"vCenter": vcenterCredentials,
					},
				},
			},
		}
	}

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredential.Namespace+":"+cloudCredential.Name)

	for _, truncatedPool := range hostnameTruncation {
		if truncatedPool.PoolNameLengthLimit > 0 || truncatedPool.ClusterNameLengthLimit > 0 {
			cluster.GenerateName = "t-"
			if truncatedPool.ClusterNameLengthLimit > 0 {
				cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit = truncatedPool.ClusterNameLengthLimit
			}

			break
		}
	}

	_, err = shepherdclusters.CreateK3SRKE2Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return nil, err
	}

	createdCluster, err := adminClient.Steve.
		SteveType(stevetypes.Provisioning).
		ByID(namespace + "/" + clusterName)

	return createdCluster, err
}

// CreateProvisioningCustomCluster provisions a non-rke1 cluster using a 3rd party client for its nodes, then runs verify checks
func CreateProvisioningCustomCluster(client *rancher.Client, externalNodeProvider *ExternalNodeProvider, clustersConfig *clusters.ClusterConfig) (*v1.SteveAPIObject, error) {
	setLogrusFormatter()
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, pool := range clustersConfig.MachinePools {
		var finalRoleCommand string
		if pool.MachinePoolConfig.ControlPlane {
			finalRoleCommand += " --controlplane"
		}

		if pool.MachinePoolConfig.Etcd {
			finalRoleCommand += " --etcd"
		}

		if pool.MachinePoolConfig.Worker {
			finalRoleCommand += " --worker"
		}

		if pool.MachinePoolConfig.Windows {
			finalRoleCommand += " --windows"
		}

		quantityPerPool = append(quantityPerPool, pool.MachinePoolConfig.Quantity)
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
		for i := int32(0); i < pool.MachinePoolConfig.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool)
	if err != nil {
		return nil, err
	}

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, nil, "")

	if clustersConfig.Hardened && strings.Contains(clustersConfig.KubernetesVersion, shepherdclusters.RKE2ClusterType.String()) {
		err = rke2Hardening.HardenRKE2Nodes(nodes, rolesPerNode)
		if err != nil {
			return nil, err
		}

		cluster = clusters.HardenRKE2ClusterConfig(clusterName, namespace, clustersConfig, nil, "")
	}

	clusterResp, err := shepherdclusters.CreateK3SRKE2Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	customCluster, err := client.Steve.SteveType(etcdsnapshot.ProvisioningSteveResouceType).ByID(clusterResp.ID)
	if err != nil {
		return nil, err
	}

	clusterStatus := &apiv1.ClusterStatus{}
	err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
	if err != nil {
		return nil, err
	}

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	if err != nil {
		return nil, err
	}

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, err
	}

	result, err := kubeProvisioningClient.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	checkFunc := shepherdclusters.IsProvisioningClusterReady
	var command string
	totalNodesObserved := 0
	for poolIndex, poolRole := range rolesPerPool {
		if strings.Contains(poolRole, "windows") {
			totalNodesObserved += int(quantityPerPool[poolIndex])
			continue
		}
		for nodeIndex := 0; nodeIndex < int(quantityPerPool[poolIndex]); nodeIndex++ {
			node := nodes[totalNodesObserved+nodeIndex]

			logrus.Infof("Execute Registration Command for node %s", node.NodeID)
			logrus.Infof("Linux pool detected, using bash...")

			command = fmt.Sprintf("%s %s", token.InsecureNodeCommand, poolRole)
			if clustersConfig.MachinePools[poolIndex].IsSecure {
				command = fmt.Sprintf("%s %s", token.NodeCommand, poolRole)
			}
			command = createRegistrationCommand(command, node.PublicIPAddress, node.PrivateIPAddress, clustersConfig.MachinePools[poolIndex])
			logrus.Infof("Command: %s", command)

			output, err := node.ExecuteCommand(command)
			if err != nil {
				return nil, err
			}
			logrus.Infof(output)
		}
		totalNodesObserved += int(quantityPerPool[poolIndex])
	}

	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return nil, err
	}
	totalNodesObserved = 0
	for poolIndex := 0; poolIndex < len(rolesPerPool); poolIndex++ {
		if strings.Contains(rolesPerPool[poolIndex], "windows") {
			for nodeIndex := 0; nodeIndex < int(quantityPerPool[poolIndex]); nodeIndex++ {
				node := nodes[totalNodesObserved+nodeIndex]

				logrus.Infof("Execute Registration Command for node %s", node.NodeID)
				logrus.Infof("Windows pool detected, using powershell.exe...")
				command = fmt.Sprintf("powershell.exe %s ", token.InsecureWindowsNodeCommand)
				if clustersConfig.MachinePools[poolIndex].IsSecure {
					command = fmt.Sprintf("powershell.exe %s ", token.WindowsNodeCommand)
				}
				command = createWindowsRegistrationCommand(command, node.PublicIPAddress, node.PrivateIPAddress, clustersConfig.MachinePools[poolIndex])
				logrus.Infof("Command: %s", command)

				output, err := node.ExecuteCommand(command)
				if err != nil {
					return nil, err
				}
				logrus.Infof(output)
			}
		}
		totalNodesObserved += int(quantityPerPool[poolIndex])
	}

	if clustersConfig.Hardened {
		if strings.Contains(clustersConfig.KubernetesVersion, shepherdclusters.K3SClusterType.String()) {
			err = k3sHardening.HardenK3SNodes(nodes, rolesPerNode, clustersConfig.KubernetesVersion)
			if err != nil {
				return nil, err
			}

			hardenCluster := clusters.HardenK3SClusterConfig(clusterName, namespace, clustersConfig, nil, "")

			_, err := shepherdclusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
			if err != nil {
				return nil, err
			}
		} else {
			err = rke2Hardening.PostRKE2HardeningConfig(nodes, rolesPerNode)
			if err != nil {
				return nil, err
			}
		}
	}

	createdCluster, err := client.Steve.
		SteveType(stevetypes.Provisioning).
		ByID(namespace + "/" + clusterName)
	return createdCluster, err
}

// CreateProvisioningRKE1Cluster provisions an rke1 cluster, then runs verify checks
func CreateProvisioningRKE1Cluster(client *rancher.Client, provider RKE1Provider, clustersConfig *clusters.ClusterConfig, nodeTemplate *nodetemplates.NodeTemplate) (*management.Cluster, error) {
	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	clusterName := namegen.AppendRandomString(provider.Name.String())
	cluster := clusters.NewRKE1ClusterConfig(clusterName, client, clustersConfig)
	clusterResp, err := shepherdclusters.CreateRKE1Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	var nodeRoles []nodepools.NodeRoles
	for _, nodes := range clustersConfig.NodePools {
		nodeRoles = append(nodeRoles, nodes.NodeRoles)
	}
	_, err = nodepools.NodePoolSetup(client, nodeRoles, clusterResp.ID, nodeTemplate.ID)
	if err != nil {
		return nil, err
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	return createdCluster, err
}

// CreateProvisioningRKE1CustomCluster provisions an rke1 cluster using a 3rd party client for its nodes, then runs verify checks
func CreateProvisioningRKE1CustomCluster(client *rancher.Client, externalNodeProvider *ExternalNodeProvider, clustersConfig *clusters.ClusterConfig) (*management.Cluster, []*nodes.Node, error) {
	setLogrusFormatter()
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, pool := range clustersConfig.NodePools {
		var finalRoleCommand string
		if pool.NodeRoles.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if pool.NodeRoles.Etcd {
			finalRoleCommand += " --etcd"
		}
		if pool.NodeRoles.Worker {
			finalRoleCommand += " --worker"
		}

		quantityPerPool = append(quantityPerPool, int32(pool.NodeRoles.Quantity))
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, nil, err
		}
	}

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool)
	if err != nil {
		return nil, nil, err
	}

	clusterName := namegen.AppendRandomString(externalNodeProvider.Name)

	cluster := clusters.NewRKE1ClusterConfig(clusterName, client, clustersConfig)

	if clustersConfig.Hardened {
		err = rke1Hardening.HardenRKE1Nodes(nodes, rolesPerPool)
		if err != nil {
			return nil, nil, err
		}

		cluster = clusters.HardenRKE1ClusterConfig(client, clusterName, clustersConfig)
	}

	clusterResp, err := shepherdclusters.CreateRKE1Cluster(client, cluster)
	if err != nil {
		return nil, nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, nil, err
	}

	customCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	if err != nil {
		return nil, nil, err
	}

	token, err := tokenregistration.GetRegistrationToken(client, customCluster.ID)
	if err != nil {
		return nil, nil, err
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return nil, nil, err
	}

	result, err := adminClient.GetManagementWatchInterface(management.ClusterType, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + customCluster.ID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return nil, nil, err
	}

	checkFunc := shepherdclusters.IsHostedProvisioningClusterReady

	var command string
	totalNodesObserved := 0
	for poolIndex, poolRole := range rolesPerPool {
		for nodeIndex := 0; nodeIndex < int(quantityPerPool[poolIndex]); nodeIndex++ {
			node := nodes[totalNodesObserved+nodeIndex]

			logrus.Infof("Execute Registration Command for node %s", node.NodeID)
			logrus.Infof("Linux pool detected, using bash...")

			command = fmt.Sprintf("%s %s", token.NodeCommand, poolRole)
			command = createRKE1RegistrationCommand(command, node.PublicIPAddress, node.PrivateIPAddress, clustersConfig.NodePools[poolIndex])
			logrus.Infof("Command: %s", command)

			if clustersConfig.RKE1CustomClusterDockerInstall != nil && clustersConfig.RKE1CustomClusterDockerInstall.InstallDockerURL != "" {
				_, err := node.ExecuteCommand("curl " + clustersConfig.RKE1CustomClusterDockerInstall.InstallDockerURL + " | sh")
				if err != nil {
					return nil, nil, err
				}

				_, err = node.ExecuteCommand("sudo systemctl start docker")
				if err != nil {
					return nil, nil, err
				}

				_, err = node.ExecuteCommand("sudo chmod 777 /var/run/docker.sock")
				if err != nil {
					return nil, nil, err
				}
			}

			output, err := node.ExecuteCommand(command)
			if err != nil {
				return nil, nil, err
			}
			logrus.Infof(output)
		}
		totalNodesObserved += int(quantityPerPool[poolIndex])
	}

	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return nil, nil, err
	}

	if clustersConfig.Hardened {
		err = rke1Hardening.PostRKE1HardeningConfig(nodes, rolesPerPool)
		if err != nil {
			return nil, nil, err
		}
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)

	return createdCluster, nodes, err
}

// CreateProvisioningAirgapCustomCluster provisions a non-rke1 cluster using corral to gather its nodes, then runs verify checks
func CreateProvisioningAirgapCustomCluster(client *rancher.Client, clustersConfig *clusters.ClusterConfig, corralPackages *corral.Packages) (*v1.SteveAPIObject, error) {
	setLogrusFormatter()
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, pool := range clustersConfig.MachinePools {
		var finalRoleCommand string
		if pool.MachinePoolConfig.ControlPlane {
			finalRoleCommand += " --controlplane"
		}

		if pool.MachinePoolConfig.Etcd {
			finalRoleCommand += " --etcd"
		}

		if pool.MachinePoolConfig.Worker {
			finalRoleCommand += " --worker"
		}

		if pool.MachinePoolConfig.Windows {
			finalRoleCommand += " --windows"
		}

		quantityPerPool = append(quantityPerPool, pool.MachinePoolConfig.Quantity)
		rolesPerPool = append(rolesPerPool, finalRoleCommand)

	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	clusterName := namegen.AppendRandomString(rke2k3sAirgapCustomCluster)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, nil, "")

	clusterResp, err := shepherdclusters.CreateK3SRKE2Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	customCluster, err := client.Steve.SteveType(stevetypes.Provisioning).ByID(clusterResp.ID)
	if err != nil {
		return nil, err
	}

	clusterStatus := &apiv1.ClusterStatus{}
	err = v1.ConvertToK8sType(customCluster.Status, clusterStatus)
	if err != nil {
		return nil, err
	}

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Register Custom Cluster Through Corral")
	corralsArgs := []corral.Args{}

	for poolIndex, poolRole := range rolesPerPool {

		regCmd := fmt.Sprintf("%s %s", token.InsecureNodeCommand, poolRole)

		// environment variables must be escaped inside original registration command
		regCmd = strings.Replace(regCmd, "\"", "\\\"", -1)

		corralsArgs = append(corralsArgs, corral.Args{
			Name:        namegen.AppendRandomString(rke2k3sNodeCorralName),
			PackageName: corralPackages.CorralPackageImages[corralPackageAirgapCustomClusterName],
			Updates:     map[string]string{"registration_command": regCmd, "node_count": fmt.Sprint(quantityPerPool[poolIndex])},
		})
	}

	_, err = corral.CreateMultipleCorrals(client.Session, corralsArgs, corralPackages.HasDebug, corralPackages.HasCleanup)
	if err != nil {
		return nil, err
	}

	createdCluster, err := client.Steve.SteveType(stevetypes.Provisioning).ByID(namespace + "/" + clusterName)
	return createdCluster, err
}

// CreateProvisioningRKE1AirgapCustomCluster provisions an rke1 cluster using corral to gather its nodes, then runs verify checks
func CreateProvisioningRKE1AirgapCustomCluster(client *rancher.Client, clustersConfig *clusters.ClusterConfig, corralPackages *corral.Packages) (*management.Cluster, error) {
	setLogrusFormatter()
	clusterName := namegen.AppendRandomString(rke1AirgapCustomCluster)
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, pool := range clustersConfig.NodePools {
		var finalRoleCommand string
		if pool.NodeRoles.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if pool.NodeRoles.Etcd {
			finalRoleCommand += " --etcd"
		}
		if pool.NodeRoles.Worker {
			finalRoleCommand += " --worker"
		}

		quantityPerPool = append(quantityPerPool, int32(pool.NodeRoles.Quantity))
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	cluster := clusters.NewRKE1ClusterConfig(clusterName, client, clustersConfig)
	clusterResp, err := shepherdclusters.CreateRKE1Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	customCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	if err != nil {
		return nil, err
	}

	token, err := tokenregistration.GetRegistrationToken(client, customCluster.ID)
	if err != nil {
		return nil, err
	}

	corralsArgs := []corral.Args{}

	logrus.Infof("Register Custom Cluster Through Corral")
	for poolIndex, poolRole := range rolesPerPool {
		// environment variables must be escaped inside original registration command
		escapedCommand := strings.Replace(token.NodeCommand, "\"", "\\\"", -1)

		corralsArgs = append(corralsArgs, corral.Args{
			Name:        namegen.AppendRandomString(rke1NodeCorralName),
			PackageName: corralPackages.CorralPackageImages[corralPackageAirgapCustomClusterName],
			Updates:     map[string]string{"registration_command": fmt.Sprintf("%s %s", escapedCommand, poolRole), "node_count": fmt.Sprint(quantityPerPool[poolIndex])},
		})
	}

	_, err = corral.CreateMultipleCorrals(client.Session, corralsArgs, corralPackages.HasDebug, corralPackages.HasCleanup)
	if err != nil {
		return nil, err
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	return createdCluster, err
}

// CreateProvisioningAKSHostedCluster provisions an AKS cluster, then runs verify checks
func CreateProvisioningAKSHostedCluster(client *rancher.Client, aksClusterConfig aks.ClusterConfig) (*management.Cluster, error) {
	cloudCredentialConfig := cloudcredentials.LoadCloudCredential("azure")
	cloudCredential, err := azure.CreateAzureCloudCredentials(client, cloudCredentialConfig)
	if err != nil {
		return nil, err
	}

	clusterName := namegen.AppendRandomString("akshostcluster")
	clusterResp, err := aks.CreateAKSHostedCluster(client, clusterName, cloudCredential.Namespace+":"+cloudCredential.Name, aksClusterConfig, false, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	return client.Management.Cluster.ByID(clusterResp.ID)
}

// CreateProvisioningEKSHostedCluster provisions an EKS cluster, then runs verify checks
func CreateProvisioningEKSHostedCluster(client *rancher.Client, eksClusterConfig eks.ClusterConfig) (*management.Cluster, error) {
	cloudCredentialConfig := cloudcredentials.LoadCloudCredential("aws")
	cloudCredential, err := aws.CreateAWSCloudCredentials(client, cloudCredentialConfig)
	if err != nil {
		return nil, err
	}

	clusterName := namegen.AppendRandomString("ekshostcluster")
	clusterResp, err := eks.CreateEKSHostedCluster(client, clusterName, cloudCredential.Namespace+":"+cloudCredential.Name, eksClusterConfig, false, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	return client.Management.Cluster.ByID(clusterResp.ID)
}

// CreateProvisioningGKEHostedCluster provisions an GKE cluster, then runs verify checks
func CreateProvisioningGKEHostedCluster(client *rancher.Client, gkeClusterConfig gke.ClusterConfig) (*management.Cluster, error) {
	credentialSpec := cloudcredentials.LoadCloudCredential(provisioninginput.GoogleProviderName.String())
	cloudCredential, err := google.CreateGoogleCloudCredentials(client, credentialSpec)
	if err != nil {
		return nil, err
	}

	clusterName := namegen.AppendRandomString("gkehostcluster")
	clusterResp, err := gke.CreateGKEHostedCluster(client, clusterName, cloudCredential.Namespace+":"+cloudCredential.Name, gkeClusterConfig, false, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	return client.Management.Cluster.ByID(clusterResp.ID)
}

func setLogrusFormatter() {
	formatter := &logrus.TextFormatter{}
	formatter.DisableQuote = true
	logrus.SetFormatter(formatter)
}

// createRKE1RegistrationCommand is a helper for rke1 custom clusters to create the registration command with advanced options configured per node
func createRKE1RegistrationCommand(command, publicIP, privateIP string, nodePool provisioninginput.NodePools) string {
	if nodePool.SpecifyCustomPublicIP {
		command += fmt.Sprintf(" --address %s", publicIP)
	}
	if nodePool.SpecifyCustomPrivateIP {
		command += fmt.Sprintf(" --internal-address %s", privateIP)
	}
	if nodePool.CustomNodeNameSuffix != "" {
		command += fmt.Sprintf(" --node-name %s", namegen.AppendRandomString(nodePool.CustomNodeNameSuffix))
	}
	for labelKey, labelValue := range nodePool.NodeLabels {
		command += fmt.Sprintf(" --label %s=%s", labelKey, labelValue)
	}
	for _, taint := range nodePool.NodeTaints {
		command += fmt.Sprintf(" --taints %s=%s:%s", taint.Key, taint.Value, taint.Effect)
	}
	return command
}

// createRegistrationCommand is a helper for rke2/k3s custom clusters to create the registration command with advanced options configured per node
func createRegistrationCommand(command, publicIP, privateIP string, machinePool provisioninginput.MachinePools) string {
	if machinePool.SpecifyCustomPublicIP {
		command += fmt.Sprintf(" --address %s", publicIP)
	}
	if machinePool.SpecifyCustomPrivateIP {
		command += fmt.Sprintf(" --internal-address %s", privateIP)
	}
	if machinePool.CustomNodeNameSuffix != "" {
		command += fmt.Sprintf(" --node-name %s", namegen.AppendRandomString(machinePool.CustomNodeNameSuffix))
	}
	for labelKey, labelValue := range machinePool.NodeLabels {
		command += fmt.Sprintf(" --label %s=%s", labelKey, labelValue)
	}
	for _, taint := range machinePool.NodeTaints {
		command += fmt.Sprintf(" --taints %s=%s:%s", taint.Key, taint.Value, taint.Effect)
	}
	return command
}

// createWindowsRegistrationCommand is a helper for rke2 windows custom clusters to create the registration command with advanced options configured per node
func createWindowsRegistrationCommand(command, publicIP, privateIP string, machinePool provisioninginput.MachinePools) string {
	if machinePool.SpecifyCustomPublicIP {
		command += fmt.Sprintf(" -Address '%s'", publicIP)
	}
	if machinePool.SpecifyCustomPrivateIP {
		command += fmt.Sprintf(" -InternalAddress '%s'", privateIP)
	}
	if machinePool.CustomNodeNameSuffix != "" {
		command += fmt.Sprintf(" -NodeName '%s'", namegen.AppendRandomString(machinePool.CustomNodeNameSuffix))
	}
	// powershell requires only 1 flag per command, so we need to append the custom labels and taints together which is different from linux
	if len(machinePool.NodeLabels) > 0 {
		// there is an existing label for all windows nodes, so we need to insert the custom labels after the existing label
		labelIndex := strings.Index(command, " -Label '") + len(" -Label '")
		customLabels := ""
		for labelKey, labelValue := range machinePool.NodeLabels {
			customLabels += fmt.Sprintf("%s=%s,", labelKey, labelValue)
		}
		command = command[:labelIndex] + customLabels + command[labelIndex:]
	}
	if len(machinePool.NodeTaints) > 0 {
		var customTaints string
		for _, taint := range machinePool.NodeTaints {
			customTaints += fmt.Sprintf("%s=%s:%s,", taint.Key, taint.Value, taint.Effect)
		}
		wrappedTaints := fmt.Sprintf(" -Taint '%s'", customTaints)
		command += wrappedTaints
	}
	return command
}

// AddRKE2K3SCustomClusterNodes is a helper method that will add nodes to the custom RKE2/K3S custom cluster.
func AddRKE2K3SCustomClusterNodes(client *rancher.Client, cluster *v1.SteveAPIObject, nodes []*nodes.Node, rolesPerNode []string) error {
	clusterStatus := &apiv1.ClusterStatus{}
	err := v1.ConvertToK8sType(cluster.Status, clusterStatus)
	if err != nil {
		return err
	}

	token, err := tokenregistration.GetRegistrationToken(client, clusterStatus.ClusterName)
	if err != nil {
		return err
	}

	var command string
	for key, node := range nodes {
		logrus.Infof("Adding node %s to cluster %s", node.NodeID, cluster.Name)
		if strings.Contains(rolesPerNode[key], "windows") {
			command = fmt.Sprintf("powershell.exe %s -Address %s", token.InsecureWindowsNodeCommand, node.PublicIPAddress)
		} else {
			command = fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, rolesPerNode[key], node.PublicIPAddress)
		}

		output, err := node.ExecuteCommand(command)
		if err != nil {
			return err
		}

		logrus.Infof(output)
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		clusterResp, err := client.Steve.SteveType(stevetypes.Provisioning).ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.ObjectMeta.State.Name == active &&
			nodestat.AllMachineReady(client, cluster.ID, defaults.ThirtyMinuteTimeout) == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteRKE2K3SCustomClusterNodes is a method that will delete nodes from the custom RKE2/K3S custom cluster.
func DeleteRKE2K3SCustomClusterNodes(client *rancher.Client, clusterID string, cluster *v1.SteveAPIObject, nodesToDelete []*nodes.Node) error {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	nodesSteveObjList, err := steveclient.SteveType("node").List(nil)
	if err != nil {
		return err
	}

	for _, nodeToDelete := range nodesToDelete {
		for _, node := range nodesSteveObjList.Data {
			snippedIP := strings.Split(node.Annotations[internalIP], ",")[0]

			if snippedIP == nodeToDelete.PrivateIPAddress {
				machine, err := client.Steve.SteveType(machineSteveResourceType).ByID(namespace + "/" + node.Annotations[machineNameAnnotation])
				if err != nil {
					return err
				}

				logrus.Infof("Deleting node %s from cluster %s", nodeToDelete.NodeID, cluster.Name)
				err = client.Steve.SteveType(machineSteveResourceType).Delete(machine)
				if err != nil {
					return err
				}

				err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
					_, err = client.Steve.SteveType(machineSteveResourceType).ByID(machine.ID)
					if err != nil {
						logrus.Infof("Node has successfully been deleted!")
						return true, nil
					}
					return false, nil
				})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// AddRKE1CustomClusterNodes is a method that will add nodes to the custom RKE1 custom cluster.
func AddRKE1CustomClusterNodes(client *rancher.Client, cluster *management.Cluster, nodes []*nodes.Node, rolesPerNode []string) error {
	token, err := tokenregistration.GetRegistrationToken(client, cluster.ID)
	if err != nil {
		return err
	}

	var command string
	for key, node := range nodes {
		logrus.Infof("Adding node %s to cluster %s", node.NodeID, cluster.Name)
		command = fmt.Sprintf("%s %s --address %s", token.NodeCommand, rolesPerNode[key], node.PublicIPAddress)

		output, err := node.ExecuteCommand(command)
		if err != nil {
			return err
		}

		logrus.Infof(output)
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Management.Cluster.ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.State == active &&
			nodestat.AllManagementNodeReady(client, cluster.ID, defaults.ThirtyMinuteTimeout) == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteRKE1CustomClusterNodes is a helper method that will delete nodes from the custom RKE1 custom cluster.
func DeleteRKE1CustomClusterNodes(client *rancher.Client, cluster *management.Cluster, nodesToDelete []*nodes.Node) error {
	nodes, err := client.Management.Node.ListAll(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": cluster.ID,
	}})
	if err != nil {
		return err
	}

	for _, nodeToDelete := range nodesToDelete {
		for _, node := range nodes.Data {
			if node.Annotations[rke1ExternalIP] == nodeToDelete.PublicIPAddress {
				machine, err := client.Management.Node.ByID(node.ID)
				if err != nil {
					return err
				}

				logrus.Infof("Deleting node %s from cluster %s", nodeToDelete.NodeID, cluster.Name)
				err = client.Management.Node.Delete(machine)
				if err != nil {
					return err
				}

				err = kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.ThirtyMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
					_, err = client.Management.Node.ByID(machine.ID)
					if err != nil {
						logrus.Infof("Node has successfully been deleted!")
						return true, nil
					}
					return false, nil
				})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// CreateProvisioningRKE1ClusterWithClusterTemplate provisions an rke1 cluster by using the rke1 template and revision ID and other values from the template.
func CreateProvisioningRKE1ClusterWithClusterTemplate(client *rancher.Client, templateID, revisionID string, nodesAndRoles []provisioninginput.NodePools, nodeTemplate *nodetemplates.NodeTemplate, answers *management.Answer) (*management.Cluster, error) {
	clusterName := namegen.AppendRandomString("rke1cluster-template-")

	rke1Cluster := &management.Cluster{
		DockerRootDir:                 "/var/lib/docker",
		Name:                          namegen.AppendRandomString("rketemplate-cluster-"),
		ClusterTemplateID:             templateID,
		ClusterTemplateRevisionID:     revisionID,
		ClusterTemplateAnswers:        answers,
		RancherKubernetesEngineConfig: nil,
	}
	clusterResp, err := shepherdclusters.CreateRKE1Cluster(client, rke1Cluster)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	var nodeRoles []nodepools.NodeRoles
	for _, nodes := range nodesAndRoles {
		nodeRoles = append(nodeRoles, nodes.NodeRoles)
	}
	_, err = nodepools.NodePoolSetup(client, nodeRoles, clusterResp.ID, nodeTemplate.ID)
	if err != nil {
		return nil, err
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	return createdCluster, err
}
