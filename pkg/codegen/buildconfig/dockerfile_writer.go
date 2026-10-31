package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// DockerfileEnvMapping ties a Dockerfile `ENV` variable to the build.yaml key
// holding its value.
type DockerfileEnvMapping struct {
	Env string // Dockerfile variable name, e.g. CATTLE_MACHINE_VERSION
	Key string // build.yaml key holding the value, e.g. machineVersion
}

// DockerfileWriter updates `ENV NAME=value` lines in a Dockerfile with values
// from build.yaml. Every other line, including comments and blank lines, is
// written back untouched.
type DockerfileWriter struct {
	Config     map[string]string      // build.yaml parsed config
	Mappings   []DockerfileEnvMapping // ENV variables to keep in sync
	Dockerfile io.Reader              // Dockerfile content
	Output     io.Writer              // updated Dockerfile
}

// Run reads the Dockerfile, rewrites the mapped ENV lines and writes the result
// to Output. Nothing is written to Output if any mapping cannot be applied.
func (w *DockerfileWriter) Run() error {
	if w.Config == nil {
		return errors.New("nil config")
	}
	if w.Dockerfile == nil {
		return errors.New("nil dockerfile input")
	}
	if w.Output == nil {
		return errors.New("nil output")
	}

	content, err := io.ReadAll(w.Dockerfile)
	if err != nil {
		return fmt.Errorf("failed to read dockerfile: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, m := range w.Mappings {
		value := w.Config[m.Key]
		if value == "" {
			return fmt.Errorf("%q is missing or empty in build config", m.Key)
		}
		prefix := "ENV " + m.Env + "="
		found := false
		for i, line := range lines {
			if strings.HasPrefix(line, prefix) {
				lines[i] = prefix + value
				found = true
			}
		}
		if !found {
			return fmt.Errorf("no %q line found in dockerfile", prefix)
		}
	}

	_, err = io.WriteString(w.Output, strings.Join(lines, "\n"))
	return err
}
