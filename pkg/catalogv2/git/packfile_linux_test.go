package git

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Count packfile opens instead of asserting a machine-dependent checkout timeout.
// Each close is watched too, so successive opens cannot be coalesced by inotify.
func TestHeadReusesPackfile(t *testing.T) {
	f := newWorkspace(t)
	for i := 0; i < 128; i++ {
		name := fmt.Sprintf("chart-%03d.yaml", i)
		require.NoError(t, os.WriteFile(filepath.Join(f.mockRemote, name), []byte(name), 0o644))
	}
	require.NoError(t, gitCmd("-C", f.mockRemote, "add", "."))
	want := f.mockCommitRemote(t, "packed")
	require.NoError(t, gitCmd("clone", "--no-checkout", "--depth", "1", "--", "file://"+f.mockRemote, f.dir))
	packs, err := filepath.Glob(filepath.Join(f.dir, ".git", "objects", "pack", "*.pack"))
	require.NoError(t, err)
	require.Len(t, packs, 1)
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, unix.Close(fd)) })
	_, err = unix.InotifyAddWatch(fd, packs[0], unix.IN_OPEN|unix.IN_CLOSE_NOWRITE)
	require.NoError(t, err)

	start := time.Now()
	got, err := Head(nil, bundledNamespace, bundledName, bundledURL, "main", false, nil)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Equal(t, want, got)
	opens, closes := 0, 0
	buf := make([]byte, 64*1024)
	for {
		n, err := unix.Read(fd, buf)
		if err == unix.EAGAIN {
			break
		}
		require.NoError(t, err)
		for offset := 0; offset < n; {
			mask := binary.NativeEndian.Uint32(buf[offset+4:])
			require.Zero(t, mask&unix.IN_Q_OVERFLOW, "inotify queue overflowed")
			if mask&unix.IN_OPEN != 0 {
				opens++
			}
			if mask&unix.IN_CLOSE_NOWRITE != 0 {
				closes++
			}
			offset += unix.SizeofInotifyEvent + int(binary.NativeEndian.Uint32(buf[offset+12:]))
		}
	}
	t.Logf("checkout of 129 packed files: %s, packfile opens=%d closes=%d", elapsed, opens, closes)
	require.Positive(t, opens, "the checkout must read the packfile")
	require.LessOrEqual(t, opens, 2, "reuse the packfile during checkout instead of reopening it for each object")
	require.Equal(t, opens, closes, "release packfile descriptors when checkout completes")
	for i := 0; i < 128; i++ {
		name := fmt.Sprintf("chart-%03d.yaml", i)
		content, err := os.ReadFile(filepath.Join(f.dir, name))
		require.NoError(t, err)
		require.Equal(t, name, string(content))
	}
}
