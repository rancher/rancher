package etcdsnapshotrestore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
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

// hostileDataDirs are data directory names that a shell must treat as one literal word. CANARY stands
// for a file path. A payload that writes to it shows that the shell ran a second command. The sibling is
// a directory that an unquoted path would also remove.
var hostileDataDirs = []struct{ name, dirName, sibling string }{
	{name: "normal", dirName: "rke2"},
	{name: "space", dirName: "with space", sibling: "with"},
	{name: "newline", dirName: "new\nline"},
	{name: "single quote", dirName: "it's"},
	{name: "backslash", dirName: `back\slash`},
	{name: "substitution", dirName: "$(touch CANARY)"},
	{name: "backtick", dirName: "`touch CANARY`"},
	{name: "semicolon", dirName: "x;touch CANARY"},
	{name: "quote escape", dirName: "x'; touch CANARY; echo '"},
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o700))
}

// fakeCurlScript records the values of --cacert and -u, and prints a member list without learners.
const fakeCurlScript = `#!/bin/sh
prev=""
for a in "$@"; do
  case "$prev" in
    --cacert) printf '%s' "$a" > "$CURL_CA" ;;
    -u) printf '%s' "$a" > "$CURL_USER" ;;
  esac
  prev=$a
done
printf '%s\n' '{"members":[{"ID":1,"name":"node-a-abc12345"}]}'
`

const membersJSON = `{"members":[{"ID":1,"name":"node-a-abc12345"}]}`

// TestPostflightPlanDataDirectory runs the postflight command with hostile data directories. The command
// must read the token from the exact path, pass the exact CA path to curl, and run no second command.
func TestPostflightPlanDataDirectory(t *testing.T) {
	t.Parallel()
	requireShellTools(t)

	password := "3f8a1c9e5b2d7f04a6c8e1b3d5f7092a"
	secret := makePlanSecret("etcd-2", "node-etcd-2", map[string]string{capr.EtcdRoleLabel: "true"})

	for _, tt := range hostileDataDirs {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			canary := filepath.Join(base, "canary")
			dataDir := filepath.Join(base, "data", strings.ReplaceAll(tt.dirName, "CANARY", canary))
			require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "server"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dataDir, "server", "token"), []byte(canonicalTokenFile(password)), 0o600))

			bin := filepath.Join(base, "bin")
			writeExecutable(t, filepath.Join(bin, "curl"), fakeCurlScript)

			adapter := defaultAdapter()
			adapter.dataDir = dataDir
			nodePlan, err := buildPostflightPlan(newTestScope(adapter, types.UID("uid-1")), secret)
			require.NoError(t, err)
			inst := nodePlan.OneTimeInstructions[0]

			cmd := exec.Command(inst.Command, inst.Args...)
			cmd.Dir = base
			cmd.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"CURL_CA="+filepath.Join(base, "curl-ca"),
				"CURL_USER="+filepath.Join(base, "curl-user"),
			)
			out, err := cmd.Output()
			require.NoError(t, err)
			assert.Equal(t, membersJSON, strings.TrimSpace(string(out)))

			ca, err := os.ReadFile(filepath.Join(base, "curl-ca"))
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(dataDir, "server/tls/server-ca.crt"), string(ca))
			user, err := os.ReadFile(filepath.Join(base, "curl-user"))
			require.NoError(t, err)
			assert.Equal(t, "server:"+password, string(user))
			_, err = os.Stat(canary)
			require.True(t, os.IsNotExist(err), "the command ran a second command")
		})
	}
}

// TestRejoinCleanupDataDirectory runs the etcd data removal of a rejoining node with hostile data
// directories. The command must remove only server/db/etcd and run no second command.
func TestRejoinCleanupDataDirectory(t *testing.T) {
	t.Parallel()
	requireShellTools(t)

	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel:         "true",
		capr.ControlPlaneRoleLabel: "true",
		capr.InitNodeLabel:         "true",
	})
	otherEtcd := makePlanSecret("etcd-2", "node-etcd-2", map[string]string{capr.EtcdRoleLabel: "true"})

	for _, tt := range hostileDataDirs {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			canary := filepath.Join(base, "canary")
			parent := filepath.Join(base, "data")
			dataDir := filepath.Join(parent, strings.ReplaceAll(tt.dirName, "CANARY", canary))
			etcdDir := filepath.Join(dataDir, "server/db/etcd")
			require.NoError(t, os.MkdirAll(etcdDir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(etcdDir, "member"), nil, 0o600))
			keep := filepath.Join(dataDir, "server/token")
			require.NoError(t, os.WriteFile(keep, nil, 0o600))
			var sibling string
			if tt.sibling != "" {
				sibling = filepath.Join(parent, tt.sibling)
				require.NoError(t, os.MkdirAll(sibling, 0o700))
			}

			bin := filepath.Join(base, "bin")
			writeExecutable(t, filepath.Join(bin, "systemctl"), "#!/bin/sh\nexit 0\n")

			adapter := defaultAdapter()
			adapter.dataDir = dataDir
			nodePlan, err := buildRestartPlan(newTestScope(adapter, types.UID("uid-1")), otherEtcd, initSecret, "10.0.0.1", "uid-1/initial", true)
			require.NoError(t, err)

			var script string
			for _, inst := range nodePlan.OneTimeInstructions {
				if strings.Contains(inst.Name, "rejoin-clean-etcd-dir") {
					script = inst.Args[len(inst.Args)-1]
				}
			}
			require.NotEmpty(t, script, "plan has no rejoin cleanup instruction")

			cmd := exec.Command("/bin/sh", "-c", script)
			// A relative path that the shell splits off would resolve here, not in the repository.
			cmd.Dir = base
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "output: %s", out)

			_, err = os.Stat(etcdDir)
			assert.True(t, os.IsNotExist(err), "etcd directory still exists")
			_, err = os.Stat(keep)
			assert.NoError(t, err, "the command removed more than the etcd directory")
			if sibling != "" {
				_, err = os.Stat(sibling)
				assert.NoError(t, err, "the command removed a sibling directory")
			}
			_, err = os.Stat(canary)
			require.True(t, os.IsNotExist(err), "the command ran a second command")
		})
	}
}
