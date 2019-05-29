package features

import (
	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/settings"
	"k8s.io/apiserver/pkg/util/feature"
)

var (
	globalGate = newFeatureGate()

	KontainerDriver   = NewFeature("kontainer-driver", false, "alpha")
	ClusterRandomName = NewFeature("cluster-randomizer", false, "alpha")
)

type featureGate struct {
	feature.FeatureGate

	// keep track of features as knownfeatures function is not sufficient
	FeatureSetting map[string]settings.Setting
}

func SetFeature(name string, val string) error {
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fmt.Errorf("unable to convert value to type bool for setting %s", name)
	}

	globalGate.SetFromMap(map[string]bool{name: b})

	return nil
}

func NewFeature(name string, def bool, stage string) settings.Setting {
	featureName := feature.Feature(name)
	prerelease := feature.GA

	switch stage {
	case "alpha":
		prerelease = feature.Alpha
	case "beta":
		prerelease = feature.Beta
	case "deprecated":
		prerelease = feature.Deprecated
	}

	fspec := feature.FeatureSpec{
		def,
		prerelease,
	}

	addFeature := map[feature.Feature]feature.FeatureSpec{
		featureName: fspec,
	}

	globalGate.Add(addFeature)
	addToFeaturesSetting(name)

	setting := settings.NewSetting(name, strconv.FormatBool(def))
	globalGate.FeatureSetting[name] = setting

	return settings.NewSetting(name, strconv.FormatBool(def))
}

func newFeatureGate() *featureGate {
	return &featureGate{
		feature.NewFeatureGate(),

		map[string]settings.Setting{},
	}
}

func addToFeaturesSetting(name string) {
	f := settings.Features.Get()
	if len(f) > 0 {
		settings.Features.Set(f + "," + name)
	} else {
		settings.Features.Set(name)
	}
}

func SettingExists(name string) bool {
	if globalGate.FeatureSetting[name] != (settings.Setting{}) {
		return true
	}
	return false
}

func IsFeatEnabled(name string) bool {
	feat := feature.Feature(name)
	return globalGate.Enabled(feat)
}

func GetFeatureSettings() map[string]settings.Setting {
	return globalGate.FeatureSetting
}
