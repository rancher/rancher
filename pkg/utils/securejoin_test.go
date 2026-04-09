package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureJoin(t *testing.T) {
	// base/
	//   subdir/
	//     file.txt
	// outside/
	//   secret.txt
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "subdir", "file.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "valid file", path: "subdir/file.txt", want: filepath.Join(base, "subdir/file.txt")},
		{name: "base itself", path: ".", want: filepath.Join(base, ".")},
		{name: "dot-dot traversal", path: "../../etc/passwd", wantErr: true},
		{name: "absolute path", path: "/etc/passwd", wantErr: true},
		{name: "non-existent", path: "doesnotexist/file.txt", want: filepath.Join(base, "doesnotexist/file.txt")},
		{name: "symlink escaping base", path: "evil/secret.txt", wantErr: true},
		{name: "symlink within base", path: "link/file.txt", want: filepath.Join(base, "link/file.txt")},
	}

	// Set up symlinks once, before the test loop.
	createSymlink(t, base, outside, "evil")
	createSymlink(t, base, filepath.Join(base, "subdir"), "link")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SecureJoin(base, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func createSymlink(t *testing.T, base, target, name string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(base, name)); err != nil {
		t.Fatal(err)
	}
}

func resolveSymlink(t *testing.T, paths ...string) string {
	p, err := filepath.EvalSymlinks(filepath.Join(paths...))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
