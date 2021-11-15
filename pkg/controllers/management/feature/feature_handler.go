package feature

import (
	"context"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/labels"
)

type handler struct {
	featuresClient       managementv3.FeatureClient
	tokensLister         managementv3.TokenCache
	tokenEnqueue         func(string, time.Duration)
	nodeDriverController managementv3.NodeDriverController
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	h := handler{
		featuresClient:       wContext.Mgmt.Feature(),
		tokensLister:         wContext.Mgmt.Token().Cache(),
		tokenEnqueue:         wContext.Mgmt.Token().EnqueueAfter,
		nodeDriverController: wContext.Mgmt.NodeDriver(),
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
		return obj, h.toggleHarvesterNodeDriver(obj.Name)
	}
	return obj, nil
}

func (h *handler) toggleHarvesterNodeDriver(harvester string) error {
	if val := features.GetFeatureByName(harvester).Enabled(); val {
		m, err := h.nodeDriverController.Cache().Get(harvester)
		if err != nil {
			return err
		}
		m.Spec.Active = val
		_, err = h.nodeDriverController.Update(m)
		return err
	}
	return nil
}

func (h *handler) refreshTokens() error {
	tokenList, err := h.tokensLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, token := range tokenList {
		if token.Labels != nil && token.Labels[tokens.TokenHashed] == "true" {
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
