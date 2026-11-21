//go:build embedded_systemcharts

package embedded

import (
	"embed"
	"io"
	"strings"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
)

//go:embed *.tgz
var content embed.FS

type chartKey struct {
	name    string
	version string
}

var index = map[chartKey]string{}

func init() {
	// shortcut: loader.LoadArchive decompresses full chart in init; switch to partial tar walk if startup time with many large charts becomes noticeable.
	entries, err := content.ReadDir(".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tgz") {
			continue
		}
		f, err := content.Open(entry.Name())
		if err != nil {
			continue
		}
		ch, err := loader.LoadArchive(f)
		_ = f.Close()
		if err != nil {
			continue
		}
		acc, err := chart.NewAccessor(ch)
		if err != nil {
			continue
		}
		// there is no Version() method in the accessor interface
		version, ok := acc.MetadataAsMap()["Version"].(string)
		if !ok || version == "" || acc.Name() == "" {
			continue
		}

		index[chartKey{name: acc.Name(), version: version}] = entry.Name()
	}
}

// Has reports whether there is an embedded chart matching the given name and version.
func Has(name, version string) bool {
	_, ok := index[chartKey{name: name, version: version}]
	return ok
}

// Get returns a reader for the embedded chart matching the given name and version, or false if not found.
func Get(name, version string) (io.ReadCloser, bool) {
	filename, ok := index[chartKey{name: name, version: version}]
	if !ok {
		return nil, false
	}
	f, err := content.Open(filename)
	if err != nil {
		return nil, false
	}
	return f, true
}
