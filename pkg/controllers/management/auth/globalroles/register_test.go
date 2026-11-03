package globalroles

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
)

// TestRegisterWranglerIndexers verifies that the GlobalRoleBinding-by-GlobalRole indexer is
// registered outside of the leader-only Register, since it is consumed by the downstream
// enqueuers which run on the replica that owns the cluster.
func TestRegisterWranglerIndexers(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	grbCache := fake.NewMockNonNamespacedCacheInterface[*v3.GlobalRoleBinding](ctrl)
	grbCache.EXPECT().AddIndexer(pkgrbac.GRBGlobalRoleIndex, gomock.Any())

	RegisterWranglerIndexers(grbCache)
}
