package etcdsnapshotrestore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeKubectlScript logs its arguments to $KUBECTL_LOG and answers the calls that the restore scripts make.
const fakeKubectlScript = `#!/bin/sh
echo "$@" >> "$KUBECTL_LOG"
case "$*" in
	"get nodes"*) printf 'node-a\nnode-b\n' ;;
	*"get pods"*) ;;
	"delete node"*) ;;
	*) exit 1 ;;
esac
`

// writeFakeKubectl writes a fake kubectl into a directory name that contains a space and returns its path and the log path.
func writeFakeKubectl(t *testing.T) (string, string) {
	t.Helper()
	requireShellTools(t)
	base := t.TempDir()
	binDir := filepath.Join(base, "my dir", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o700))
	kubectl := filepath.Join(binDir, "kubectl")
	require.NoError(t, os.WriteFile(kubectl, []byte(fakeKubectlScript), 0o700))
	return kubectl, filepath.Join(base, "kubectl.log")
}

func runScript(t *testing.T, script string, env []string, args ...string) {
	t.Helper()
	scriptPath := filepath.Join(t.TempDir(), "script.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))

	// A script that does not run kubectl retries with sleeps, so bound the run.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{scriptPath}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", out)
}

// TestScriptsKubectlPathWithSpace checks that the scripts run kubectl when the path contains a space.
func TestScriptsKubectlPathWithSpace(t *testing.T) {
	t.Parallel()

	t.Run("wait for pod list", func(t *testing.T) {
		t.Parallel()
		kubectl, logPath := writeFakeKubectl(t)
		runScript(t, waitForPodListScript, []string{"KUBECTL_LOG=" + logPath},
			kubectl, "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "get", "pods", "--all-namespaces")
		log, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(log), "get pods --all-namespaces")
	})

	t.Run("clean up nodes", func(t *testing.T) {
		t.Parallel()
		kubectl, logPath := writeFakeKubectl(t)
		nodeNames := filepath.Join(t.TempDir(), "node-names")
		require.NoError(t, os.WriteFile(nodeNames, []byte("node-a\n"), 0o600))

		runScript(t, nodeCleanupScript,
			[]string{"KUBECTL_LOG=" + logPath, "KUBECTL=" + kubectl, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml"},
			nodeNames)
		log, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(log), "delete node node-b --wait=false")
		assert.False(t, strings.Contains(string(log), "delete node node-a"), "node-a must stay")
	})
}
