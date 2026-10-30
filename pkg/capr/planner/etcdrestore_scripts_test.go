package planner

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

// fakeKubectlScript logs its arguments to $KUBECTL_LOG and answers the calls that the etcd restore scripts make.
const fakeKubectlScript = `#!/bin/sh
echo "$@" >> "$KUBECTL_LOG"
case "$*" in
	*custom-columns*) echo True ;;
	"get nodes"*) printf 'node-a\nnode-b\n' ;;
	*"get pods"*) ;;
	"delete node"*) ;;
	*) exit 1 ;;
esac
`

// writeFakeKubectl writes a fake kubectl into a directory name that contains a space and returns its path and the log path.
func writeFakeKubectl(t *testing.T) (string, string) {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("/bin/sh is not available: %v", err)
	}
	base := t.TempDir()
	binDir := filepath.Join(base, "my dir", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o700))
	kubectl := filepath.Join(binDir, "kubectl")
	require.NoError(t, os.WriteFile(kubectl, []byte(fakeKubectlScript), 0o700))
	return kubectl, filepath.Join(base, "kubectl.log")
}

func runScript(t *testing.T, script string, env []string, args ...string) string {
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
	return string(out)
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// TestEtcdRestoreScriptsKubectlPathWithSpace checks that the scripts run kubectl when the path contains a space.
func TestEtcdRestoreScriptsKubectlPathWithSpace(t *testing.T) {
	t.Parallel()

	t.Run("wait for pod list", func(t *testing.T) {
		t.Parallel()
		kubectl, logPath := writeFakeKubectl(t)
		runScript(t, etcdRestorePostRestoreWaitForPodListCleanupScript, []string{"KUBECTL_LOG=" + logPath},
			kubectl, "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "get", "pods", "--all-namespaces")
		assert.Contains(t, readLog(t, logPath), "get pods --all-namespaces")
	})

	t.Run("wait for ready", func(t *testing.T) {
		t.Parallel()
		// The script compares the output of wc -l as a string. BSD wc pads that output, so the script never ends there.
		if out, err := exec.Command("/bin/sh", "-c", "wc -l < /dev/null").Output(); err != nil || strings.TrimSpace(string(out)) != strings.TrimRight(string(out), "\n") {
			t.Skip("wc -l pads its output on this platform")
		}
		kubectl, logPath := writeFakeKubectl(t)
		runScript(t, etcdRestoreNodeWaitForReadyScript,
			[]string{"KUBECTL_LOG=" + logPath, "KUBECTL=" + kubectl, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml"},
			"machine-1")
		log := readLog(t, logPath)
		assert.Contains(t, log, "rke.cattle.io/machine=machine-1")
		assert.Contains(t, log, "get nodes")
	})

	t.Run("clean up nodes", func(t *testing.T) {
		t.Parallel()
		kubectl, logPath := writeFakeKubectl(t)
		dir := t.TempDir()
		machineIDs := filepath.Join(dir, "machine-ids")
		nodeNames := filepath.Join(dir, "node-names")
		require.NoError(t, os.WriteFile(machineIDs, nil, 0o600))
		require.NoError(t, os.WriteFile(nodeNames, []byte("node-a\n"), 0o600))

		runScript(t, etcdRestoreNodeCleanUpScript,
			[]string{"KUBECTL_LOG=" + logPath, "KUBECTL=" + kubectl, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml"},
			machineIDs, nodeNames)
		log := readLog(t, logPath)
		assert.Contains(t, log, "delete node node-b --wait=false")
		assert.False(t, strings.Contains(log, "delete node node-a"), "node-a must stay")
	})
}
