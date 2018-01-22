package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/app"
	"github.com/rancher/rancher/k8s"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	VERSION = "dev"
)

func main() {
	if reexec.Init() {
		return
	}

	os.Unsetenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AGENT_PID")

	if dir, err := os.Getwd(); err == nil {
		dmPath := filepath.Join(dir, "management-state", "bin")
		os.MkdirAll(dmPath, 0700)
		newPath := fmt.Sprintf("%s%s%s", dmPath, string(os.PathListSeparator), os.Getenv("PATH"))

		os.Setenv("PATH", newPath)
	}

	config := app.Config{
		InteralListenPort: 8081,
	}

	app := cli.NewApp()
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "Kube config for accessing k8s cluster",
			EnvVar:      "KUBECONFIG",
			Destination: &config.KubeConfig,
		},
		cli.BoolFlag{
			Name:        "add-local",
			Usage:       "Add local cluster to management server",
			Destination: &config.AddLocal,
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Enable debug logs",
			Destination: &config.Debug,
		},
		cli.IntFlag{
			Name:        "http-listen-port",
			Usage:       "HTTP listen port",
			Value:       8080,
			Destination: &config.HTTPListenPort,
		},
		cli.IntFlag{
			Name:        "https-listen-port",
			Usage:       "HTTPS listen port",
			Value:       8443,
			Destination: &config.HTTPSListenPort,
		},
		cli.StringFlag{
			Name:        "k8s-mode",
			Usage:       "Mode to run or access k8s API server for management API (internal, exec)",
			Value:       "internal",
			Destination: &config.K8sMode,
		},
		cli.StringSliceFlag{
			Name:  "acme-domain",
			Usage: "Domain to register with LetsEncrypt",
		},
		cli.BoolFlag{
			Name:        "http-only",
			Usage:       "Disable HTTPS",
			Destination: &config.HTTPOnly,
		},
	}

	app.Action = func(c *cli.Context) error {
		config.ACMEDomains = c.GlobalStringSlice("acme-domains")
		return run(config)
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func run(cfg app.Config) error {
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	signal.GoroutineDumpOn(syscall.SIGUSR1, syscall.SIGILL)
	ctx := signal.SigTermCancelContext(context.Background())

	os.Args = []string{os.Args[0]}
	kubeConfig, local, err := k8s.GetConfig(ctx, cfg.K8sMode, cfg.AddLocal, cfg.KubeConfig, cfg.InteralListenPort)
	if err != nil {
		return err
	}

	cfg.AddLocal = local

	return app.Run(ctx, *kubeConfig, &cfg)
}
