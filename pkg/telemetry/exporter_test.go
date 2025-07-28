package telemetry

//import (
//	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
//	"github.com/rancher/wrangler/v3/pkg/generic/fake"
//	"github.com/stretchr/testify/assert"
//	"go.uber.org/mock/gomock"
//	"testing"
//)
//
//func TestTelemetryManager(t *testing.T) {
//
//	assert := assert.New(t)
//	ctrl := gomock.NewController(t)
//	clusterCache := fake.NewMockCacheInterface[*v3.Cluster](ctrl)
//	nodeCache := fake.NewMockCacheInterface[*v3.Node](ctrl)
//
//	telG := NewTelemetryGatherer(clusterCache, nodeCache)
//
//	manager := NewTelemetryExporterManager(telG)
//	assert.NotNil(manager)
//
//}
