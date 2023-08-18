package provisioning

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/framework/clients/corral"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/sirupsen/logrus"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"
	k3sHardening "github.com/rancher/rancher/tests/framework/extensions/hardening/k3s"
	rke2Hardening "github.com/rancher/rancher/tests/framework/extensions/hardening/rke2"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/rke1/nodetemplates"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/tokenregistration"
	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/rancher/rancher/tests/framework/pkg/wait"

	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "fleet-default"

	rke2k3sAirgapCustomCluster           = "rke2k3sairgapcustomcluster"
	rke2k3sNodeCorralName                = "rke2k3sregisterNode"
	corralPackageAirgapCustomClusterName = "airgapCustomCluster"
	rke1AirgapCustomCluster              = "rke1airgapcustomcluster"
	rke1NodeCorralName                   = "rke1registerNode"
)

// CreateProvisioningCluster provisions a non-rke1 cluster, then runs verify checks
func CreateProvisioningCluster(client *rancher.Client, provider Provider, clustersConfig *clusters.ClusterConfig, hostnameTruncation []machinepools.HostnameTruncation) (*v1.SteveAPIObject, error) {
	cloudCredential, err := provider.CloudCredFunc(client)
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
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	if err != nil {
		return nil, err
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

	machinePools := machinepools.CreateAllMachinePools(*clustersConfig.NodesAndRoles, machineConfigResp, hostnameTruncation)
	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, machinePools, cloudCredential.ID)

	for _, truncatedPool := range hostnameTruncation {
		if truncatedPool.PoolNameLengthLimit > 0 || truncatedPool.ClusterNameLengthLimit > 0 {
			cluster.GenerateName = "t-"
			if truncatedPool.ClusterNameLengthLimit > 0 {
				cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit = truncatedPool.ClusterNameLengthLimit
			}
			break
		}
	}

	_, err = clusters.CreateK3SRKE2Cluster(client, cluster)
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

	createdCluster, err := adminClient.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(namespace + "/" + clusterName)
	return createdCluster, err
}

// CreateProvisioningCustomCluster provisions a non-rke1 cluster using a 3rd party client for its nodes, then runs verify checks
func CreateProvisioningCustomCluster(client *rancher.Client, externalNodeProvider ExternalNodeProvider, clustersConfig *clusters.ClusterConfig) (*v1.SteveAPIObject, error) {
	setLogrusFormatter()
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, nodes := range *clustersConfig.NodesAndRoles {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}
		if nodes.Windows {
			finalRoleCommand += " --windows"
		}
		quantityPerPool = append(quantityPerPool, nodes.Quantity)
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
		for i := int32(0); i < nodes.Quantity; i++ {
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

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
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

	checkFunc := clusters.IsProvisioningClusterReady
	var command string
	for key, node := range nodes {
		logrus.Infof("Execute Registration Command for node %s", node.NodeID)
		if strings.Contains(rolesPerNode[key], "windows") {
			continue
		}
		logrus.Infof("Linux pool detected, using bash")
		command := fmt.Sprintf("%s %s --address %s", token.InsecureNodeCommand, rolesPerNode[key], node.PublicIPAddress)
		logrus.Infof("Command: %s", command)
		output, err := node.ExecuteCommand(command)
		if err != nil {
			return nil, err
		}
		logrus.Infof(output)
	}
	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return nil, err
	}

	for key, node := range nodes {
		if strings.Contains(rolesPerNode[key], "windows") {
			logrus.Infof("Windows pool detected, using powershell.exe")
			command = fmt.Sprintf("powershell.exe %s -Address %s", token.InsecureWindowsNodeCommand, node.PublicIPAddress)
			logrus.Infof("Command: %s", command)
			output, err := node.ExecuteCommand(command)
			if err != nil {
				return nil, err
			}
			logrus.Infof(output)
		}
	}

	if clustersConfig.Hardened {
		var hardenCluster *apiv1.Cluster
		if strings.Contains(clustersConfig.KubernetesVersion, clusters.K3SClusterType.String()) {
			err = k3sHardening.HardeningNodes(client, clustersConfig.Hardened, nodes, rolesPerNode, clustersConfig.KubernetesVersion)
			if err != nil {
				return nil, err
			}

			hardenCluster = clusters.HardenK3SClusterConfig(clusterName, namespace, clustersConfig, nil, "")
		} else {
			err = rke2Hardening.HardeningNodes(client, clustersConfig.Hardened, nodes, rolesPerNode)
			if err != nil {
				return nil, err
			}

			hardenCluster = clusters.HardenRKE2ClusterConfig(clusterName, namespace, clustersConfig, nil, "")
		}

		_, err := clusters.UpdateK3SRKE2Cluster(client, clusterResp, hardenCluster)
		if err != nil {
			return nil, err
		}

		logrus.Infof("Cluster has been successfully hardened!")
	}

	createdCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(namespace + "/" + clusterName)
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
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	if client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	nodePools, err := nodepools.NodePoolSetup(client, *clustersConfig.NodesAndRolesRKE1, clusterResp.ID, nodeTemplate.ID)
	if err != nil {
		return nil, err
	}

	if clustersConfig.DrainBeforeDelete {
		nodePools.DrainBeforeDelete = clustersConfig.DrainBeforeDelete
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	return createdCluster, err
}

// CreateProvisioningRKE1CustomCluster provisions an rke1 cluster using a 3rd party client for its nodes, then runs verify checks
func CreateProvisioningRKE1CustomCluster(client *rancher.Client, externalNodeProvider ExternalNodeProvider, clustersConfig *clusters.ClusterConfig) (*management.Cluster, []*nodes.Node, error) {
	setLogrusFormatter()
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}
	for _, nodes := range *clustersConfig.NodesAndRolesRKE1 {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}

		quantityPerPool = append(quantityPerPool, int32(nodes.Quantity))
		rolesPerPool = append(rolesPerPool, finalRoleCommand)
		for i := int64(0); i < nodes.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
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
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
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

	for key, node := range nodes {
		logrus.Infof("Execute Registration Command for node %s", node.NodeID)
		command := fmt.Sprintf("%s %s --address %s --internal-address %s",
			token.NodeCommand, rolesPerNode[key], node.PublicIPAddress, node.PrivateIPAddress)

		output, err := node.ExecuteCommand(command)
		if err != nil {
			return nil, nil, err
		}
		logrus.Infof(output)
	}

	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)

	return createdCluster, nodes, err
}

