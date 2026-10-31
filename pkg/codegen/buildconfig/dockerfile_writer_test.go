package main_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	main "github.com/rancher/rancher/pkg/codegen/buildconfig"
	"github.com/stretchr/testify/require"
)

func TestDockerfileWriterRun(t *testing.T) {
	t.Parallel()

	mappings := []main.DockerfileEnvMapping{
		{Env: "CATTLE_MACHINE_VERSION", Key: "machineVersion"},
		{Env: "CATTLE_SYSTEM_AGENT_VERSION", Key: "systemAgentVersion"},
	}
	cfg := map[string]string{
		"machineVersion":     "v0.2.0",
		"systemAgentVersion": "v0.3.0",
	}

	input := `FROM scratch
# keep this comment
ENV CATTLE_MACHINE_VERSION=v0.1.0
ENV CATTLE_MACHINE_PROVISION_IMAGE=rancher/machine:${CATTLE_MACHINE_VERSION}

# make sure the version is consistent
ENV CATTLE_SYSTEM_AGENT_VERSION=v0.1.0
ENV CATTLE_SYSTEM_AGENT_VERSION_OTHER=untouched
`
	expected := `FROM scratch
# keep this comment
ENV CATTLE_MACHINE_VERSION=v0.2.0
ENV CATTLE_MACHINE_PROVISION_IMAGE=rancher/machine:${CATTLE_MACHINE_VERSION}

# make sure the version is consistent
ENV CATTLE_SYSTEM_AGENT_VERSION=v0.3.0
ENV CATTLE_SYSTEM_AGENT_VERSION_OTHER=untouched
`

	t.Run("replaces mapped lines and preserves the rest", func(t *testing.T) {
		t.Parallel()
		out := new(bytes.Buffer)
		w := main.DockerfileWriter{Config: cfg, Mappings: mappings, Dockerfile: strings.NewReader(input), Output: out}
		require.NoError(t, w.Run())
		require.Equal(t, expected, out.String())
	})

	t.Run("is a no-op when already in sync", func(t *testing.T) {
		t.Parallel()
		out := new(bytes.Buffer)
		w := main.DockerfileWriter{Config: cfg, Mappings: mappings, Dockerfile: strings.NewReader(expected), Output: out}
		require.NoError(t, w.Run())
		require.Equal(t, expected, out.String())
	})

	t.Run("preserves a missing trailing newline", func(t *testing.T) {
		t.Parallel()
		out := new(bytes.Buffer)
		w := main.DockerfileWriter{
			Config:     map[string]string{"machineVersion": "v0.2.0"},
			Mappings:   mappings[:1],
			Dockerfile: strings.NewReader("ENV CATTLE_MACHINE_VERSION=v0.1.0"),
			Output:     out,
		}
		require.NoError(t, w.Run())
		require.Equal(t, "ENV CATTLE_MACHINE_VERSION=v0.2.0", out.String())
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name string
			w    main.DockerfileWriter
			want string
		}{
			{"nil config", main.DockerfileWriter{Dockerfile: strings.NewReader(input), Output: io.Discard}, "nil config"},
			{"nil input", main.DockerfileWriter{Config: cfg, Output: io.Discard}, "nil dockerfile input"},
			{"nil output", main.DockerfileWriter{Config: cfg, Dockerfile: strings.NewReader(input)}, "nil output"},
			{
				"missing key",
				main.DockerfileWriter{Config: map[string]string{}, Mappings: mappings, Dockerfile: strings.NewReader(input), Output: io.Discard},
				"machineVersion",
			},
			{
				"empty value",
				main.DockerfileWriter{Config: map[string]string{"machineVersion": "", "systemAgentVersion": "v0.3.0"}, Mappings: mappings, Dockerfile: strings.NewReader(input), Output: io.Discard},
				"missing or empty",
			},
			{
				"missing ENV line",
				main.DockerfileWriter{Config: cfg, Mappings: mappings, Dockerfile: strings.NewReader("FROM scratch\n"), Output: io.Discard},
				"ENV CATTLE_MACHINE_VERSION=",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				require.ErrorContains(t, tt.w.Run(), tt.want)
			})
		}
	})

	t.Run("writes nothing on error", func(t *testing.T) {
		t.Parallel()
		out := new(bytes.Buffer)
		w := main.DockerfileWriter{
			Config:     cfg,
			Mappings:   mappings,
			Dockerfile: strings.NewReader("ENV CATTLE_MACHINE_VERSION=v0.1.0\n"),
			Output:     out,
		}
		require.Error(t, w.Run())
		require.Zero(t, out.Len())
	})
}
