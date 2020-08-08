package features

import (
	"fmt"
	"strconv"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	features = make(map[string]*Feature)

	// Features, ex.: ClusterRandomName = newFeature("cluster-randomizer", "Randomizes clusters.", false, false)

	UnsupportedStorageDrivers = newFeature(
		"unsupported-storage-drivers",
		"Allows the use of types for storage providers and provisioners that are not enabled by default.",
		false,
		true,
		true)
	IstioVirtualServiceUI = newFeature(
		"istio-virtual-service-ui",
		"Exposes a UI that enables users to create, read, update and delete virtual services and destination rules, which are traffic management features of Istio.",
		true,
		true,
		true)
	Steve = newFeature(
		"dashboard",
		"Deploy experimental new UI for managing resources inside of clusters.",
		true,
		false,
		true)
	SteveProxy = newFeature(
		"proxy",
		"Use new experimental proxy for Kubernetes API requests.",
		false,
		true,
		true)
	MCM = newFeature(
		"multi-cluster-management",
		"Multi-cluster provisioning and management of Kubernetes clusters.",
		true,
		true,
		false)
)

type Feature struct {
	name string
	// val is the effective value- it is equal to default until explicitly changed
	description string
	val         bool
	def         bool
	// if a feature is not dynamic, then rancher must be restarted when the value is changed
	dynamic bool
	// Whether we should install this feature or assume something else will install and manage the Feature CR
	install bool
}

// InitializeFeatures updates feature default if given valid --features flag and creates/updates necessary features in k8s
func InitializeFeatures(featuresClient managementv3.FeatureClient, featureArgs string) {
	// applies any default values assigned in --features flag to feature map
	if err := applyArgumentDefaults(featureArgs); err != nil {
		logrus.Errorf("failed to apply feature args: %v", err)
	}

	if featuresClient == nil {
		return
	}

	// creates any features in map that do not exist, updates features with new default value
	for key, f := range features {
		featureState, err := featuresClient.Get(key, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("unable to retrieve feature %s in initialize features: %v", f.name, err)
			}

			if f.install {
				// value starts off as nil, that way rancher can determine if value has been manually assigned
				newFeature := &v3.Feature{
					ObjectMeta: metav1.ObjectMeta{
						Name: f.name,
					},
					Spec: v3.FeatureSpec{
						Value: nil,
					},
					Status: v3.FeatureStatus{
						Default:     f.def,
						Dynamic:     f.dynamic,
						Description: f.description,
					},
				}

				if _, err := featuresClient.Create(newFeature); err != nil {
					logrus.Errorf("unable to create feature %s in initialize features: %v", f.name, err)
				}
			}
		} else {
			newFeatureState := featureState.DeepCopy()
			// checks if default value has changed
			if featureState.Status.Default != f.def {
				newFeatureState.Status.Default = f.def
			}

			// checks if developer has changed dynamic value from previous rancher version
			if featureState.Status.Dynamic != f.dynamic {
				newFeatureState.Status.Dynamic = f.dynamic
			}

			// checks if developer has changed description value from previous rancher version
			if featureState.Status.Description != f.description {
				newFeatureState.Status.Description = f.description
			}

			if newFeatureState, err = featuresClient.Update(newFeatureState); err != nil {
				logrus.Errorf("unable to update feature %s in initialize features: %v", f.name, err)
				continue
			}

			if featureState.Spec.Value == nil {
				continue
			}

			if *featureState.Spec.Value == f.val {
				continue
			}

			f.Set(*featureState.Spec.Value)
		}
	}
}

// applyArgumentDefaults reads the features arguments and uses their values to overwrite
// the corresponding feature default value
func applyArgumentDefaults(featureArgs string) error {
	if featureArgs == "" {
		return nil
	}

	formattingError := fmt.Errorf("feature argument [%s] should be of the form \"features=feature1=bool,feature2=bool\"", featureArgs)
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
		features[k].val = v
	}

	return nil
}

// Enabled returns whether the feature is enabled
func (f *Feature) Enabled() bool {
	return f.val
}

// Disable will disable a feature such that regardless of the user's choice it will always be false
func (f *Feature) Disable() {
	f.val = false
	f.def = false
	delete(features, f.name)
}

// Dynamic returns whether the feature is dynamic. Rancher must be restarted when
// a non-dynamic feature's effective value is changed.
func (f *Feature) Dynamic() bool {
	return f.dynamic
}

func (f *Feature) Set(val bool) {
	f.val = val
}

func (f *Feature) Name() string {
	return f.name
}

func GetFeatureByName(name string) *Feature {
	return features[name]
}

func IsEnabled(feature *v3.Feature) bool {
	if feature == nil {
		return false
	}
	if feature.Spec.Value == nil {
		return feature.Status.Default
	}
	return *feature.Spec.Value
}

// newFeature adds feature to the global feature map
func newFeature(name, description string, def, dynamic, install bool) *Feature {
	feature := &Feature{
		name:        name,
		description: description,
		def:         def,
		val:         def,
		dynamic:     dynamic,
		install:     install,
	}

	// feature will be stored in feature map, features contained in feature
	// map will be then be initialized
	features[name] = feature

	return feature
}
