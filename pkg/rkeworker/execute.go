package rkeworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/rkecerts"
)

func ExecutePlan(ctx context.Context, nodeConfig *NodeConfig) error {
	if nodeConfig.Certs != "" {
		bundle, err := rkecerts.Unmarshal(nodeConfig.Certs)
		if err != nil {
			return err
		}

		if err := bundle.Explode(); err != nil {
			return err
		}
	}

	f := fileWriter{}
	for _, file := range nodeConfig.Files {
		f.write(file.Name, file.Contents)
	}

	for name, process := range nodeConfig.Processes {
		if strings.Contains(name, "sidekick") || strings.Contains(name, "share-mnt") {
			if err := runProcess(ctx, name, process, false); err != nil {
				return err
			}
		}
	}

	for name, process := range nodeConfig.Processes {
		if !strings.Contains(name, "sidekick") {
			if err := runProcess(ctx, name, process, true); err != nil {
				return err
			}
		}
	}

	return nil
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
