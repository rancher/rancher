package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// PinnedChart is a chart that Rancher installs at an exact version, whatever versions the charts repository has.
type PinnedChart struct {
	Name    string
	Version string
}

// PinnedCharts returns the charts that Rancher installs at an exact version, with the version that it installs.
// The version of system-upgrade-controller is only pinned when its setting has a value: with none, Rancher installs the
// latest one.
func PinnedCharts() []PinnedChart {
	pins := []PinnedChart{
		{Name: "fleet", Version: settings.FleetVersion.Get()},
		{Name: "rancher-webhook", Version: settings.RancherWebhookVersion.Get()},
		{Name: "remotedialer-proxy", Version: settings.RemoteDialerProxyVersion.Get()},
		{Name: "rancher-turtles", Version: settings.RancherTurtlesVersion.Get()},
	}
	// The setting only reads its variable when Rancher runs with the settings provider, which the export does not.
	version := os.Getenv(settings.GetEnvKey(settings.SystemUpgradeControllerChartVersion.Name))
	if version == "" {
		version = settings.SystemUpgradeControllerChartVersion.Get()
	}
	if version != "" {
		pins = append(pins, PinnedChart{Name: "system-upgrade-controller", Version: version})
	}
	return pins
}

// CheckPinnedCharts checks that the version of each pinned chart is the one FetchImagesV2 reads for the Rancher
// version, which is the newest one that supports it. Rancher installs the pinned version, so when the two differ the
// images of the chart that is installed are not in the list and an air-gapped installation cannot pull them.
//
// A pinned chart that the repositories of chartsPath (comma separated) do not have is not checked: it is in another
// repository. A mismatch is an error when strict is true, which is for the build of a release, and a warning
// otherwise.
func CheckPinnedCharts(chartsPath, rancherVersion string, pins []PinnedChart, strict bool) error {
	type repository struct {
		charts Charts
		index  *repo.IndexFile
	}
	var repositories []repository
	for _, path := range strings.Split(chartsPath, ",") {
		index, err := repo.LoadIndexFile(filepath.Join(path, "index.yaml"))
		if err != nil {
			return err
		}
		charts := Charts{ExportConfig{ChartsPath: path, RancherVersion: rancherVersion}}
		repositories = append(repositories, repository{charts, index})
	}

	var mismatches []string
	for _, pin := range pins {
		found, satisfied := false, false
		listed := "no version that supports Rancher " + rancherVersion
		for _, r := range repositories {
			versions, ok := r.index.Entries[pin.Name]
			if !ok {
				continue
			}
			found = true
			newest, err := r.charts.newestVersionOfTheMinor(versions)
			if err != nil {
				return err
			}
			if newest == nil {
				continue
			}
			if newest.Version == pin.Version {
				satisfied = true
				break
			}
			listed = newest.Version
		}
		if found && !satisfied {
			mismatches = append(mismatches, fmt.Sprintf("%s is pinned at %s and the list has the images of %s", pin.Name, pin.Version, listed))
		}
	}
	if len(mismatches) == 0 {
		return nil
	}

	message := "Rancher installs these charts at the pinned version, so the images of the list have to be of that version, " +
		"or an air-gapped installation cannot pull them:\n  " + strings.Join(mismatches, "\n  ")
	if strict {
		return fmt.Errorf("%s", message)
	}
	logrus.Warn(message)
	return nil
}
