package features

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const primeEnv = "RANCHER_VERSION_TYPE"

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
	MCM = newFeature(
		"multi-cluster-management",
		"Multi-cluster provisioning and management of Kubernetes clusters.",
		true,
		false,
		true)
	MCMAgent = newFeature(
		"multi-cluster-management-agent",
		"Run downstream controllers for multi-cluster management.",
		false,
		false,
		false)
	Fleet = newFeature(
		"fleet",
		"Install Fleet when starting Rancher",
		true,
		false,
		true)
	Gitops = newFeature(
		"continuous-delivery",
		"Gitops components in fleet",
		true,
		false,
		true)
	Auth = newFeature(
		"auth",
		"Enable authentication",
		true,
		false,
		false)
	EmbeddedClusterAPI = newFeature(
		"embedded-cluster-api",
		"Enable a Rancher-managed instance of cluster-api core controller",
		false,
		false,
		false)
	ManagedSystemUpgradeController = newFeature(
		"managed-system-upgrade-controller",
		"Enable the installation of the system-upgrade-controller app as a managed system chart",
		true,
		false,
		false)
	RKE2 = newFeature(
		"rke2",
		"Enable provisioning of RKE2",
		true,
		false,
		true)
	Legacy = newFeature(
		"legacy",
		"Enable legacy features",
		false,
		true,
		true)
	ProvisioningV2 = newFeature(
		"provisioningv2",
		"Enable cluster-api based provisioning framework",
		true,
		false,
		false)
	TokenHashing = newFeature(
		"token-hashing",
		"Enable one way hashing of tokens. Once enabled token hashing can not be disabled",
		false,
		true,
		true)
	Harvester = newFeature(
		"harvester",
		"Enable Harvester integration, with ability to import and manage Harvester clusters",
		true,
		true,
		true)
	RKE1CustomNodeCleanup = newFeature(
		"rke1-custom-node-cleanup",
		"Enable cleanup RKE1 custom cluster nodes when they are deleted",
		true,
		true,
		true)
	HarvesterBaremetalContainerWorkload = newFeature(
		"harvester-baremetal-container-workload",
		"[Experimental]: Deploy container workloads to underlying harvester cluster",
		false,
		true,
		true)
	ProvisioningV2FleetWorkspaceBackPopulation = newFeature(
		"provisioningv2-fleet-workspace-back-population",
		"[Experimental]: Allow Fleet workspace name to be changed on clusters administrated by provisioning v2",
		false,
		false,
		true)
	UIExtension = newFeature(
		"uiextension",
		"Enable UI Extensions when starting Rancher",
		true,
		false,
		true)
	UISQLCache = newFeature(
		"ui-sql-cache",
		"Improve performance by enabling SQLite-backed caching. This also enables server-side pagination and other scaling based performance improvements.",
		true,
		false,
		true)
	RKE1UI = newFeature(
		"rke1-ui",
		"Enable RKE1 provisioning in the Rancher UI",
		true,
		true,
		true)
	ProvisioningPreBootstrap = newFeature(
		"provisioningprebootstrap",
		"Support running pre-bootstrap workloads on downstream clusters",
		false,
		false,
		true)
	CleanStaleSecrets = newFeature(
		"clean-stale-secrets",
		"Remove unused impersonation secrets from the cattle-impersonation namespace",
		true,
		false,
		true)
	AggregatedRoleTemplates = newFeature(
		"aggregated-roletemplates",
		"[Experimental] Make RoleTemplates use aggregation for generated RBAC roles",
		false,
		false,
		true).lockOnInstall()
	ClusterAgentSchedulingCustomization = newFeature(
		"cluster-agent-scheduling-customization",
		"Enables the automatic deployment of Pod Disruption Budgets and Priority Classes when deploying the cattle-cluster-agent. Disabling this feature will not impact existing clusters.",
		false,
		true,
		true)
	Provisioningv2ETCDSnapshotBackPopulation = newFeature(
		"v2prov-etcd-snapshot-backpopulate",
		"Allow Rancher to create ETCD Snapshot CRs for downstream clusters in the local cluster",
		true,
		false,
		true)
	OIDCProvider = newFeature(
		"oidc-provider",
		"Provide an OIDC provider embedded in Rancher. Required to enable SSO in Rancher Prime components.",
		isPrime(),
		false,
		true)
	RancherSCCRegistrationExtension = newFeature(
		"rancher-scc-registration-extension",
		"Enable Rancher's SCC registration extension to register the system(s) for customer support",
		isPrime(),
		false,
		true)
	Turtles = newFeature(
		"turtles",
		"Enable Rancher Turtles for managing CAPI lifecycle",
		true,
		false,
		false)
	ClusterAutoscaling = newFeature(
		"cluster-autoscaling",
		"Enable Rancher cluster-autoscaler support",
		isPrime(),
		false,
		true,
	)
	V3Public = newFeature(
		"v3-public",
		"Enable /v3-public API endpoints",
		true,
		false,
		true,
	)
)

