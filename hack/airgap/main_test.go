//go:build k3s_export
// +build k3s_export

package main

import (
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
