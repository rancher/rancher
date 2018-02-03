package service

import (
	"context"
	"os"
	"os/exec"

	"time"

	"fmt"
	"strings"

	"github.com/coreos/etcd/etcdmain"
	"github.com/docker/docker/pkg/reexec"
	"github.com/rancher/rancher/k8s/apiserver"
	"github.com/sirupsen/logrus"
)

var (
	funcs = map[string]func(){
		"etcd":       etcdmain.Main,
		"api-server": apiserver.APIServer,
	}
)

func init() {
	if os.Getenv("ETCD_ARGS") == "" {
		os.Setenv("ETCD_ARGS", "--advertise-client-urls http://localhost:2382 --listen-peer-urls http://localhost:2382 --listen-client-urls http://localhost:2381")
	}

	for name, f := range funcs {
		reexec.Register(name, f)
	}
}

func Service(ctx context.Context, internal bool, name string) {
	newProcess(internal, name).run(ctx)
}

func newProcess(internal bool, name string) process {
	if internal {
		return processFunc{name: name}
	}
	return processExec{name: name}
}

type process interface {
	run(ctx context.Context)
}

type processExec struct {
	name string
}

type processFunc struct {
	name string
}

func (p processExec) run(ctx context.Context) {
	run := true

	args := []string{p.name}
	argStr := os.Getenv(strings.ToUpper(fmt.Sprintf("%s_ARGS", p.name)))
	if argStr != "" {
		args = append(args, strings.Split(argStr, " ")...)
	}

	var cmd *exec.Cmd

	go func() {
		<-ctx.Done()
		run = false
		cmd.Process.Kill()
	}()

	for run {
		cmd = &exec.Cmd{
			Path:   reexec.Self(),
			Args:   args,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Stdin:  os.Stdin,
		}
		err := cmd.Run()
		logrus.Errorf("failed to run %s: %v", p.name, err)
		time.Sleep(2 * time.Second)
	}
}

func (p processFunc) run(ctx context.Context) {
	args := []string{p.name}
	argStr := os.Getenv(strings.ToUpper(fmt.Sprintf("%s_ARGS", p.name)))
	if argStr != "" {
		args = append(args, strings.Split(argStr, " ")...)
		os.Args = args
	}

	funcs[p.name]()
}
