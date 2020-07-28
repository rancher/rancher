package mcmstart

import (
	"context"
	"sync"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/apimachinery/pkg/labels"
)

type handler struct {
	sync.Mutex
	once         sync.Once
	cancel       func()
	parentCTX    context.Context
	ctx          context.Context
	mcm          wrangler.MultiClusterManager
	featureCache managementv3.FeatureCache
}

func Register(ctx context.Context, features managementv3.FeatureController, mcm wrangler.MultiClusterManager) {
	h := &handler{
		parentCTX:    ctx,
		mcm:          mcm,
		featureCache: features.Cache(),
	}
	features.OnChange(ctx, "mcm-start", h.onChange)
}

func (h *handler) installed() bool {
	installed := true
	// only check if it's installed once, after that just assume it is
	h.once.Do(func() {
		// ignore error
		existingFeatures, _ := h.featureCache.List(labels.Everything())
		for _, f := range existingFeatures {
			if features.MCM.Name() == f.Name {
				return
			}
		}
		installed = false
	})
	return installed
}

func (h *handler) onChange(key string, feature *v3.Feature) (*v3.Feature, error) {
	h.Lock()
	defer h.Unlock()

	if !h.installed() {
		if features.MCM.Enabled() {
			return feature, h.start()
		}
		return feature, nil
	}

	if features.MCM.Name() != key {
		return feature, nil
	}

	if features.IsEnabled(feature) {
		return feature, h.start()
	}
	return feature, h.stop()
}

func (h *handler) start() error {
	if h.ctx != nil {
		return nil
	}

	mcmCTX, cancel := context.WithCancel(h.parentCTX)
	if err := h.mcm.Start(mcmCTX); err != nil {
		cancel()
		return err
	}

	h.ctx = mcmCTX
	h.cancel = cancel
	return nil
}

func (h *handler) stop() error {
	if h.ctx == nil {
		return nil
	}
	h.cancel()
	h.ctx = nil
	h.cancel = nil
	return nil
}
