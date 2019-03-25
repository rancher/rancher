package rkeworker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var ErrHyperKubePSScriptAgentRetry = errors.New("PowerShell process is going to retry")

func doExecutePlan(ctx context.Context, nodeConfig *NodeConfig, _ bool) error {
	// run as docker
	for name, process := range nodeConfig.Processes {
		if strings.HasPrefix(name, "run-container-") {
			if err := runProcess(ctx, process.Name, process, true, false); err != nil {
				return err
			}
		}
	}

	// run as powershell
	for name, process := range nodeConfig.Processes {
		if strings.HasPrefix(name, "run-script-") {
			if err := executePowerShell(ctx, process.Name, process.Args...); err != nil {
				return err
			}
		}
	}

	return nil
}

func Stop(ctx context.Context) error {
	return executePowerShell(ctx, "stop")
}

func Remove(ctx context.Context) error {
	return executePowerShell(ctx, "remove")
}

func executePowerShell(rootCtx context.Context, scriptName string, args ...string) error {
	scriptPath, err := getPowerShellScript(scriptName)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(rootCtx)

	psArgs := make([]string, 0, 32)
	psArgs = append(psArgs, "-Sta", "-NoLogo", "-NonInteractive", "-File", scriptPath)
	psArgs = append(psArgs, args...)
	command := exec.CommandContext(ctx, "powershell.exe", psArgs...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf(`failed to open stdout from %s powershell script execution, %v`, scriptPath, err)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf(`failed to open stderr from %s powershell script execution, %v`, scriptPath, err)
	}

	g.Go(func() error {
		defer stdout.Close()

		bs := make([]byte, 1<<20)
		for {
			readSize, err := stdout.Read(bs)
			if readSize > 0 {
				logrus.Infof("[powershell] %s", string(bs[:readSize]))
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
				logrus.Warnf("[powershell] %s", string(bs[:readSize]))
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
				switch exitError.ExitCode() {
				case 0:
					return nil
				case 2:
					return ErrHyperKubePSScriptAgentRetry
				}
			}

			return fmt.Errorf(`failed to execute %s powershell script, %v`, scriptPath, err)
		}
		return nil
	})

	return g.Wait()
}

func getPowerShellScript(scriptName string) (string, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", errors.Wrapf(err, "cannot locate the execution dir from %s", os.Args[0])
	}

	scriptPath := fmt.Sprintf(`%s\%s.ps1`, dir, scriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		return "", errors.Wrapf(err, `cannot find %s'`, scriptPath)
	}

	return scriptPath, nil
}
