package nodetemplates

// The json/yaml config key for the VSphere node template config
const VmwareVsphereNodeTemplateConfigurationFileKey = "vmwarevsphereConfig"

// VmwareVsphereNodeTemplateConfig is configuration need to create a VSphere node template
type VmwareVsphereNodeTemplateConfig struct {
	Cfgparam              []string `json:"cfgparam" yaml:"cfgparam"`
	CloneFrom             string   `json:"cloneFrom" yaml:"cloneFrom"`
	CloudConfig           string   `json:"cloudConfig" yaml:"cloudConfig"`
	Cloundinit            string   `json:"cloundinit" yaml:"cloundinit"`
	ContentLibrary        string   `json:"contentLibrary" yaml:"contentLibrary"`
	CPUCount              string   `json:"cpuCount" yaml:"cpuCount"`
	CreationType          string   `json:"creationType" yaml:"creationType"`
	CustomAttribute       []string `json:"customAttribute" yaml:"customAttribute"`
	DataCenter            string   `json:"dataCenter" yaml:"dataCenter"`
	DataStore             string   `json:"dataStore" yaml:"dataStore"`
	DatastoreCluster      string   `json:"datastoreCluster" yaml:"datastoreCluster"`
	DiskSize              string   `json:"diskSize" yaml:"diskSize"`
	Folder                string   `json:"folder" yaml:"folder"`
	HostSystem            string   `json:"hostSystem" yaml:"hostSystem"`
	MemorySize            string   `json:"memorySize" yaml:"memorySize"`
	Network               []string `json:"network" yaml:"network"`
	OS                    string   `json:"os" yaml:"os"`
	Password              string   `json:"password" yaml:"password"`
	Pool                  string   `json:"pool" yaml:"pool"`
	SSHPassword           string   `json:"sshPassword" yaml:"sshPassword"`
	SSHPort               string   `json:"sshPort" yaml:"sshPort"`
	SSHUser               string   `json:"sshUser" yaml:"sshUser"`
	SSHUserGroup          string   `json:"sshUserGroup" yaml:"sshUserGroup"`
	Tag                   []string `json:"tag" yaml:"tag"`
	Username              string   `json:"username" yaml:"username"`
	VappIpallocationplicy string   `json:"vappIpallocationplicy" yaml:"vappIpallocationplicy"`
	VappIpprotocol        string   `json:"vappIpprotocol" yaml:"vappIpprotocol"`
	VappProperty          []string `json:"vappProperty" yaml:"vappProperty"`
	VappTransport         string   `json:"vappTransport" yaml:"vappTransport"`
	Vcenter               string   `json:"vcenter" yaml:"vcenter"`
	VcenterPort           string   `json:"vcenterPort" yaml:"vcenterPort"`
}
