package etcdsnapshotrestore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// distroTokenHash mirrors k3s's util.ShortHash(password, 12): the first 12 characters of the
// hex-encoded sha256 of the token password. This is the value the distro stamps on every snapshot
// (util.GetTokenHash), and therefore the value TokenHashCommandFormat has to reproduce in shell.
// Reimplemented rather than imported — pulling in the k3s module (and the kubeadm bootstrap-token
// parser it depends on) is what TokenHashCommandFormat exists to avoid.
func distroTokenHash(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])[:12]
}

// writeTokenFile lays down a token file at <dir>/server/token with the given contents and returns
// dir, ready to be passed to TokenHashCommandFormat as the distro data directory.
func writeTokenFile(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	// The command interpolates the data directory into an unquoted shell assignment, so a path with
	// a space in it would not survive. Production data directories never have one, but t.TempDir
	// derives its path from the test name, so guard rather than debug a mangled command later.
	if strings.ContainsAny(dir, " \t\n'\"$") {
		t.Fatalf("temp dir %q contains characters the command cannot carry; rename the test case", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server", "token"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runTokenHashCommand runs TokenHashCommandFormat under /bin/sh exactly as buildPreflightPlan does,
// returning trimmed stdout, combined stderr and the error (non-nil on a non-zero exit).
func runTokenHashCommand(t *testing.T, dataDir string) (string, string, error) {
	t.Helper()

	var stdout, stderr strings.Builder
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(TokenHashCommandFormat, dataDir))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// requireShellTools skips the test when the utilities the command shells out to are unavailable, so
// the package still tests on a machine without a POSIX userland.
func requireShellTools(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"/bin/sh"} {
		if _, err := os.Stat(bin); err != nil {
			t.Skipf("%s is not available: %v", bin, err)
		}
	}
	for _, bin := range []string{"head", "sed", "sha256sum", "cut"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not on PATH: %v", bin, err)
		}
	}
}

// caHash stands in for the sha256-of-server-CA that clientaccess.FormatTokenBytes prepends. Only its
// shape matters to the command (64 hex characters containing no colon), not its value.
const caHash = "b2c3f6b8b8bd0d2a3e5f4c07e1c7f9a4d3b6e8c1a9f2d4b7c0e3a6f9b2d5c8e1"

// canonicalTokenFile renders the token file exactly as k3s writes it: FormatTokenBytes supplies the
// `K10<CA-hash>::` prefix, deps.readTokens supplies the `server:` username, and handlers.WriteToken
// appends the trailing newline.
func canonicalTokenFile(password string) string {
	return fmt.Sprintf("K10%s::server:%s\n", caHash, password)
}

// TestTokenHashCommandMatchesDistroHash is the unit-level counterpart to the imported e2e test
// Test_Imported_Operation_SetE_ImportedETCDSnapshotRestoreColonToken: it pins the Preflight check's
// shell derivation against the hash the distro stamps, for passwords that make the two diverge.
//
// The colon cases are the point. clientaccess.parseToken splits the username from the password with
// strings.SplitN(token, ":", 2), so a password keeps every colon after the first one — meaning any
// greedy strip in the shell (`sed 's/.*://'`) hashes only the trailing segment, and hashes the empty
// string for a password ending in a colon.
func TestTokenHashCommandMatchesDistroHash(t *testing.T) {
	t.Parallel()
	requireShellTools(t)

	for _, tt := range []struct {
		name     string
		password string
	}{
		{
			name:     "no colon",
			password: "3f8a1c9e5b2d7f04a6c8e1b3d5f7092a",
		},
		{
			name:     "single colon",
			password: "left:right",
		},
		{
			name:     "several colons",
			password: "a:b:c:d",
		},
		{
			name:     "leading colon",
			password: ":leading",
		},
		{
			// The greedy form hashed the empty string here, which is the worst case: sha256 of ""
			// is a perfectly well-formed hash, so the mismatch looked like a rotated token.
			name:     "trailing colon",
			password: "trailing:colon:",
		},
		{
			name:     "internal space",
			password: "pass with spaces",
		},
		{
			// A password that itself looks like a full K10 token must survive intact: the prefix
			// strip is anchored and applied once, and the username strip runs after it.
			name:     "password resembling a K10 token",
			password: "K10deadbeef::nested:secret",
		},
		{
			name:     "password resembling a bootstrap token",
			password: "abcdef.0123456789abcdef",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := writeTokenFile(t, canonicalTokenFile(tt.password))
			got, stderr, err := runTokenHashCommand(t, dir)
			if err != nil {
				t.Fatalf("command failed: %v\nstderr: %s", err, stderr)
			}
			if want := distroTokenHash(tt.password); got != want {
				t.Errorf("hash of password %q = %q, want %q (the value the distro stamps on the snapshot)", tt.password, got, want)
			}
		})
	}
}

