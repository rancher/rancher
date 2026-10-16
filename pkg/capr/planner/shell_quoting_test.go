package planner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hostileDirNames returns directory names that a shell must treat as one literal word. Names that
// contain a payload write to canary if the shell runs a second command.
func hostileDirNames(canary string) map[string]string {
	return map[string]string{
		"space":        "with space",
		"newline":      "new\nline",
		"single quote": "it's",
		"backslash":    `back\slash`,
		"substitution": "$(touch " + canary + ")",
		"backtick":     "`touch " + canary + "`",
		"semicolon":    "x;touch " + canary,
		"quote escape": "x'; touch " + canary + "; echo '",
	}
}

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("/bin/sh is not available: %v", err)
	}
}

func runInstruction(t *testing.T, instruction plan.OneTimeInstruction) string {
	t.Helper()
	cmd := exec.Command(instruction.Command, instruction.Args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", out)
	return string(out)
}

func requireNoCanary(t *testing.T, canary string) {
	t.Helper()
	_, err := os.Stat(canary)
	require.True(t, os.IsNotExist(err), "the command ran a second command")
}

func newControlPlane(distroDir, provisioningDir string) *rkev1.RKEControlPlane {
	cp := &rkev1.RKEControlPlane{}
	cp.Spec.KubernetesVersion = "v1.28.0+rke2r1"
	cp.Spec.DataDirectories.K8sDistro = distroDir
	cp.Spec.DataDirectories.Provisioning = provisioningDir
	return cp
}

// TestShellInstructionsKeepBytesForValidPaths pins the exact command strings for valid paths. A change
// to these strings changes the plan of every existing cluster and restarts plan application.
func TestShellInstructionsKeepBytesForValidPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		distroDir       string
		provisioningDir string
		wantCleanup     string
		wantCapture     string
		wantEtcdName    string
		wantManifests   string
	}{
		{
			name:          "default directories",
			wantCleanup:   "rm -rf /var/lib/rancher/capr/idempotence/etcd-restore",
			wantCapture:   "curl -f --retry 100 --retry-delay 5 --cacert /var/lib/rancher/rke2/server/tls/server-ca.crt https://localhost:9345/db/info | grep 'clientURLs'",
			wantEtcdName:  "cat /var/lib/rancher/rke2/server/db/etcd/name",
			wantManifests: "rm -rf /var/lib/rancher/rke2/server/manifests/rke2-*.yaml",
		},
		{
			name:            "custom directories",
			distroDir:       "/data/rke2",
			provisioningDir: "/data/capr",
			wantCleanup:     "rm -rf /data/capr/idempotence/etcd-restore",
			wantCapture:     "curl -f --retry 100 --retry-delay 5 --cacert /data/rke2/server/tls/server-ca.crt https://localhost:9345/db/info | grep 'clientURLs'",
			wantEtcdName:    "cat /data/rke2/server/db/etcd/name",
			wantManifests:   "rm -rf /data/rke2/server/manifests/rke2-*.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cp := newControlPlane(tt.distroDir, tt.provisioningDir)

			cleanup := generateIdempotencyCleanupInstruction(cp, "etcd-restore")
			assert.Equal(t, "remove idempotency tracking", cleanup.Name)
			assert.Equal(t, []string{"-c", tt.wantCleanup}, cleanup.Args)

			nodePlan, err := (&Planner{}).addInitNodePeriodicInstruction(plan.NodePlan{}, cp)
			require.NoError(t, err)
			require.Len(t, nodePlan.PeriodicInstructions, 2)
			assert.Equal(t, captureAddressInstructionName, nodePlan.PeriodicInstructions[0].Name)
			assert.Equal(t, []string{"-c", tt.wantCapture}, nodePlan.PeriodicInstructions[0].Args)
			assert.Equal(t, etcdNameInstructionName, nodePlan.PeriodicInstructions[1].Name)
			assert.Equal(t, []string{"-c", tt.wantEtcdName}, nodePlan.PeriodicInstructions[1].Args)

			generated, removal := generateManifestRemovalInstruction(cp, &planEntry{Metadata: &plan.Metadata{
				Labels: map[string]string{capr.EtcdRoleLabel: "true"},
			}})
			require.True(t, generated)
			assert.Equal(t, "remove server manifests", removal.Name)
			assert.Equal(t, []string{"-c", tt.wantManifests}, removal.Args)
		})
	}
}

