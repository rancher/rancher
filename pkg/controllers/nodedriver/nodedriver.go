package nodedriver

import (
	"context"
	"sync/atomic"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/wrangler"
)

var leader = atomic.Bool{}

func Register(ctx context.Context, wrangler *wrangler.Context) {
	wrangler.Mgmt.NodeDriver().OnChange(ctx, "custom-node-driver-handler", onChange)

	// when this pod becomes the leader, do not run the onChange code
	wrangler.OnLeader(func(ctx context.Context) error {
		leader.Store(true)
		return nil
	})
}

func onChange(_ string, obj *v3.NodeDriver) (*v3.NodeDriver, error) {
	if obj == nil {
		return obj, nil
	}

	// if leader, no need to sync
	if leader.Load() {
		return obj, nil
	}

	if !obj.Spec.Active {
		return obj, nil
	}

	// only cache drivers which are not built-in to rancher image
	if obj.Spec.Builtin {
		return obj, nil
	}

	driver := drivers.NewDynamicDriver(obj.Spec.Builtin, obj.Spec.DisplayName, obj.Spec.URL, obj.Spec.Checksum)
	if driver.Exists() {
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
