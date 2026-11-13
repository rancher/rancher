package capr

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "normal path stays unchanged", in: "/var/lib/rancher/rke2", want: "/var/lib/rancher/rke2"},
		{name: "allowed punctuation stays unchanged", in: "/opt/a_b-c.d@e+f:g,h", want: "/opt/a_b-c.d@e+f:g,h"},
		{name: "unicode letters stay unchanged", in: "/data/é/日本", want: "/data/é/日本"},
		{name: "empty string", in: "", want: "''"},
		{name: "space", in: "/var/lib/my dir", want: "'/var/lib/my dir'"},
		{name: "newline", in: "/var/lib/a\nb", want: "'/var/lib/a\nb'"},
		{name: "tab", in: "/var/lib/a\tb", want: "'/var/lib/a\tb'"},
		{name: "single quote", in: "/var/lib/it's", want: `'/var/lib/it'\''s'`},
		{name: "double quote", in: `/var/lib/a"b`, want: `'/var/lib/a"b'`},
		{name: "backslash", in: `/var/lib/a\b`, want: `'/var/lib/a\b'`},
		{name: "command substitution", in: "/a/$(id)", want: "'/a/$(id)'"},
		{name: "backtick", in: "/a/`id`", want: "'/a/`id`'"},
		{name: "semicolon", in: "/a;id", want: "'/a;id'"},
		{name: "glob", in: "/a/*?", want: "'/a/*?'"},
		{name: "exclamation mark", in: "/a/b!", want: "'/a/b!'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ShellQuote(tt.in))
		})
	}
}

// TestShellQuoteRunsAsOneWord runs the quoted value under sh and checks that sh passes the exact
// original string as one argument and runs no second command.
func TestShellQuoteRunsAsOneWord(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skipf("/bin/sh is not available: %v", err)
	}

	canary := filepath.Join(t.TempDir(), "canary")
	inputs := []string{
		"/var/lib/rancher/rke2",
		"/var/lib/my dir",
		"/var/lib/a\nb",
		"/var/lib/it's",
		`/var/lib/a"b`,
		`/var/lib/a"; touch ` + canary + `; "`,
		`/var/lib/a\b`,
		"/a/$(touch " + canary + ")",
		"/a/`touch " + canary + "`",
		"/a;touch " + canary,
		"/a'; touch " + canary + "; echo '",
		"/a/*?[x]",
		"-leading-dash",
		"",
	}
	for _, in := range inputs {
		// printf %s writes the single argument without a trailing newline.
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+ShellQuote(in)).Output()
		require.NoError(t, err, "input %q", in)
		assert.Equal(t, in, string(out), "input %q", in)
		_, err = os.Stat(canary)
		require.True(t, os.IsNotExist(err), "input %q ran a second command", in)
	}
}

func TestPowerShellQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "normal path", in: "C:/var/lib/rancher/rke2/bin/kubelet.exe", want: "'C:/var/lib/rancher/rke2/bin/kubelet.exe'"},
		{name: "space", in: "C:/my dir", want: "'C:/my dir'"},
		{name: "newline", in: "C:/a\nb", want: "'C:/a\nb'"},
		{name: "single quote", in: "C:/it's", want: "'C:/it''s'"},
		{name: "double quote", in: `C:/a"b`, want: `'C:/a"b'`},
		{name: "curly quotes", in: "C:/a\u2018b\u2019c\u201Ad\u201Be", want: "'C:/a\u2018\u2018b\u2019\u2019c\u201A\u201Ad\u201B\u201Be'"},
		{name: "backslash", in: `C:/a\b`, want: `'C:/a\b'`},
		{name: "subexpression", in: "C:/$(id)", want: "'C:/$(id)'"},
		{name: "empty", in: "", want: "''"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PowerShellQuote(tt.in))
		})
	}
}

// TestPowerShellQuoteRunsAsOneString runs the quoted value in PowerShell when pwsh is installed.
func TestPowerShellQuoteRunsAsOneString(t *testing.T) {
	t.Parallel()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh is not on PATH")
	}

	canary := filepath.Join(t.TempDir(), "canary")
	inputs := []string{
		"C:/var/lib/rancher/rke2",
		"C:/my dir",
		"C:/a\nb",
		"C:/it's",
		`C:/a"b`,
		`C:/a"; New-Item ` + canary + `; "`,
		"C:/a\u2018b\u2019c\u201Ad\u201Be",
		`C:/a\b`,
		"C:/$(New-Item " + canary + ")",
		"C:/a'; New-Item " + canary + "; '",
		"C:/a`nb",
	}
	for _, in := range inputs {
		out, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command",
			"[Console]::Out.Write("+PowerShellQuote(in)+")").Output()
		require.NoError(t, err, "input %q", in)
		assert.Equal(t, in, string(out), "input %q", in)
		_, err = os.Stat(canary)
		require.True(t, os.IsNotExist(err), "input %q ran a second command", in)
	}
}
