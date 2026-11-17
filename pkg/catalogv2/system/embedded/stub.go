//go:build !embedded_systemcharts

package embedded

import "io"

// Has returns false when embedded charts are not compiled into the binary.
func Has(name, version string) bool {
	return false
}

// Get returns nil, false when embedded charts are not compiled into the binary.
func Get(name, version string) (io.ReadCloser, bool) {
	return nil, false
}
