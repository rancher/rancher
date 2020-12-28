package chart

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Archive struct {
	io.ReadCloser

	Metadata *chart.Metadata
	Path     string
	temp     bool
}

func (a *Archive) Open() (io.ReadCloser, error) {
	var err error
	a.ReadCloser, err = os.Open(a.Path)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Archive) Close() error {
	var (
		err error
	)
	if a.ReadCloser != nil {
		err = a.ReadCloser.Close()
	}
	if a.temp {
		os.RemoveAll(filepath.Dir(a.Path))
	}

	return err
}

func LoadArchive(path string) (*Archive, bool, error) {
	if s, err := os.Stat(path); err == nil && !s.IsDir() {
		return &Archive{
			Path: path,
		}, true, nil
	}

	if ok, err := chartutil.IsChartDir(path); !ok || err != nil {
		return nil, false, nil
	}

	c, err := loader.LoadDir(path)
	if err != nil {
		return nil, false, err
	}

	tempDir, err := ioutil.TempDir("", "chart-archive-")
	if err != nil {
		return nil, false, fmt.Errorf("creating archive for %s: %w", path, err)
	}

	file, err := chartutil.Save(c, tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, false, err
	}

	return &Archive{
		Metadata: c.Metadata,
		Path:     file,
		temp:     true,
	}, true, nil
}
