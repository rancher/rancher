package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ehazlett/simplelog"
	"github.com/rancher/norman/pkg/dump"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/app"
	"github.com/rancher/rancher/k8s"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	VERSION = "dev"
)

func main() {
	app.RegisterPasswordResetCommand()
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

	var config app.Config

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
			Name:        "debug",
			Usage:       "Enable debug logs",
			Destination: &config.Debug,
		},
		cli.StringFlag{
			Name:        "add-local",
			Usage:       "Add local cluster (true, false, auto)",
			Value:       "auto",
			Destination: &config.AddLocal,
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
			Usage:       "Mode to run or access k8s API server for management API (embedded, external, auto)",
			Value:       "auto",
			Destination: &config.K8sMode,
		},
		cli.StringFlag{
			Name:  "log-format",
			Usage: "Log formatter used (json, text, simple)",
			Value: "simple",
		},
		cli.StringSliceFlag{
			Name:  "acme-domain",
			Usage: "Domain to register with LetsEncrypt",
		},
	}

	app.Action = func(c *cli.Context) error {
		// enable profiler
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()

		config.ACMEDomains = c.GlobalStringSlice("acme-domain")
		initLogs(c, config)
		return run(config)
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func initLogs(c *cli.Context, cfg app.Config) {
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	switch c.String("log-format") {
	case "simple":
		logrus.SetFormatter(&simplelog.StandardFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	logserver.StartServerWithDefaults()
}

func run(cfg app.Config) error {
	dump.GoroutineDumpOn(syscall.SIGUSR1, syscall.SIGILL)
	ctx := signal.SigTermCancelContext(context.Background())

	embedded, ctx, kubeConfig, err := k8s.GetConfig(ctx, cfg.K8sMode, cfg.KubeConfig)
	if err != nil {
		return err
	}
	cfg.Embedded = embedded

	os.Unsetenv("KUBECONFIG")
	kubeConfig.Timeout = 30 * time.Second
	return app.Run(ctx, *kubeConfig, &cfg)
}
