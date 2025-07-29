package telemetry

import (
	"context"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/telemetry/initcond"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// func TestMain(m *testing.M) {
// 	goleak.VerifyTestMain(m)
// }

type testExporter struct {
	received chan struct{}
}

func newTestExporter() *testExporter {
	return &testExporter{
		received: make(chan struct{}, 1),
	}
}

func (t *testExporter) Register(_ TelemetryGatherer) {}

func (t *testExporter) CollectAndExport() error {
	select {
	case t.received <- struct{}{}:
	default:
	}
	return nil
}

var _ TelemetryExporter = (*testExporter)(nil)

// This test is designed to catch race conditions with go test -race.
// It was also initially testing using 	"go.uber.org/goleak" to verify there were no
// goroutine leaks
func TestTelemetryManager(t *testing.T) {

	assert := assert.New(t)
	ctrl := gomock.NewController(t)

	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)

	clusterCache.EXPECT().List(gomock.Any()).AnyTimes().DoAndReturn(func(_ any) ([]*v3.Cluster, error) {
		return []*v3.Cluster{}, nil
	})

	nodeCache := fake.NewMockCacheInterface[*v3.Node](ctrl)

	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_ string, _ any) ([]*v3.Node, error) {
		return []*v3.Node{}, nil
	})
	telG := NewTelemetryGatherer(clusterCache, nodeCache)

	manager := NewTelemetryExporterManager(telG, time.Millisecond)
	assert.NotNil(manager)

	ctx, ca := context.WithCancel(context.Background())
	defer ca()
	t1 := newTestExporter()
	t2 := newTestExporter()
	manager.Register("a1", t1, time.Millisecond*10)
	manager.Register("a2", t2, time.Millisecond*5)

	assert.Equal(manager.Status("a1"), ExporterStatusNotRunning)
	assert.Equal(manager.Status("a2"), ExporterStatusNotRunning)

	err := manager.Start(ctx, initcond.InitInfo{
		ClusterUUID:    "asdasdasdasd",
		InstallUUID:    "asdlkfuj89sdf",
		ServerURL:      "http://localhost:8443",
		RancherVersion: "v2.dev",
		GitHash:        "asdasdasdasdasd",
	})

	assert.Nil(err)

	assert.Eventually(
		func() bool {
			return manager.Status("a1") == ExporterStatusRunning
		},
		time.Second, time.Millisecond*5,
	)

	assert.Eventually(
		func() bool {
			return manager.Status("a2") == ExporterStatusRunning
		},
		time.Second, time.Millisecond*5,
	)

	assert.Eventually(func() bool {
		select {
		case <-t1.received:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond*10)

	assert.Eventually(func() bool {
		select {
		case <-t2.received:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond*10)

	dynamicIds := []string{"a3", "a4", "a5", "a6", "a7", "a8", "a9"}
	for _, id := range dynamicIds {
		id := id
		go func() {
			manager.Register(id, newTestExporter(), time.Hour)
		}()
	}

	for _, id := range dynamicIds {
		assert.Eventually(
			func() bool {
				return manager.Status(id) == ExporterStatusRunning && manager.Has(id)
			},
			time.Second, time.Millisecond*5,
		)
	}

	for _, id := range dynamicIds {
		go func() {
			manager.Delete(id)
		}()
	}

	for _, id := range dynamicIds {
		assert.Eventually(
			func() bool {
				return manager.Status(id) == ExporterStatusNotRunning && !manager.Has(id)
			},
			time.Second, time.Millisecond*5,
		)
	}

	for _, id := range dynamicIds {
		go func() {
			manager.Register(id, newTestExporter(), time.Hour)
		}()

		go func() {
			manager.Delete(id)
		}()
	}

	stopErr := manager.Stop()
	assert.Nil(stopErr)

}