// TestTokenHashCommandTolerantOfFileShape holds the derivation steady across the incidental
// variation in how the token file can land on disk. The password is fixed, so any difference in the
// output is the command reacting to the file's shape rather than to its credentials.
func TestTokenHashCommandTolerantOfFileShape(t *testing.T) {
	t.Parallel()
	requireShellTools(t)

	const password = "some:pass"

	for _, tt := range []struct {
		name     string
		contents string
	}{
		{
			name:     "canonical",
			contents: canonicalTokenFile(password),
		},
		{
			name:     "no trailing newline",
			contents: fmt.Sprintf("K10%s::server:%s", caHash, password),
		},
		{
			// bytes.TrimSpace on the distro side tolerates surrounding whitespace, so this must too —
			// and must trim only the edges, leaving any space inside the password alone.
			name:     "surrounding whitespace",
			contents: fmt.Sprintf("  K10%s::server:%s  \n", caHash, password),
		},
		{
			// parseToken accepts an empty CA hash (it only rejects a hash of the wrong non-zero
			// length), which is the form a bare token normalises to.
			name:     "empty CA hash",
			contents: fmt.Sprintf("K10::server:%s\n", password),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := writeTokenFile(t, tt.contents)
			got, stderr, err := runTokenHashCommand(t, dir)
			if err != nil {
				t.Fatalf("command failed: %v\nstderr: %s", err, stderr)
			}
			if want := distroTokenHash(password); got != want {
				t.Errorf("hash = %q, want %q", got, want)
			}
		})
	}
}

// TestTokenHashCommandFailsLoudlyWithoutAToken covers the reason the command guards its input at
// all. Every step of the derivation is a pipeline, whose exit status is the last element's, so an
// unreadable token file otherwise leaves the shell exiting 0 having printed sha256 of the empty
// string. That is a well-formed hash, so Preflight would compare it against the snapshot's and
// report a token mismatch — sending whoever reads the failure after a token rotation that never
// happened. A non-zero exit with no hash on stdout is what lets the step say what actually went
// wrong.
func TestTokenHashCommandFailsLoudlyWithoutAToken(t *testing.T) {
	t.Parallel()
	requireShellTools(t)

	emptyStringHash := distroTokenHash("")

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		// A data directory with no server/token in it at all.
		dir := t.TempDir()
		got, stderr, err := runTokenHashCommand(t, dir)
		if err == nil {
			t.Errorf("command succeeded with no token file, output %q; it must exit non-zero", got)
		}
		if got == emptyStringHash {
			t.Errorf("command printed sha256 of the empty string (%q), which preflight would read as a token mismatch", got)
		}
		if stderr == "" {
			t.Error("command wrote nothing to stderr; the failure needs to say the token file was unreadable")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()

		dir := writeTokenFile(t, "")
		got, _, err := runTokenHashCommand(t, dir)
		if err == nil {
			t.Errorf("command succeeded on an empty token file, output %q; it must exit non-zero", got)
		}
		if got == emptyStringHash {
			t.Errorf("command printed sha256 of the empty string (%q), which preflight would read as a token mismatch", got)
		}
	})

	t.Run("prefix but no password", func(t *testing.T) {
		t.Parallel()

		// parseToken rejects this too: SplitN(":", 2) yields an empty password, and it requires a
		// non-empty one.
		dir := writeTokenFile(t, fmt.Sprintf("K10%s::server:\n", caHash))
		got, _, err := runTokenHashCommand(t, dir)
		if err == nil {
			t.Errorf("command succeeded on a token file with no password, output %q; it must exit non-zero", got)
		}
		if got == emptyStringHash {
			t.Errorf("command printed sha256 of the empty string (%q), which preflight would read as a token mismatch", got)
		}
	})
}
