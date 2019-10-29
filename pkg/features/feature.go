package features

import (
	"fmt"
	"strconv"
	"strings"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	features = make(map[string]*feature)

	// Features, ex.: ClusterRandomName = newFeature("cluster-randomizer", false)

	UnsupportedStorageDrivers = newFeature("unsupported-storage-drivers", false)
	IstioVirtualServiceUI     = newFeature("istio-virtual-service-ui", true)
)

type feature struct {
	name           string
	def            bool
	featuresLister v3.FeatureLister
}

// InitializeFeatures updates feature default if given valid --features flag and creates/updates necessary features in k8s
func InitializeFeatures(ctx *config.ScaledContext, featureArgs string) {
	// applies any default values assigned in --features flag to feature map
	if err := applyArgumentDefaults(featureArgs); err != nil {
		logrus.Errorf("failed to apply feature args: %v", err)
	}

	// creates any features in map that do not exist, updates features with new default value
	for key, f := range features {
		f.featuresLister = ctx.Management.Features("").Controller().Lister()

		oldFeatureState, err := ctx.Management.Features("").Get(key, v1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("unable to retrieve feature %s in initialize features: %v", f.name, err)
			}

			// value starts off as nil, that way rancher can determine if value has been manually assigned
			newFeature := &v3.Feature{
				ObjectMeta: v1.ObjectMeta{
					Name: f.name,
				},
				Default: f.def,
				Value:   nil,
			}

			if _, err := ctx.Management.Features("").Create(newFeature); err != nil {
				logrus.Errorf("unable to create feature %s in initialize features: %v", f.name, err)
			}
		} else {
			// checks if developer has changed default value from previous rancher version
			if oldFeatureState.Default != f.def {
				newFeatureState := oldFeatureState.DeepCopy()
				newFeatureState.Default = f.def
				if _, err := ctx.Management.Features("").Update(newFeatureState); err != nil {
					logrus.Errorf("unable to update feature %s in initialize features: %v", f.name, err)
				}
			}
		}
	}
}

// applyArgumentDefaults reads the features arguments and uses their values to overwrite
// the corresponding feature default value
func applyArgumentDefaults(featureArgs string) error {
	if featureArgs == "" {
		return nil
	}

	formattingError := fmt.Errorf("feature argument should be of the form \"features=feature1=bool,feature2=bool\"")
	args := strings.Split(featureArgs, ",")

	applyFeatureDefaults := make(map[string]bool)

	for _, feature := range args {
		featureSet := strings.Split(feature, "=")
		if len(featureSet) != 2 {
			return formattingError
		}

		key := featureSet[0]
		if features[key] == nil {
			return fmt.Errorf("\"%s\" is not a valid feature", key)
		}

		value, err := strconv.ParseBool(featureSet[1])
		if err != nil {
			return formattingError
		}

		applyFeatureDefaults[key] = value
	}

	// only want to apply defaults once all args have been parsed and validated
	for k, v := range applyFeatureDefaults {
		features[k].def = v
	}

	return nil
}

// Enabled returns whether the Feature is enabled in the global feature gate.
// This should be primarily used for schema enable function and add feature handler functions
func (f *feature) Enabled() bool {
	featureState, err := f.featuresLister.Get("", f.name)
	if err != nil {
		return false
	}
	if featureState.Value != nil {
		return *featureState.Value
	}
	return featureState.Default
}

// newFeature adds feature to global feature gate
func newFeature(name string, def bool) *feature {
	feature := &feature{
		name,
		def,
		nil,
	}

	// feature will be stored in feature map, features contained in feature
	// map will be then be initialized
	features[name] = feature

	return feature
}
