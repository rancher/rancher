package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
)

func TestK3sImages(t *testing.T) {
	tests := []struct {
		name    string
		version string
		fetcher func(context.Context, string) (io.ReadCloser, error)
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
			fetcher: func(ctx context.Context, s string) (io.ReadCloser, error) {
				return nil, fmt.Errorf("general network failure")
			},
			err: "general network failure",
		},
		{
			name:    "valid version: empty fetch",
			version: "v1.2.3+k3s0",
			fetcher: func(ctx context.Context, s string) (io.ReadCloser, error) {
				fetchedImages := ``
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{},
		},
		{
			name:    "valid version: pause image",
			version: "v1.2.3+k3s0",
			fetcher: func(ctx context.Context, s string) (io.ReadCloser, error) {
				fetchedImages := "docker.io/rancher/mirrored-pause:v1.1.3\ndocker.io/foo/bar"
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{"docker.io/rancher/mirrored-pause:v1.1.3"},
		},
		{
			name:    "valid version: core dns",
			version: "v1.2.3+k3s0",
			fetcher: func(ctx context.Context, s string) (io.ReadCloser, error) {
				fetchedImages := "foo.bar/bar/foo\ndocker.io/rancher/mirrored-coredns-coredns"
				return io.NopCloser(strings.NewReader(fetchedImages)), nil
			},
			want: []string{"docker.io/rancher/mirrored-coredns-coredns"},
		},
		{
			name:    "valid version: reject partial match",
			version: "v1.2.3+k3s0",
			fetcher: func(ctx context.Context, s string) (io.ReadCloser, error) {
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

			got, err := k3sImages(t.Context(), tc.version)
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

func TestSave(t *testing.T) {
	defer func() {
		fetcher = fetch
		imageFetcher = fetchImage
	}()

	fetcher = func(ctx context.Context, s string) (io.ReadCloser, error) {
		images := "docker.io/rancher/mirrored-pause:v1.1.3\ndocker.io/rancher/mirrored-coredns-coredns:v1.1.3\n"
		return io.NopCloser(strings.NewReader(images)), nil
	}

	fetchedRefs := make(map[string]bool)
	imageFetcher = func(ctx context.Context, ref name.Reference, platform v1.Platform) (v1.Image, error) {
		fetchedRefs[ref.Name()] = true
		if platform.OS != "linux" || platform.Architecture != "arm64" {
			return nil, fmt.Errorf("unexpected platform: %+v", platform)
		}
		return empty.Image, nil
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "sub", "k3s-airgap-images.tar")

	err := Save(t.Context(), "v1.2.3+k3s0", "arm64", outPath)
	if err != nil {
		t.Fatalf("unexpected error from Save: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output tarball not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("output tarball is empty")
	}

	if !fetchedRefs["index.docker.io/rancher/mirrored-pause:v1.1.3"] ||
		!fetchedRefs["index.docker.io/rancher/mirrored-coredns-coredns:v1.1.3"] {
		t.Errorf("expected images were not fetched: %+v", fetchedRefs)
	}
}
