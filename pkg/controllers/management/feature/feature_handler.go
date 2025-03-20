package feature

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	datamanagement "github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

type handler struct {
	featuresClient       managementv3.FeatureClient
	featureEnqueue       func(string, time.Duration)
	tokensLister         managementv3.TokenCache
	tokenEnqueue         func(string, time.Duration)
	nodeDriverController normanv3.NodeDriverInterface
	managementContext    *config.ManagementContext
}

func Register(ctx context.Context, management *config.ManagementContext, wContext *wrangler.Context) {
	h := handler{
		featuresClient:       wContext.Mgmt.Feature(),
		featureEnqueue:       wContext.Mgmt.Feature().EnqueueAfter,
		tokensLister:         wContext.Mgmt.Token().Cache(),
		tokenEnqueue:         wContext.Mgmt.Token().EnqueueAfter,
		nodeDriverController: management.Management.NodeDrivers(""),
		managementContext:    management,
	}
	wContext.Mgmt.Feature().OnChange(ctx, "feature-handler", h.sync)
}

func (h *handler) sync(_ string, obj *v3.Feature) (*v3.Feature, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	obj, err := h.setLockedValue(obj)
	if err != nil {
		return obj, err
	}

	if obj.Name == features.TokenHashing.Name() {
		return obj, h.refreshTokens()
	}

	if obj.Name == features.Harvester.Name() {
		if err = h.syncHarvesterBaremetalContainerWorkloadFeature(obj); err != nil {
			return obj, err
		}
		return obj, h.syncHarvesterNodeDriver(obj)
	}

	if obj.Name == features.HarvesterBaremetalContainerWorkload.Name() {
		return obj, h.syncHarvesterFeature(obj)
	}
	return obj, nil
}

// syncHarvesterFeature ensures that Harvester feature is enabled
// if baremetal management feature is enabled and annotates feature with experimental annotation
func (h *handler) syncHarvesterFeature(obj *v3.Feature) error {

	objCopy := obj.DeepCopy()

	if objCopy.Annotations == nil {
		objCopy.Annotations = make(map[string]string)
	}

	objCopy.Annotations[v3.ExperimentalFeatureKey] = v3.ExperimentalFeatureValue

	if !reflect.DeepEqual(obj, objCopy) {
		_, err := h.featuresClient.Update(objCopy)
		return err
	}

	// if feature is enabled, ensure harvester feature is also enabled
	if features.GetFeatureByName(obj.Name).Enabled() {
		harvesterFeature, err := h.featuresClient.Get(features.Harvester.Name(), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error fetching feature %s: %w", features.Harvester.Name(), err)
		}
		harvesterFeatureCopy := harvesterFeature.DeepCopy()
		harvesterFeatureCopy.SetValue(true)
		if !reflect.DeepEqual(harvesterFeature, harvesterFeatureCopy) {
			if _, err := h.featuresClient.Update(harvesterFeatureCopy); err != nil {
				return fmt.Errorf("error updating Harvester feature %s: %w", obj.Name, err)
			}
		}
	}

	return nil
}

// syncHarvesterNodeDriver ensures that the Harvester node driver is disabled
// when the Harvester feature is disabled and that the node driver is enabled,
// when the Harvester feature is enabled - provided that the node driver
// exists. If it doesn't exist, the node driver is created.
func (h *handler) syncHarvesterNodeDriver(feature *v3.Feature) error {
	if feature.Spec.Value == nil {
		logrus.Debugf("feature %s contains nil value", feature.Name)
		return nil
	}

	driver, err := h.nodeDriverController.Controller().Lister().Get("", datamanagement.HarvesterDriver)
	if err != nil {
		if errors.IsNotFound(err) {
			return datamanagement.AddHarvesterMachineDriver(h.managementContext)
		}
		h.featureEnqueue(feature.Name, 10*time.Second)
		return err
	}

	if driver.Spec.Active == *feature.Spec.Value {
		return nil
	}

	driver = driver.DeepCopy()
	driver.Spec.Active = *feature.Spec.Value

	logrus.Infof("updating node driver %s", driver.Name)
	_, err = h.nodeDriverController.Update(driver)
	if err != nil {
		h.featureEnqueue(feature.Name, 10*time.Second)
	}
	return nil
}

// syncHarvesterBaremetalContainerWorkloadFeature ensures that the Harvester
// baremetal container workload feature is disabled when the Harvester feature
// is disabled.
func (h *handler) syncHarvesterBaremetalContainerWorkloadFeature(feat *v3.Feature) error {
	if feat.Spec.Value != nil && !*feat.Spec.Value {
		baremetal, err := h.featuresClient.Get(features.HarvesterBaremetalContainerWorkload.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		update := baremetal.DeepCopy()
		update.SetValue(false)
		if !reflect.DeepEqual(baremetal, update) {
			if _, err = h.featuresClient.Update(update); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *handler) refreshTokens() error {
	tokenList, err := h.tokensLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, token := range tokenList {
		if token.Annotations[tokens.TokenHashed] == "true" {
			continue
		}
		h.tokenEnqueue(token.Name, 10*time.Second)
	}
	return nil
}

// setLockedValue evaluates whether a value should be written to the lockedValue
// field on status and records the value if so.
func (h *handler) setLockedValue(obj *v3.Feature) (*v3.Feature, error) {
	lockedValueFromSpec := EvaluateLockedValueFromSpec(obj)

	if lockedValueFromSpec == nil && obj.Status.LockedValue == nil {
		return obj, nil
	}
	// Should update if locked value from spec does not match locked value on status. This includes if one is nil and
	// the other is not.
	update := (lockedValueFromSpec == nil && obj.Status.LockedValue != nil) ||
		(lockedValueFromSpec != nil && obj.Status.LockedValue == nil) || *lockedValueFromSpec != *obj.Status.LockedValue

	if !update {
		return obj, nil
	}

	featureCopy := obj.DeepCopy()
	featureCopy.Status.LockedValue = lockedValueFromSpec
	return h.featuresClient.Update(featureCopy)
}

// EvaluateLockedValueFromSpec evaluates whether updates to a feature's effective value
// should be prevented. If so LockedValue returns the value that should
// be pinned to a feature. If nil is returned, the features value can be
// changed and those changes should toggle the associated behavior.
// Return value meanings:
// * nil - not currently locked
// * false - currently locked and false value
// * true - currently locked and true value
func EvaluateLockedValueFromSpec(obj *v3.Feature) *bool {
	if obj.Status.LockedValue != nil {
		return obj.Status.LockedValue
	}
	switch obj.Name {
	case features.TokenHashing.Name():
		if obj.Spec.Value == nil {
			return nil
		}
		if !(*obj.Spec.Value) {
			return nil
		}
		value := true
		return &value
	}
	return nil
}
