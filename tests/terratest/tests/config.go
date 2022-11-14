package tests

import "github.com/rancher/rancher/tests/terratest/models"

type GoogleAuthEncodedJSON struct {
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url" yaml:"auth_provider_x509_cert_url"`
	AuthURI                 string `json:"auth_uri" yaml:"auth_uri"`
	ClientEmail             string `json:"client_email" yaml:"client_email"`
	ClientID                string `json:"client_id" yaml:"client_id"`
	ClientX509CertURL       string `json:"client_x509_cert_url" yaml:"client_x509_cert_url"`
	PrivateKey              string `json:"private_key" yaml:"private_key"`
	PrivateKeyID            string `json:"private_key_id" yaml:"private_key_id"`
	ProjectID               string `json:"project_id" yaml:"project_id"`
	TokenURI                string `json:"token_uri" yaml:"token_uri"`
	Type                    string `json:"type" yaml:"type"`
}

type TerraformConfig struct {
	Ami                                 string `json:"ami" yaml:"ami"`
	AvailabilityZones                   string `json:"availabilityZones" yaml:"availabilityZones"`
	AWSAccessKey                        string `json:"awsAccessKey" yaml:"awsAccessKey"`
	AWSInstanceType                     string `json:"awsInstanceType" yaml:"awsInstanceType"`
	AWSRootSize                         string `json:"awsRootSize" yaml:"awsRootSize"`
	AWSSecretKey                        string `json:"awsSecretKey" yaml:"awsSecretKey"`
	AWSSecurityGroupName                string `json:"awsSecurityGroupName" yaml:"awsSecurityGroupName"`
	AWSSecurityGroups                   string `json:"awsSecurityGroups" yaml:"awsSecurityGroups"`
	AWSSubnetID                         string `json:"awsSubnetID" yaml:"awsSubnetID"`
	AWSSubnets                          string `json:"awsSubnets" yaml:"awsSubnets"`
	AWSVpcID                            string `json:"awsVpcID" yaml:"awsVpcID"`
	AWSZoneLetter                       string `json:"awsZoneLetter" yaml:"awsZoneLetter"`
	AzureClientID                       string `json:"azureClientID" yaml:"azureClientID"`
	AzureClientSecret                   string `json:"azureClientSecret" yaml:"azureClientSecret"`
	AzureSubscriptionID                 string `json:"azureSubscriptionID" yaml:"azureSubscriptionID"`
	CloudCredentialName                 string `json:"cloudCredentialName" yaml:"cloudCredentialName"`
	ClusterName                         string `json:"clusterName" yaml:"clusterName"`
	DefaultClusterRoleForProjectMembers string `json:"defaultClusterRoleForProjectMembers" yaml:"defaultClusterRoleForProjectMembers"`
	EnableNetworkPolicy   string `json:"enableNetworkPolicy" yaml:"enableNetworkPolicy"`
	GKENetwork            string `json:"gkeNetwork" yaml:"gkeNetwork"`
	GKEProjectID          string `json:"gkeProjectID" yaml:"gkeProjectID"`
	GKESubnetwork         string `json:"gkeSubnetwork" yaml:"gkeSubnetwork"`
	GoogleAuthEncodedJSON GoogleAuthEncodedJSON
	HostnamePrefix        string `json:"hostnamePrefix" yaml:"hostnamePrefix"`
	LinodeImage           string `json:"linodeImage" yaml:"linodeImage"`
	LinodeRootPass        string `json:"linodeRootPass" yaml:"linodeRootPass"`
	LinodeToken           string `json:"linodeToken" yaml:"linodeToken"`
	MachineConfigName     string `json:"machineConfigName" yaml:"machineConfigName"`
	NetworkPlugin         string `json:"networkPlugin" yaml:"networkPlugin"`
	NodeTemplateName      string `json:"nodeTemplateName" yaml:"nodeTemplateName"`
	OSDiskSizeGB          string `json:"osDiskSizeGB" yaml:"osDiskSizeGB"`
	PrivateAccess         string `json:"privateAccess" yaml:"privateAccess"`
	PublicAccess          string `json:"publicAccess" yaml:"publicAccess"`
	Region                string `json:"region" yaml:"region"`
	ResourceGroup         string `json:"resourceGroup" yaml:"resourceGroup"`
	ResourceLocation      string `json:"resourceLocation" yaml:"resourceLocation"`
	VMSize                string `json:"vmSize" yaml:"vmSize"`
}

type TerratestConfig struct {
	KubernetesVersion         string            `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	NodeCount                 int64             `json:"nodeCount" yaml:"nodeCount"`
	Nodepools                 []models.Nodepool `json:"nodepools" yaml:"nodepools"`
	ScaledDownNodeCount       int64             `json:"scaledDownNodeCount" yaml:"scaledDownNodeCount"`
	ScaledDownNodepools       []models.Nodepool `json:"scaledDownNodepools" yaml:"scaledDownNodepools"`
	ScaledUpNodeCount         int64             `json:"scaledUpNodeCount" yaml:"scaledUpNodeCount"`
	ScaledUpNodepools         []models.Nodepool `json:"scaledUpNodepools" yaml:"scaledUpNodepools"`
	UpgradedKubernetesVersion string            `json:"upgradedKubernetesVersion" yaml:"upgradedKubernetesVersion"`
}
