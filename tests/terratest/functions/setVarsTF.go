package functions

import (
	"fmt"
	"os"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/terratest/tests"
)

func SetVarsTF(module string) (done bool, err error) {

	rancherConfig := new(rancher.Config)
	config.LoadConfig("rancher", rancherConfig)

	terraformConfig := new(tests.TerraformConfig)
	config.LoadConfig("terraform", terraformConfig)

	switch {
	case module == "aks":
		file, err := os.Create("../../modules/hosted/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\nazure_client_id = \"%s\"\nazure_client_secret = \"%s\"\nazure_subscription_id = \"%s\"\ncluster_name = \"%s\"\nresource_group = \"%s\"\nresource_location = \"%s\"\ndns_prefix = \"%s\"\nnetwork_plugin = \"%s\"\navailability_zones = [%s]\nos_disk_size_gb = \"%s\"\nvm_size = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.AzureClientID, terraformConfig.AzureClientSecret, terraformConfig.AzureSubscriptionID, terraformConfig.ClusterName, terraformConfig.ResourceGroup, terraformConfig.ResourceLocation, terraformConfig.HostnamePrefix, terraformConfig.NetworkPlugin, terraformConfig.AvailabilityZones, terraformConfig.OSDiskSizeGB, terraformConfig.VMSize)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "eks":
		file, err := os.Create("../../modules/hosted/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\naws_access_key = \"%s\"\naws_secret_key = \"%s\"\naws_instance_type = \"%s\"\naws_region = \"%s\"\naws_subnets = [%s]\naws_security_groups = [%s]\ncluster_name = \"%s\"\nhostname_prefix = \"%s\"\npublic_access = \"%s\"\nprivate_access = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.AWSAccessKey, terraformConfig.AWSSecretKey, terraformConfig.AWSInstanceType, terraformConfig.Region, terraformConfig.AWSSubnets, terraformConfig.AWSSecurityGroups, terraformConfig.ClusterName, terraformConfig.HostnamePrefix, terraformConfig.PublicAccess, terraformConfig.PrivateAccess)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "gke":
		file, err := os.Create("../../modules/hosted/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ngoogle_auth_encoded_json = \"%s\"\ncloud_credential_name = \"%s\"\ncluster_name = \"%s\"\ngke_region = \"%s\"\ngke_project_id = \"%s\"\ngke_network = \"%s\"\ngke_subnetwork = \"%s\"\nhostname_prefix = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.GoogleAuthEncodedJSON, terraformConfig.CloudCredentialName, terraformConfig.ClusterName, terraformConfig.Region, terraformConfig.GKEProjectID, terraformConfig.GKENetwork, terraformConfig.GKESubnetwork, terraformConfig.HostnamePrefix)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "ec2_k3s":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\naws_access_key = \"%s\"\naws_secret_key = \"%s\"\naws_ami = \"%s\"\naws_region = \"%s\"\naws_security_group_name = \"%s\"\naws_subnet_id = \"%s\"\naws_vpc_id = \"%s\"\naws_zone_letter = \"%s\"\nmachine_config_name = \"%s\"\ncluster_name = \"%s\"\nenable_network_policy = \"%s\"\ndefault_cluster_role_for_project_members = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.AWSAccessKey, terraformConfig.AWSSecretKey, terraformConfig.Ami, terraformConfig.Region, terraformConfig.AWSSecurityGroupName, terraformConfig.AWSSubnetID, terraformConfig.AWSVpcID, terraformConfig.AWSZoneLetter, terraformConfig.MachineConfigName, terraformConfig.ClusterName, terraformConfig.EnableNetworkPolicy, terraformConfig.DefaultClusterRoleForProjectMembers)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "ec2_rke1":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\naws_access_key = \"%s\"\naws_secret_key = \"%s\"\naws_ami = \"%s\"\naws_instance_type = \"%s\"\naws_region = \"%s\"\naws_security_group_name = \"%s\"\naws_subnet_id = \"%s\"\naws_vpc_id = \"%s\"\naws_zone_letter = \"%s\"\naws_root_size = \"%s\"\ncluster_name = \"%s\"\nnetwork_plugin = \"%s\"\nnode_template_name = \"%s\"\nhostname_prefix = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.AWSAccessKey, terraformConfig.AWSSecretKey, terraformConfig.Ami, terraformConfig.AWSInstanceType, terraformConfig.Region, terraformConfig.AWSSecurityGroupName, terraformConfig.AWSSubnetID, terraformConfig.AWSVpcID, terraformConfig.AWSZoneLetter, terraformConfig.AWSRootSize, terraformConfig.ClusterName, terraformConfig.NetworkPlugin, terraformConfig.NodeTemplateName, terraformConfig.HostnamePrefix)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "ec2_rke2":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\naws_access_key = \"%s\"\naws_secret_key = \"%s\"\naws_ami = \"%s\"\naws_region = \"%s\"\naws_security_group_name = \"%s\"\naws_subnet_id = \"%s\"\naws_vpc_id = \"%s\"\naws_zone_letter = \"%s\"\nmachine_config_name = \"%s\"\ncluster_name = \"%s\"\nenable_network_policy = \"%s\"\ndefault_cluster_role_for_project_members = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.AWSAccessKey, terraformConfig.AWSSecretKey, terraformConfig.Ami, terraformConfig.Region, terraformConfig.AWSSecurityGroupName, terraformConfig.AWSSubnetID, terraformConfig.AWSVpcID, terraformConfig.AWSZoneLetter, terraformConfig.MachineConfigName, terraformConfig.ClusterName, terraformConfig.EnableNetworkPolicy, terraformConfig.DefaultClusterRoleForProjectMembers)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "linode_k3s":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\nlinode_token = \"%s\"\nimage = \"%s\"\nregion = \"%s\"\nroot_pass = \"%s\"\nmachine_config_name = \"%s\"\ncluster_name = \"%s\"\nenable_network_policy = \"%s\"\ndefault_cluster_role_for_project_members = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.LinodeToken, terraformConfig.LinodeImage, terraformConfig.Region, terraformConfig.LinodeRootPass, terraformConfig.MachineConfigName, terraformConfig.ClusterName, terraformConfig.EnableNetworkPolicy, terraformConfig.DefaultClusterRoleForProjectMembers)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "linode_rke1":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\nlinode_token = \"%s\"\nimage = \"%s\"\nregion = \"%s\"\nroot_pass = \"%s\"\ncluster_name = \"%s\"\nnetwork_plugin = \"%s\"\nnode_template_name = \"%s\"\nhostname_prefix = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.LinodeToken, terraformConfig.LinodeImage, terraformConfig.Region, terraformConfig.LinodeRootPass, terraformConfig.ClusterName, terraformConfig.NetworkPlugin, terraformConfig.NodeTemplateName, terraformConfig.HostnamePrefix)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	case module == "linode_rke2":
		file, err := os.Create("../../modules/node_driver/" + module + "/terraform.tfvars")

		if err != nil {
			return false, err
		}

		defer file.Close()

		tfvarFile := fmt.Sprintf("rancher_api_url = \"https://%s\"\nrancher_admin_bearer_token = \"%s\"\ncloud_credential_name = \"%s\"\nlinode_token = \"%s\"\nimage = \"%s\"\nregion = \"%s\"\nroot_pass = \"%s\"\nmachine_config_name = \"%s\"\ncluster_name = \"%s\"\nenable_network_policy = \"%s\"\ndefault_cluster_role_for_project_members = \"%s\"", rancherConfig.Host, rancherConfig.AdminToken, terraformConfig.CloudCredentialName, terraformConfig.LinodeToken, terraformConfig.LinodeImage, terraformConfig.Region, terraformConfig.LinodeRootPass, terraformConfig.MachineConfigName, terraformConfig.ClusterName, terraformConfig.EnableNetworkPolicy, terraformConfig.DefaultClusterRoleForProjectMembers)
		_, err = file.WriteString(tfvarFile)

		if err != nil {
			return false, err
		}
		return true, nil

	default:
		return false, fmt.Errorf("invalid module provided")
	}

}
