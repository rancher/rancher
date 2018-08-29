package rkeworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	rancherDir = `C:\etc\rancher`

	ErrHyperKubePSScriptCrash      = errors.New("crashed to execute PowerShell process")
	ErrHyperKubePSScriptAgentRetry = errors.New("PowerShell process is going to retry")
)

func ExecutePlan(ctx context.Context, nodeConfig *NodeConfig, writeCertOnly bool) error {
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
	if writeCertOnly {
		return nil
	}

	// pre run docker
	for name, process := range nodeConfig.Processes {
		if strings.HasPrefix(name, "pre-run-docker") {
			if err := runProcess(ctx, process.Name, process, true); err != nil {
				return err
			}
		}
	}

	// post run docker
	for name, process := range nodeConfig.Processes {
		if strings.HasPrefix(name, "post-run-docker") {
			if err := runProcess(ctx, process.Name, process, true); err != nil {
				return err
			}
		}
	}

	// run hyperkube.ps1
	if hyperkubeProcess, ok := nodeConfig.Processes["hyperkube"]; !ok {
		return errors.New("can't execute hyperkube.ps1 without any commands")
	} else {
		scriptPath := rancherDir + `\hyperkube.ps1`
		if _, err := os.Stat(scriptPath); err != nil {
			return fmt.Errorf(`can't find %s'`, scriptPath)
		}

		args := make([]string, 0, len(hyperkubeProcess.Args)+1)
		args = append(args, hyperkubeProcess.Args...)
		if connected := ctx.Value("isConnected").(bool); !connected {
			args = append(args, "-Force")
		}

		return executePowerShell(ctx, scriptPath, args...)
	}
}

func Stop(ctx context.Context) error {
	scriptPath := rancherDir + `\stop.ps1`
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf(`can't find %s while stopping`, scriptPath)
	}

	return executePowerShell(ctx, scriptPath)
}

func Remove(ctx context.Context, callback func() error) error {
	scriptPath := rancherDir + `\remove.ps1`
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf(`can't find %s while removing`, scriptPath)
	}

	if err := executePowerShell(ctx, scriptPath); err != nil {
		return err
	}

	if callback != nil {
		return callback()
	}

	return nil
}

func executePowerShell(rootCtx context.Context, scriptPath string, args ...string) error {
	g, ctx := errgroup.WithContext(rootCtx)

	psArgs := make([]string, 0, 32)
	psArgs = append(psArgs, "-Sta", "-NoLogo", "-NonInteractive", "-File", scriptPath)
	psArgs = append(psArgs, args...)
	command := exec.CommandContext(ctx, "powershell.exe", psArgs...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf(`failed to open stdout from %s execution, %v`, scriptPath, err)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf(`failed to open stderr from %s execution, %v`, scriptPath, err)
	}

	g.Go(func() error {
		defer stdout.Close()

		bs := make([]byte, 1<<20)
		for {
			readSize, err := stdout.Read(bs)
			if readSize > 0 {
				logrus.Info(string(bs[:readSize]))
			}

			if err != nil {
				if io.EOF != err && io.ErrClosedPipe != err {
					return fmt.Errorf(`failed to read stdout from %s execution, %v`, scriptPath, err)
				}
				break
			}
		}

		return nil
	})

	g.Go(func() error {
		defer stderr.Close()

		bs := make([]byte, 1<<20)
		for {
			readSize, err := stderr.Read(bs)
			if readSize > 0 {
				logrus.Warn(string(bs[:readSize]))
			}

			if err != nil {
				if io.EOF != err && io.ErrClosedPipe != err {
					return fmt.Errorf(`failed to read stderr from %s execution, %v`, scriptPath, err)
				}
				break
			}
		}

		return nil
	})

	g.Go(func() error {
		if err := command.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				waitStatus := exitError.Sys().(syscall.WaitStatus)
				return powerShellExistStatusError(waitStatus.ExitStatus())
			}

			return fmt.Errorf(`can't execute %s process, %v`, scriptPath, err)
		}
		// waitStatus := command.ProcessState.Sys().(syscall.WaitStatus)
		// if waitStatus.ExitStatus() != 0 {
		// 	return fmt.Errorf(`%s process is failed, exit by %d`, scriptPath, waitStatus.ExitStatus())
		// }
		return nil
	})

	return g.Wait()
}

func powerShellExistStatusError(existStatus int) error {
	switch existStatus {
	case 1:
		return ErrHyperKubePSScriptCrash
	case 2:
		return ErrHyperKubePSScriptAgentRetry
	}

	return fmt.Errorf("unknown exist code: %d", existStatus)
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
