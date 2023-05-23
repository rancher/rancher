package corralha

// The json/yaml config key for the corral package to be build ..
const (
	CorralRancherHAConfigConfigurationFileKey = "corralRancherHA"
)

// CorralPackages is a struct that has the path to the packages
type CorralRancherHA struct {
	Name string `json:"name" yaml:"name"`
}
