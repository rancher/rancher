package main

import "io"

// ToolsSystemImagesWriter generates the v3 ToolsSystemImages variable from
// the values in build.yaml.
type ToolsSystemImagesWriter struct {
	Config map[string]string // build.yaml parsed config
	Output io.Writer         // generated tools_system_images.go
}

// Run renders the Go source for ToolsSystemImages and writes it to Output.
func (w *ToolsSystemImagesWriter) Run() error {
	// TODO: not implemented yet, writes nothing.
	return nil
}
