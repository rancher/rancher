package fleetcharts

import (
	"context"
	"sync"
	"time"

	fleetv1alpha1api "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

var (
	fleetWaitLock sync.Mutex
)

func WaitForFleet(ctx context.Context, clients *wrangler.Context) {
	if !features.Fleet.Enabled() {
		return
	}

	fleetWaitLock.Lock()
	defer fleetWaitLock.Unlock()

	// ensure that fleet has been deployed before we start controllers
	for {
		_, err := clients.SharedControllerFactory.ForKind(fleetv1alpha1api.SchemeGroupVersion.WithKind("Bundle"))
		if err == nil {
			break
		}
		logrus.Infof("Waiting for fleet to be installed before proceeding")
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