func TestGenerateIdempotencyCleanupInstructionHostilePath(t *testing.T) {
	t.Parallel()
	requireSh(t)

	base := t.TempDir()
	canary := filepath.Join(base, "canary")
	for name, dirName := range hostileDirNames(canary) {
		t.Run(name, func(t *testing.T) {
			provisioningDir := filepath.Join(base, "cleanup-"+strings.ReplaceAll(name, " ", "-"), dirName)
			target := filepath.Join(provisioningDir, "idempotence", "etcd-restore")
			require.NoError(t, os.MkdirAll(target, 0o700))
			keep := filepath.Join(provisioningDir, "idempotence", "other")
			require.NoError(t, os.MkdirAll(keep, 0o700))

			runInstruction(t, generateIdempotencyCleanupInstruction(newControlPlane("", provisioningDir), "etcd-restore"))

			assert.NoDirExists(t, target)
			assert.DirExists(t, keep)
			requireNoCanary(t, canary)
		})
	}
}

func TestGenerateManifestRemovalInstructionHostilePath(t *testing.T) {
	t.Parallel()
	requireSh(t)

	base := t.TempDir()
	canary := filepath.Join(base, "canary")
	entry := &planEntry{Metadata: &plan.Metadata{Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}}}
	for name, dirName := range hostileDirNames(canary) {
		t.Run(name, func(t *testing.T) {
			distroDir := filepath.Join(base, "manifests-"+strings.ReplaceAll(name, " ", "-"), dirName)
			manifestDir := filepath.Join(distroDir, "server/manifests")
			require.NoError(t, os.MkdirAll(manifestDir, 0o700))
			for _, f := range []string{"rke2-coredns.yaml", "rke2-ingress.yaml", "custom.yaml"} {
				require.NoError(t, os.WriteFile(filepath.Join(manifestDir, f), nil, 0o600))
			}

			generated, instruction := generateManifestRemovalInstruction(newControlPlane(distroDir, ""), entry)
			require.True(t, generated)
			runInstruction(t, instruction)

			// The glob must still match after quoting, and it must match only the runtime files.
			assert.NoFileExists(t, filepath.Join(manifestDir, "rke2-coredns.yaml"))
			assert.NoFileExists(t, filepath.Join(manifestDir, "rke2-ingress.yaml"))
			assert.FileExists(t, filepath.Join(manifestDir, "custom.yaml"))
			requireNoCanary(t, canary)
		})
	}
}

func TestAddInitNodePeriodicInstructionHostilePath(t *testing.T) {
	t.Parallel()
	requireSh(t)

	base := t.TempDir()
	canary := filepath.Join(base, "canary")
	for name, dirName := range hostileDirNames(canary) {
		t.Run(name, func(t *testing.T) {
			distroDir := filepath.Join(base, "periodic-"+strings.ReplaceAll(name, " ", "-"), dirName)
			require.NoError(t, os.MkdirAll(filepath.Join(distroDir, "server/db/etcd"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(distroDir, "server/db/etcd/name"), []byte("etcd-node-1"), 0o600))

			nodePlan, err := (&Planner{}).addInitNodePeriodicInstruction(plan.NodePlan{}, newControlPlane(distroDir, ""))
			require.NoError(t, err)
			require.Len(t, nodePlan.PeriodicInstructions, 2)

			// etcd-name reads the file, so it proves that the path reaches cat as one argument.
			etcdName := nodePlan.PeriodicInstructions[1]
			out := runInstruction(t, plan.OneTimeInstruction{CommonInstruction: etcdName.CommonInstruction})
			assert.Equal(t, "etcd-node-1", out)

			// capture-address calls curl with a retry loop. Replace curl and grep with shell functions
			// that print their arguments, so the test sees how sh split the command.
			capture := nodePlan.PeriodicInstructions[0].Args[1]
			script := "curl() { for a in \"$@\"; do printf '%s\\n' \"$a\"; done; }; grep() { cat; }; " + capture
			out = runInstruction(t, capturedCommand(script))
			args := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			cacertIndex := -1
			for i, a := range args {
				if a == "--cacert" {
					cacertIndex = i
				}
			}
			require.NotEqual(t, -1, cacertIndex, "output: %q", out)
			assert.Equal(t, filepath.Join(distroDir, "server/tls/server-ca.crt"), strings.Join(args[cacertIndex+1:len(args)-1], "\n"))
			assert.Equal(t, "https://localhost:9345/db/info", args[len(args)-1])
			requireNoCanary(t, canary)
		})
	}
}

func capturedCommand(script string) plan.OneTimeInstruction {
	instruction := plan.OneTimeInstruction{}
	instruction.Command = "/bin/sh"
	instruction.Args = []string{"-c", script}
	return instruction
}