// CreateProvisioningAirgapCustomCluster provisions a non-rke1 cluster using corral to gather its nodes, then runs verify checks
func CreateProvisioningAirgapCustomCluster(client *rancher.Client, clustersConfig *clusters.ClusterConfig, corralPackages *corral.CorralPackages) (*v1.SteveAPIObject, error) {
	setLogrusFormatter()
	rolesPerNode := map[int32]string{}
	for _, nodes := range *clustersConfig.NodesAndRoles {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}
		if nodes.Windows {
			finalRoleCommand += " --windows"
		}

		rolesPerNode[nodes.Quantity] = finalRoleCommand
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	clusterName := namegen.AppendRandomString(rke2k3sAirgapCustomCluster)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, clustersConfig, nil, "")

	clusterResp, err := clusters.CreateK3SRKE2Cluster(client, cluster)
	if err != nil {
		return nil, err
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	customCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(clusterResp.ID)
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
	for quantity, roles := range rolesPerNode {
		err = corral.UpdateCorralConfig("node_count", fmt.Sprint(quantity))
		if err != nil {
			return nil, err
		}

		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, roles)
		logrus.Infof("registration command is %s", command)
		err = corral.UpdateCorralConfig("registration_command", command)
		if err != nil {
			return nil, err
		}

		corralName := namegen.AppendRandomString(rke2k3sNodeCorralName)
		_, err = corral.CreateCorral(
			client.Session,
			corralName,
			corralPackages.CorralPackageImages[corralPackageAirgapCustomClusterName],
			corralPackages.HasDebug,
			corralPackages.HasCleanup,
		)
		if err != nil {
			return nil, err
		}
	}

	createdCluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(namespace + "/" + clusterName)
	return createdCluster, err
}

// CreateProvisioningRKE1AirgapCustomCluster provisions an rke1 cluster using corral to gather its nodes, then runs verify checks
func CreateProvisioningRKE1AirgapCustomCluster(client *rancher.Client, clustersConfig *clusters.ClusterConfig, corralPackages *corral.CorralPackages) (*management.Cluster, error) {
	setLogrusFormatter()
	clusterName := namegen.AppendRandomString(rke1AirgapCustomCluster)
	rolesPerNode := map[int64]string{}
	for _, nodes := range *clustersConfig.NodesAndRolesRKE1 {
		var finalRoleCommand string
		if nodes.ControlPlane {
			finalRoleCommand += " --controlplane"
		}
		if nodes.Etcd {
			finalRoleCommand += " --etcd"
		}
		if nodes.Worker {
			finalRoleCommand += " --worker"
		}

		rolesPerNode[nodes.Quantity] = finalRoleCommand
	}

	if clustersConfig.PSACT == string(provisioninginput.RancherBaseline) {
		err := clusters.CreateRancherBaselinePSACT(client, clustersConfig.PSACT)
		if err != nil {
			return nil, err
		}
	}

	cluster := clusters.NewRKE1ClusterConfig(clusterName, client, clustersConfig)
	clusterResp, err := clusters.CreateRKE1Cluster(client, cluster)
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

	logrus.Infof("Register Custom Cluster Through Corral")
	for quantity, roles := range rolesPerNode {
		err = corral.UpdateCorralConfig("node_count", fmt.Sprint(quantity))
		if err != nil {
			return nil, err
		}

		command := fmt.Sprintf("%s %s", token.NodeCommand, roles)
		logrus.Infof("registration command is %s", command)
		err = corral.UpdateCorralConfig("registration_command", command)
		if err != nil {
			return nil, err
		}

		corralName := namegen.AppendRandomString(rke1NodeCorralName)

		_, err = corral.CreateCorral(
			client.Session,
			corralName,
			corralPackages.CorralPackageImages[corralPackageAirgapCustomClusterName],
			corralPackages.HasDebug,
			corralPackages.HasCleanup,
		)
		if err != nil {
			return nil, err
		}
	}
	createdCluster, err := client.Management.Cluster.ByID(clusterResp.ID)
	return createdCluster, err
}

func setLogrusFormatter() {
	formatter := &logrus.TextFormatter{}
	formatter.DisableQuote = true
	logrus.SetFormatter(formatter)
}
