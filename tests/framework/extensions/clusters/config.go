package clusters

const ConfigurationFileKey = "clusters"

type Config struct {
	NodesAndRoles string `yaml:"nodesAndRoles" default:""`
}
