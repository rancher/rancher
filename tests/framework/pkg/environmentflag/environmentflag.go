package environmentflag

import (
	"strings"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/utils/strings/slices"
)

const (
	ConfigurationFileKey = "flags"
)

// EnvironmentFlags is a map of environment flags. The key is the flag enum and the value is true if the flag is set.
type EnvironmentFlags map[EnvironmentFlag]bool

type Config struct {
	DesiredFlags string `json:"desiredflags" yaml:"desiredflags" default:""`
}

// NewEnvironmentFlags creates a new EnvironmentFlags.
func NewEnvironmentFlags() EnvironmentFlags {
	return make(EnvironmentFlags)
}

// LoadEnvironmentFlags loads the environment flags from the configuration file.
// If the flags field does not exist, it returns an empty map.
func LoadEnvironmentFlags(configurationFileKey string, e EnvironmentFlags) {
	flagsConfig := new(Config)
	config.LoadConfig(configurationFileKey, flagsConfig)

	flags := strings.Split(strings.ToLower(flagsConfig.DesiredFlags), "|")

	for i := EnvironmentFlag(0); i < environmentFlagLastItem; i++ {
		if slices.Contains(flags, strings.ToLower(i.String())) {
			e[EnvironmentFlag(i)] = true
		}
	}
}

// GetValue returns the value of the flag.
// If the flag is not set, it returns false.
func (e EnvironmentFlags) GetValue(flag EnvironmentFlag) bool {
	return e[flag]
}
