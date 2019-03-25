package rkeworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/rkecerts"
)

func ExecutePlan(ctx context.Context, nodeConfig *NodeConfig, writeCertOnly bool) error {
	var bundleChanged bool
	if nodeConfig.Certs != "" {
		bundle, err := rkecerts.Unmarshal(nodeConfig.Certs)
		if err != nil {
			return err
		}
		bundleChanged = bundle.Changed()
		if err := bundle.Explode(); err != nil {
			return err
		}
	}

	f := fileWriter{}
	for _, file := range nodeConfig.Files {
		f.write(file.Name, file.Contents)
	}
	if writeCertOnly {
		return nil
	}

	return doExecutePlan(ctx, nodeConfig, bundleChanged)
}

type fileWriter struct {
	errs []error
}

func (f *fileWriter) write(path string, base64Content string) {
	if path == "" {
		return
	}

	content, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		f.errs = append(f.errs, err)
		return
	}

	existing, err := ioutil.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		f.errs = append(f.errs, err)
	}
	if err := ioutil.WriteFile(path, content, 0600); err != nil {
		f.errs = append(f.errs, err)
	}
}

func (f *fileWriter) err() error {
	return types.NewErrors(f.errs...)
}
