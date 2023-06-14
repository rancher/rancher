//go:generate go run golang.org/x/tools/cmd/stringer -type=EnvironmentFlag -output=zz_environmentflags.go -linecomment
package environmentflag

// EnvironmentFlag is integer enum for environment flags.
type EnvironmentFlag int

// EnvironmentFlag represents a flag that can be set within configuration file.
// To add a new flag, add it to the enum before environmentFlagLastItem.
// And run `go generate` in the tests/framework/pkg/environmentflag directory.
const (
	KubernetesUpgradeAllClusters EnvironmentFlag = iota
	WorkloadUpgradeAllClusters
	UpdateClusterName
	GatekeeperAllowedNamespaces
	UpgradeAllClusters
	UseExistingRegistries
	InstallRancher
	Long
	Short
	environmentFlagLastItem // This is used to determine the number of items in the enum
)
