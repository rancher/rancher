package operations

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	planapi "github.com/rancher/rancher/pkg/plan"
)

const testProvisioningDir = "/var/lib/rancher/capr"

func TestIdempotentActionScriptPath(t *testing.T) {
	t.Parallel()

	got := IdempotentActionScriptPath(testProvisioningDir)
	want := "/var/lib/rancher/capr/idempotence/idempotent.sh"
	if got != want {
		t.Fatalf("IdempotentActionScriptPath = %q, want %q", got, want)
	}
}

func TestIdempotentScriptFile(t *testing.T) {
	t.Parallel()

	file := IdempotentScriptFile(testProvisioningDir)

	if file.Path != IdempotentActionScriptPath(testProvisioningDir) {
		t.Errorf("Path = %q, want %q", file.Path, IdempotentActionScriptPath(testProvisioningDir))
	}
	if !file.Dynamic {
		t.Error("Dynamic should be true so the script can be updated in-place")
	}
	if !file.Minor {
		t.Error("Minor should be true so the script doesn't force a drain")
	}

	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		t.Fatalf("Content is not valid base64: %v", err)
	}
	if string(decoded) != IdempotentActionScript {
		t.Errorf("decoded content does not match IdempotentActionScript")
	}
}

func TestGenerateIdempotencyCleanupInstruction(t *testing.T) {
	t.Parallel()

	t.Run("empty key returns zero-valued instruction", func(t *testing.T) {
		got := GenerateIdempotencyCleanupInstruction(testProvisioningDir, "")
		if got.Command != "" || got.Name != "" || len(got.Args) != 0 {
			t.Fatalf("expected zero instruction, got %+v", got)
		}
	})

	t.Run("non-empty key generates rm command", func(t *testing.T) {
		got := GenerateIdempotencyCleanupInstruction(testProvisioningDir, "etcd-restore")
		if got.Command != "/bin/sh" {
			t.Errorf("Command = %q, want /bin/sh", got.Command)
		}
		if len(got.Args) != 2 || got.Args[0] != "-c" {
			t.Fatalf("Args = %v, want [-c, ...]", got.Args)
		}
		want := "rm -rf /var/lib/rancher/capr/idempotence/etcd-restore"
		if got.Args[1] != want {
			t.Errorf("Args[1] = %q, want %q", got.Args[1], want)
		}
	})
}

func TestIdempotentInstruction(t *testing.T) {
	t.Parallel()

	identifier := "etcd-restore/restore"
	value := "abc-uid"
	command := "rke2"
	extraArgs := []string{"server", "--cluster-reset"}
	env := []string{"FOO=bar"}

	got := IdempotentInstruction(testProvisioningDir, identifier, value, command, extraArgs, env)

	hashedCommand := planapi.PlanHash([]byte(command))
	hashedValue := planapi.PlanHash([]byte(value))

	wantName := fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand)
	if got.Name != wantName {
		t.Errorf("Name = %q, want %q", got.Name, wantName)
	}
	if got.Command != "/bin/sh" {
		t.Errorf("Command = %q, want /bin/sh", got.Command)
	}

	// The script invocation must contain (in order): -x, script path, lowercased identifier,
	// hashedValue, hashedCommand, the wrapped command, the provisioning dir, then the extra args.
	wantArgs := []string{
		"-x",
		IdempotentActionScriptPath(testProvisioningDir),
		strings.ToLower(identifier),
		hashedValue,
		hashedCommand,
		command,
		testProvisioningDir,
		"server",
		"--cluster-reset",
	}
	if len(got.Args) != len(wantArgs) {
		t.Fatalf("Args length = %d, want %d. Got: %v", len(got.Args), len(wantArgs), got.Args)
	}
	for i, want := range wantArgs {
		if got.Args[i] != want {
			t.Errorf("Args[%d] = %q, want %q", i, got.Args[i], want)
		}
	}

	if len(got.Env) != 1 || got.Env[0] != "FOO=bar" {
		t.Errorf("Env = %v, want [FOO=bar]", got.Env)
	}
}

func TestIdempotentInstructionLowercasesIdentifier(t *testing.T) {
	t.Parallel()

	got := IdempotentInstruction(testProvisioningDir, "Etcd-Restore/Restore", "v1", "/bin/true", nil, nil)
	// The lowercased identifier is the 3rd script arg (after -x and the script path).
	if got.Args[2] != "etcd-restore/restore" {
		t.Errorf("identifier was not lowercased: %q", got.Args[2])
	}
}

