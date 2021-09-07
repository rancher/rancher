package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/creasty/defaults"
)

var (
	instance   *configuration = nil
	configPath                = os.Getenv("CONFIG_PATH")
)

type RancherServerConfig struct {
	CattleTestURL     string `json:"cattleTestURL"`
	AdminToken        string `json:"adminToken"`
	UserToken         string `json:"userToken,omitempty"`
	CNI               string `json:"cni" default:"calico"`
	KubernetesVersion string `json:"kubernetesVersion"`
	NodeRoles         string `json:"nodeRoles,omitempty"`
	DefaultNamespace  string `default:"fleet-default"`
	Insecure          *bool  `json:"insecure" default:"true"`
	CAFile            string `json:"caFile" default:""`
	CACerts           string `json:"caCerts" default:""`
}

type DigitalOceanConfig struct {
	DOAccessKey string `json:"doAccessKey"`
	DOImage     string `json:"doImage" default:"ubuntu-20-04-x64"`
	DORegion    string `json:"doRegion" default:"nyc3"`
	DOSize      string `json:"doSize" default:"s-2vcpu-4gb"`
}

type AWSConfig struct {
	AWSInstanceType    string `json:"awsInstanceType,omitempty"`
	AWSRegion          string `json:"awsRegion,omitempty"`
	AWSRegionAZ        string `json:"awsRegionAZ,omitempty"`
	AWSAMI             string `json:"awsAMI,omitempty"`
	AWSSecurityGroup   string `json:"awsSecurityGroups,omitempty"`
	AWSAccessKeyID     string `json:"awsAccessKeyID,omitempty"`
	AWSSecretAccessKey string `json:"awsSecretAccessKey,omitempty"`
	AWSSSHKeyName      string `json:"awsSSHKeyName,omitempty"`
	AWSCICDInstanceTag string `json:"awsCICDInstanceTag,omitempty"`
	AWSIAMProfile      string `json:"awsIAMProfile,omitempty"`
	AWSUser            string `json:"awsUser,omitempty"`
	AWSVolumeSize      int64  `json:"awsVolumeSize,omitempty"`
	AWSSSHKeyPath      string `json:"awsSSHKeyPath,omitempty"`
}

type configuration struct {
	RancherServerConfig RancherServerConfig `json:"rancherServerConfig"`
	DigitalOceanConfig  DigitalOceanConfig  `json:"digitalOceanConfig"`
	AWSConfig           AWSConfig           `json:"awsConfig"`
}

func init() {
	var conf configuration
	if configPath != "" {
		configContents, err := ioutil.ReadFile(configPath)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(configContents, &conf)
		if err != nil {
			panic(err)
		}
	}

	if err := defaults.Set(&conf); err != nil {
		panic(err)
	}
	instance = &conf

}

func GetInstance() *configuration {
	return instance
}
