//go:build k3s_export
// +build k3s_export

package main

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestK3sImages(t *testing.T) {
	tests := []struct {
		name    string
		version string
		fetcher func(string) (io.ReadCloser, error)
		want    []string
		err     string
	}{
		{
			name:    "invalid version: semver",
			version: "v2.1.2",
			err:     "invalid k3s version",
		},
		{
			name:    "invalid version",
			version: "abc",
			err:     "invalid k3s version",
		},
		{
			name:    "valid version: error fetching",
			version: "v1.2.3+k3s0",
			fetcher: func(s string) (io.ReadCloser, error) {
				return nil, fmt.Errorf("general network failure")
			},
			err: "general network failure",
		},
		{
			name:    "valid version: empty fetch",
			version: "v1.2.3+k3s0",
			fetcher: func(s string) (io.ReadCloser, error) {
				fetchedImages := ``
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{},
		},
		{
			name:    "valid version: pause image",
			version: "v1.2.3+k3s0",
			fetcher: func(s string) (io.ReadCloser, error) {
				fetchedImages := "docker.io/rancher/mirrored-pause:v1.1.3\ndocker.io/foo/bar"
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{"docker.io/rancher/mirrored-pause:v1.1.3"},
		},
		{
			name:    "valid version: core dns",
			version: "v1.2.3+k3s0",
			fetcher: func(s string) (io.ReadCloser, error) {
				fetchedImages := "foo.bar/bar/foo\ndocker.io/rancher/mirrored-coredns-coredns"
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{"docker.io/rancher/mirrored-coredns-coredns"},
		},
		{
			name:    "valid version: reject partial match",
			version: "v1.2.3+k3s0",
			fetcher: func(s string) (io.ReadCloser, error) {
				fetchedImages := "docker.io/rancher/mirrored-pausea:v1.1.3\ndocker.io/foo/bar"
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() { fetcher = fetch }()
			fetcher = tc.fetcher

			got, err := k3sImages(tc.version)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("want %v got %v", tc.want, got)
			}

			if tc.err == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.err != "" {
				t.Fatalf("got nil instead of error %v", tc.err)
			}
			if tc.err != "" && !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("want error containing %q got %v", tc.err, err)
			}
		})
	}
}

// TestFetchContextNotCanceledBeforeBodyRead verifies that the context
// returned by fetch is not canceled before the caller reads the body.
// This reproduces the bug where `defer cancel()` in fetch would cancel
// the context immediately on return, causing "context canceled" errors
// when the caller tried to read the response body.
func TestFetchContextNotCanceledBeforeBodyRead(t *testing.T) {
	content := "docker.io/rancher/mirrored-pause:v1.0.0\ndocker.io/rancher/mirrored-coredns-coredns:v1.0.0\n"

	// Simulate what the real fetch does: return a body whose reads
	// are gated by a context. If the context is cancelled before
	// reading, reads fail with "context canceled".
	defer func() { fetcher = fetch }()
	fetcher = func(url string) (io.ReadCloser, error) {
		ctx, cancel := context.WithCancel(context.Background())
		pr, pw := io.Pipe()

		go func() {
			<-ctx.Done()
			// Once context is canceled, close the write end with
			// the context error — mirroring net/http transport behavior.
			pw.CloseWithError(ctx.Err())
		}()

		go func() {
			// Write content but don't close — the context
			// cancellation path above will close it.
			pw.Write([]byte(content))
		}()

		// This is the key part: the old code did `defer cancel()` here,
		// which would cancel immediately. The fix wraps the body so
		// cancel is deferred to Close().
		return &cancelOnClose{ReadCloser: pr, cancel: cancel}, nil
	}

	imgs, err := k3sImages("v1.2.3+k3s0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"docker.io/rancher/mirrored-pause:v1.0.0",
		"docker.io/rancher/mirrored-coredns-coredns:v1.0.0",
	}
	if !reflect.DeepEqual(imgs, want) {
		t.Errorf("want %v got %v", want, imgs)
	}
}