func ListEnabled() []string {
	ret := []string{}
	for _, f := range features {
		if f.Enabled() {
			ret = append(ret, f.name)
		}
	}
	return ret
}

type Feature struct {
	name        string
	description string
	// val is the effective value- it is equal to default until explicitly changed.
	// The order of precedence is lockedValue > value > default
	val *bool
	// default value of feature
	def bool
	// if a feature is not dynamic, then rancher must be restarted when the value is changed
	dynamic bool
	// Whether we should install this feature or assume something else will install and manage the Feature CR
	install bool
	// If a feature is locked on install, it can't be modified after install. A new Rancher instance is required to change the value.
	lockedOnInstall bool
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

	// external-rules feature flag was removed in 2.9. We need to delete it for users upgrading from 2.8.
	err := featuresClient.Delete("external-rules", &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logrus.Errorf("unable to delete external-rules feature: %v", err)
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
				if f.lockedOnInstall {
					newFeature.Status.LockedValue = &f.def
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

			newFeatureState, err = featuresClient.Update(newFeatureState)
			if err != nil {
				logrus.Errorf("unable to update feature %s in initialize features: %v", f.name, err)
				continue
			}

			newVal := newFeatureState.Status.LockedValue
			if newVal == nil {
				newVal = featureState.Spec.Value
			}
			if newVal == nil {
				f.Unset()
			} else {
				f.Set(*newVal)
			}
		}
	}
}

func SetFeature(featuresClient managementv3.FeatureClient, featureName string, value bool) error {
	if featuresClient == nil {
		return nil
	}

	featureState, err := featuresClient.Get(featureName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	featureState.Spec.Value = &[]bool{value}[0]
	if _, err = featuresClient.Update(featureState); err != nil {
		return err
	}

	return nil
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
	}

	return nil
}

// Enabled returns whether the feature is enabled
func (f *Feature) Enabled() bool {
	if f.val != nil {
		return *f.val
	}
	return f.def
}

// RequireRestarts determines whether a Rancher restart is necessary. This is
// the case when (1) the feature is non-dynamic and (2) when the value (.spec.value or .status.lockedValue)
// was changed by a user.
//
// Rancher does not require restart when only the default value changes because
// this only happens during an upgrade.
func RequireRestarts(f *Feature, obj *v3.Feature) bool {
	if f.Dynamic() {
		return false
	}

	val := obj.Status.LockedValue
	if val == nil {
		val = obj.Spec.Value
	}

	if val == nil {
		// val was already nil, so no changes here
		if f.val == nil {
			return false
		}

		// we're unsetting val, let's check if the previous value was
		// the same as the default
		return *f.val != f.def
	}

	return *val != f.Enabled()
}

// Dynamic returns whether the feature is dynamic. Rancher must be restarted when
// a non-dynamic feature's value is changed (excluding a default value change)
func (f *Feature) Dynamic() bool {
	return f.dynamic
}

func (f *Feature) Set(val bool) {
	f.val = &val
}

func (f *Feature) Unset() {
	f.val = nil
}

func (f *Feature) Name() string {
	return f.name
}

func (f *Feature) lockOnInstall() *Feature {
	f.lockedOnInstall = true
	return f
}

func GetFeatureByName(name string) *Feature {
	return features[name]
}

func IsEnabled(feature *v3.Feature) bool {
	if feature == nil {
		return false
	}
	if feature.Status.LockedValue != nil {
		return *feature.Status.LockedValue
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
		dynamic:     dynamic,
		install:     install,
	}

	// feature will be stored in feature map, features contained in feature
	// map will then be initialized
	features[name] = feature

	return feature
}

// isPrime returns true if it is a Rancher Prime installation
func isPrime() bool {
	return os.Getenv(primeEnv) == "prime"
}
