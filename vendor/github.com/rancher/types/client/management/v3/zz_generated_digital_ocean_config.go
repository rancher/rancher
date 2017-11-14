package client

const (
	DigitalOceanConfigType                   = "digitalOceanConfig"
	DigitalOceanConfigFieldAccessToken       = "accessToken"
	DigitalOceanConfigFieldBackups           = "backups"
	DigitalOceanConfigFieldImage             = "image"
	DigitalOceanConfigFieldIpv6              = "ipv6"
	DigitalOceanConfigFieldPrivateNetworking = "privateNetworking"
	DigitalOceanConfigFieldRegion            = "region"
	DigitalOceanConfigFieldSSHKeyFingerprint = "sshKeyFingerprint"
	DigitalOceanConfigFieldSSHKeyPath        = "sshKeyPath"
	DigitalOceanConfigFieldSSHPort           = "sshPort"
	DigitalOceanConfigFieldSSHUser           = "sshUser"
	DigitalOceanConfigFieldSize              = "size"
	DigitalOceanConfigFieldUserdata          = "userdata"
)

type DigitalOceanConfig struct {
	AccessToken       string `json:"accessToken,omitempty"`
	Backups           *bool  `json:"backups,omitempty"`
	Image             string `json:"image,omitempty"`
	Ipv6              *bool  `json:"ipv6,omitempty"`
	PrivateNetworking *bool  `json:"privateNetworking,omitempty"`
	Region            string `json:"region,omitempty"`
	SSHKeyFingerprint string `json:"sshKeyFingerprint,omitempty"`
	SSHKeyPath        string `json:"sshKeyPath,omitempty"`
	SSHPort           string `json:"sshPort,omitempty"`
	SSHUser           string `json:"sshUser,omitempty"`
	Size              string `json:"size,omitempty"`
	Userdata          string `json:"userdata,omitempty"`
}
