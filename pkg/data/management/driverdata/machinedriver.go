// Package driverdata holds static node driver and KEv2 operator data.
// It must only depend on API types so that low-level packages such as
// pkg/auth/audit can import it.
package driverdata

const (
	AlibabaDriver      = "aliyunecs"
	Amazonec2driver    = "amazonec2"
	Azuredriver        = "azure"
	DigitalOceandriver = "digitalocean"
	ExoscaleDriver     = "exoscale"
	HarvesterDriver    = "harvester"
	Linodedriver       = "linode"
	NutanixDriver      = "nutanix"
	OCIDriver          = "oci"
	OTCDriver          = "otc"
	OpenstackDriver    = "openstack"
	PacketDriver       = "packet"
	PhoenixNAPDriver   = "pnap"
	PodDriver          = "pod"
	RackspaceDriver    = "rackspace"
	SoftLayerDriver    = "softlayer"
	Vmwaredriver       = "vmwarevsphere"
	GoogleDriver       = "google"
	OutscaleDriver     = "outscale"
)

// DriverDataConfig contains driver‑specific metadata that is parsed as
// annotations on the corresponding NodeDriver object.
// FileToFieldAliases field maps `Schema field => driver field`
type DriverDataConfig struct {
	FileToFieldAliases       map[string]string
	PublicCredentialFields   []string
	PrivateCredentialFields  []string
	PasswordFields           []string
	OptionalCredentialFields []string
	Defaults                 map[string]string
}

var DriverData = map[string]DriverDataConfig{
	AlibabaDriver: {
		FileToFieldAliases: map[string]string{"sshKeyContents": "sshKeypath"},
	},
	Amazonec2driver: {
		FileToFieldAliases:      map[string]string{"sshKeyContents": "sshKeypath", "userdata": "userdata"},
		PublicCredentialFields:  []string{"accessKey"},
		PrivateCredentialFields: []string{"secretKey"},
	},
	Azuredriver: {
		FileToFieldAliases:       map[string]string{"customData": "customData"},
		PublicCredentialFields:   []string{"clientId", "subscriptionId", "tenantId", "environment"},
		PrivateCredentialFields:  []string{"clientSecret"},
		OptionalCredentialFields: []string{"tenantId"},
	},
	DigitalOceandriver: {
		FileToFieldAliases:      map[string]string{"sshKeyContents": "sshKeyPath", "userdata": "userdata"},
		PrivateCredentialFields: []string{"accessToken"},
	},
	ExoscaleDriver: {
		FileToFieldAliases:      map[string]string{"sshKey": "sshKey", "userdata": "userdata"},
		PublicCredentialFields:  []string{"apiKey"},
		PrivateCredentialFields: []string{"apiSecretKey"},
	},
	HarvesterDriver: {
		PublicCredentialFields:   []string{"clusterType", "clusterId"},
		PrivateCredentialFields:  []string{"kubeconfigContent"},
		OptionalCredentialFields: []string{"clusterId"},
		Defaults:                 map[string]string{"clusterType": "imported"},
	},
	Linodedriver: {
		PrivateCredentialFields: []string{"token"},
		PasswordFields:          []string{"rootPass"},
	},
	NutanixDriver: {
		PublicCredentialFields:  []string{"endpoint", "username", "port"},
		PrivateCredentialFields: []string{"password"},
	},
	OCIDriver: {
		PublicCredentialFields:  []string{"tenancyId", "userId", "fingerprint"},
		PrivateCredentialFields: []string{"privateKeyContents"},
		PasswordFields:          []string{"privateKeyPassphrase"},
	},
	OTCDriver: {
		FileToFieldAliases:      map[string]string{"privateKeyFile": "privateKeyFile"},
		PublicCredentialFields:  []string{"accessKey", "username"},
		PrivateCredentialFields: []string{"secretKey", "password", "token"},
	},
	OpenstackDriver: {
		FileToFieldAliases:      map[string]string{"cacert": "cacert", "privateKeyFile": "privateKeyFile", "userDataFile": "userDataFile"},
		PrivateCredentialFields: []string{"password"},
	},
	PacketDriver: {
		FileToFieldAliases:      map[string]string{"userdata": "userdata"},
		PrivateCredentialFields: []string{"apiKey"},
	},
	PhoenixNAPDriver: {
		PublicCredentialFields:  []string{"clientIdentifier"},
		PrivateCredentialFields: []string{"clientSecret"},
	},
	PodDriver: {
		FileToFieldAliases: map[string]string{"userdata": "userdata"},
	},
	RackspaceDriver: {
		PrivateCredentialFields: []string{"apiKey"},
	},
	SoftLayerDriver: {
		PrivateCredentialFields: []string{"apiKey"},
	},
	Vmwaredriver: {
		FileToFieldAliases:      map[string]string{"cloudConfig": "cloud-config"},
		PublicCredentialFields:  []string{"username", "vcenter", "vcenterPort"},
		PrivateCredentialFields: []string{"password"},
		Defaults:                map[string]string{"vcenterPort": "443"},
	},
	GoogleDriver: {
		FileToFieldAliases:      map[string]string{"authEncodedJson": "authEncodedJson", "userdata": "userdata"},
		PrivateCredentialFields: []string{"authEncodedJson"},
	},
	OutscaleDriver: {
		PublicCredentialFields:  []string{"accessKey", "region"},
		PrivateCredentialFields: []string{"secretKey"},
	},
}
