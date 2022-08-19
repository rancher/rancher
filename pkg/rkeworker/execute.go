package rkeworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/sirupsen/logrus"
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
		logrus.Debugf("writing file %s", file.Name)
		f.write(file.Name, file.Contents)
	}
	if writeCertOnly {
		logrus.Debug("writing certificates only, no need to continue executing plan")
		return nil
	}

	for name, process := range nodeConfig.Processes {
		if strings.Contains(name, "sidekick") || strings.Contains(name, "share-mnt") {
			// windows dockerfile VOLUME declaration must to satisfy one of them:
			// 	- a non-existing or empty directory
			//  - a drive other than C:
			// so we could use a script to **start** the container to put expected resources into the "shared" directory,
			// like the action of `/usr/bin/sidecar.ps1` for windows rke-tools container
			if err := runProcess(ctx, name, process, runtime.GOOS == "windows", false); err != nil {
				return err
			}
		}
	}

	for name, process := range nodeConfig.Processes {
		if !strings.Contains(name, "sidekick") {
			if err := runProcess(ctx, name, process, true, bundleChanged); err != nil {
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

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		f.errs = append(f.errs, err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		f.errs = append(f.errs, err)
	}
}

func (f *fileWriter) err() error {
	return types.NewErrors(f.errs...)
}
