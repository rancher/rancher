package etcdsnapshotrestore

import (
	"encoding/base64"
	"path"
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// TestBuildRestartPlanDropInDataDirectory checks that the token-file value in the server drop-in parses back
// to the exact path, so a data directory cannot end the YAML string or add a key.
func TestBuildRestartPlanDropInDataDirectory(t *testing.T) {
	t.Parallel()

	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel:         "true",
		capr.ControlPlaneRoleLabel: "true",
		capr.InitNodeLabel:         "true",
	})
	otherEtcd := makePlanSecret("etcd-2", "node-etcd-2", map[string]string{capr.EtcdRoleLabel: "true"})

	tests := []struct {
		name    string
		dataDir string
	}{
		{name: "normal", dataDir: "/var/lib/rancher/rke2"},
		{name: "space", dataDir: "/data/my dir"},
		{name: "double quote", dataDir: `/data/a"b`},
		{name: "quote then key", dataDir: "/data/a\"\ninjected: \"yes"},
		{name: "backslash", dataDir: `/data/a\b`},
		{name: "newline", dataDir: "/data/a\nb"},
		{name: "single quote", dataDir: "/data/it's"},
		{name: "colon and hash", dataDir: "/data/a: b #c"},
		{name: "unicode", dataDir: "/data/é/日本"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter := defaultAdapter()
			adapter.dataDir = tt.dataDir
			s := newTestScope(adapter, types.UID("uid-1"))

			nodePlan, err := buildRestartPlan(s, otherEtcd, initSecret, "10.0.0.1", "uid-1/initial", true)
			require.NoError(t, err)
			require.Len(t, nodePlan.Files, 2)

			content, err := base64.StdEncoding.DecodeString(nodePlan.Files[1].Content)
			require.NoError(t, err)

			parsed := map[string]string{}
			require.NoError(t, yaml.Unmarshal(content, &parsed), "content: %q", content)
			assert.Equal(t, map[string]string{
				"server":     "https://10.0.0.1:9345",
				"token-file": path.Join(tt.dataDir, "server/token"),
			}, parsed)
		})
	}
}
