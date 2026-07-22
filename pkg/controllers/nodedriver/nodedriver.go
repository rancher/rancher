package nodedriver

import (
	"context"
	"sync/atomic"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/drivers"
	"github.com/rancher/rancher/pkg/wrangler"

	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
)

var leader = atomic.Bool{}

func Register(ctx context.Context, wrangler *wrangler.Context) {
	wrangler.Mgmt.NodeDriver().OnChange(ctx, "custom-node-driver-handler", onChange)

	// when this pod becomes the leader, do not run the onChange code
	wrangler.OnLeaderOrDie("nodedriver-register", func(ctx context.Context) error {
		leader.Store(true)
		return nil
	})
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
