package main

import "io"

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
// to Output.
func (w *DockerfileWriter) Run() error {
	// TODO: not implemented yet, writes nothing.
	return nil
}
