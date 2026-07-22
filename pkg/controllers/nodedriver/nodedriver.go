package nodedriver

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/wrangler"

	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
)

func Register(ctx context.Context, wrangler *wrangler.Context) {
	// This handler ensures all Rancher pods have an up-to-date version of the driver binaries inside the UIPath/assets directory.
	// This acts as a cache for rancher/machine (runs in a separate Job/Pod), which downloads the drivers from Rancher instead of the Internet.
	// Given those requests could be load-balanced to any of the Pods, all replicas must have the same data.
	wrangler.Mgmt.NodeDriver().OnChange(ctx, "custom-node-driver-handler", onChange)
}

func onChange(_ string, obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	// only cache drivers which are not built-in to rancher image
	if obj == nil || obj.Spec.Builtin || !obj.Spec.Active {
		return obj, nil
	}

	driver, err := drivers.NewDynamicDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	if err != nil {
		logrus.Errorf("failed initializing NodeDriver %q: %v", obj.Name, err)
		return obj, generic.ErrSkip
	}

	if driver.Valid() {
		// driver binary already downloaded and matches the expected hash
		return obj, nil
	}

	if err := driver.Stage(true); err != nil {
		return obj, err
	}

	if err := driver.Install(); err != nil {
		return obj, err
	}

	if err := driver.Executable(); err != nil {
		return obj, err
	}

	return obj, nil
}