func TestConvertToIdempotentInstruction(t *testing.T) {
	t.Parallel()

	base := planapi.OneTimeInstruction{
		CommonInstruction: planapi.CommonInstruction{
			Name:    "remove-etcd-db-dir",
			Image:   "rancher/rancher:test",
			Command: "rm",
			Args:    []string{"-rf", "/var/lib/rancher/rke2/server/db/etcd"},
			Env:     []string{"FOO=bar"},
		},
		SaveOutput: true,
	}

	got := ConvertToIdempotentInstruction(testProvisioningDir, "etcd-restore/clean-etcd-dir", "v1", base)

	// Image and SaveOutput are preserved across the wrapping.
	if got.Image != base.Image {
		t.Errorf("Image = %q, want %q", got.Image, base.Image)
	}
	if !got.SaveOutput {
		t.Error("SaveOutput should be preserved through the wrapping")
	}

	// The wrapped command (`rm`) must appear as one of the script's arguments — that's how the
	// idempotent script knows what to exec. Same for the env and trailing args.
	joined := strings.Join(got.Args, " ")
	if !strings.Contains(joined, "rm") {
		t.Errorf("wrapped command 'rm' missing from Args: %v", got.Args)
	}
	if !strings.Contains(joined, "/var/lib/rancher/rke2/server/db/etcd") {
		t.Errorf("wrapped argument missing from Args: %v", got.Args)
	}
	if len(got.Env) != 1 || got.Env[0] != "FOO=bar" {
		t.Errorf("Env not preserved: %v", got.Env)
	}
}

func TestIdempotentInstructionStableAcrossInvocations(t *testing.T) {
	t.Parallel()

	// Same inputs must produce the same instruction (including name) — otherwise the plan
	// would change every reconcile and re-trigger the instruction.
	a := IdempotentInstruction(testProvisioningDir, "key", "v1", "/bin/true", []string{"arg"}, nil)
	b := IdempotentInstruction(testProvisioningDir, "key", "v1", "/bin/true", []string{"arg"}, nil)

	if a.Name != b.Name {
		t.Errorf("instruction Name not stable: %q vs %q", a.Name, b.Name)
	}
	if strings.Join(a.Args, "|") != strings.Join(b.Args, "|") {
		t.Errorf("instruction Args not stable: %v vs %v", a.Args, b.Args)
	}
}

func TestIdempotentInstructionChangesWithValue(t *testing.T) {
	t.Parallel()

	// Changing the value must change the instruction Name — that's the whole point of the
	// hashedValue in the script path: it forces the script to re-run.
	a := IdempotentInstruction(testProvisioningDir, "key", "v1", "/bin/true", nil, nil)
	b := IdempotentInstruction(testProvisioningDir, "key", "v2", "/bin/true", nil, nil)

	if a.Name == b.Name {
		t.Errorf("instruction Name should differ when value changes: %q", a.Name)
	}
}

func TestGenerateIdempotencyCleanupInstructionHostilePath(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("/bin/sh is not available: %v", err)
	}

	base := t.TempDir()
	canary := filepath.Join(base, "canary")
	tests := []struct {
		name    string
		dirName string
	}{
		{name: "normal", dirName: "capr"},
		{name: "space", dirName: "with space"},
		{name: "newline", dirName: "new\nline"},
		{name: "single quote", dirName: "it's"},
		{name: "backslash", dirName: `back\slash`},
		{name: "substitution", dirName: "$(touch " + canary + ")"},
		{name: "backtick", dirName: "`touch " + canary + "`"},
		{name: "semicolon", dirName: "x;touch " + canary},
		{name: "quote escape", dirName: "x'; touch " + canary + "; echo '"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provisioningDir := filepath.Join(base, strings.ReplaceAll(tt.name, " ", "-"), tt.dirName)
			target := filepath.Join(provisioningDir, "idempotence", "etcd-restore")
			if err := os.MkdirAll(target, 0o700); err != nil {
				t.Fatal(err)
			}
			keep := filepath.Join(provisioningDir, "idempotence", "other")
			if err := os.MkdirAll(keep, 0o700); err != nil {
				t.Fatal(err)
			}

			got := GenerateIdempotencyCleanupInstruction(provisioningDir, "etcd-restore")
			if out, err := exec.Command(got.Command, got.Args...).CombinedOutput(); err != nil {
				t.Fatalf("command failed: %v, output: %s", err, out)
			}

			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Errorf("target %q still exists, err = %v", target, err)
			}
			if _, err := os.Stat(keep); err != nil {
				t.Errorf("sibling %q was removed: %v", keep, err)
			}
			if _, err := os.Stat(canary); !os.IsNotExist(err) {
				t.Errorf("the command ran a second command, err = %v", err)
			}
		})
	}
}
