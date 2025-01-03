package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	"github.com/ehazlett/simplelog"
	_ "github.com/rancher/norman/controller"
	"github.com/rancher/norman/pkg/kwrapper/k8s"
	"github.com/rancher/rancher/pkg/data/management"
	"github.com/rancher/rancher/pkg/logserver"
	"github.com/rancher/rancher/pkg/rancher"
	"github.com/rancher/rancher/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	profileAddress = "localhost:6060"
	kubeConfig     string
)

func main() {
	management.RegisterPasswordResetCommand()
	management.RegisterEnsureDefaultAdminCommand()
	if reexec.Init() {
		return
	}

	os.Unsetenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AGENT_PID")

	if dm := os.Getenv("CATTLE_DEV_MODE"); dm != "" {
		if dir, err := os.Getwd(); err == nil {
			dmPath := filepath.Join(dir, "management-state", "bin")
			os.MkdirAll(dmPath, 0700)
		}
	}

	var config rancher.Options

	app := cli.NewApp()
	app.Version = version.FriendlyVersion()
	app.Usage = "Complete container management platform"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "Kube config for accessing k8s cluster",
			EnvVar:      "KUBECONFIG",
			Destination: &kubeConfig,
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Enable debug logs",
			Destination: &config.Debug,
		},
		cli.BoolFlag{
			Name:        "trace",
			Usage:       "Enable trace logs",
			Destination: &config.Trace,
		},
		cli.StringFlag{
			Name:        "add-local",
			Usage:       "Add local cluster (true, false)",
			Value:       "true",
			Destination: &config.AddLocal,
			Hidden:      true,
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
			Name:   "acme-domain",
			EnvVar: "ACME_DOMAIN",
			Usage:  "Domain to register with LetsEncrypt",
			Value:  &config.ACMEDomains,
		},
		cli.BoolFlag{
			Name:        "no-cacerts",
			Usage:       "Skip CA certs population in settings when set to true",
			Destination: &config.NoCACerts,
		},
		cli.StringFlag{
			Name:        "audit-log-path",
			EnvVar:      "AUDIT_LOG_PATH",
			Value:       "/var/log/auditlog/rancher-api-audit.log",
			Usage:       "Log path for Rancher Server API. Default path is /var/log/auditlog/rancher-api-audit.log",
			Destination: &config.AuditLogPath,
		},
		cli.IntFlag{
			Name:        "audit-log-maxage",
			Value:       10,
			EnvVar:      "AUDIT_LOG_MAXAGE",
			Usage:       "Defined the maximum number of days to retain old audit log files",
			Destination: &config.AuditLogMaxage,
		},
		cli.IntFlag{
			Name:        "audit-log-maxbackup",
			Value:       10,
			EnvVar:      "AUDIT_LOG_MAXBACKUP",
			Usage:       "Defines the maximum number of audit log files to retain",
			Destination: &config.AuditLogMaxbackup,
		},
		cli.IntFlag{
			Name:        "audit-log-maxsize",
			Value:       100,
			EnvVar:      "AUDIT_LOG_MAXSIZE",
			Usage:       "Defines the maximum size in megabytes of the audit log file before it gets rotated, default size is 100M",
			Destination: &config.AuditLogMaxsize,
		},
		cli.IntFlag{
			Name:        "audit-level",
			Value:       0,
			EnvVar:      "AUDIT_LEVEL",
			Usage:       "Audit log level: 0 - disable audit log, 1 - log event metadata, 2 - log event metadata and request body, 3 - log event metadata, request body and response body",
			Destination: &config.AuditLogLevel,
		},
		cli.BoolFlag{
			Name:        "enable-audit-log",
			Usage:       "enable the rancher audit log system",
			EnvVar:      "AUDIT_LOG_ENABLED",
			Destination: &config.AuditLogEnabled,
		},
		cli.StringFlag{
			Name:        "profile-listen-address",
			Value:       "127.0.0.1:6060",
			Usage:       "Address to listen on for profiling",
			Destination: &profileAddress,
		},
		cli.StringFlag{
			Name:        "features",
			EnvVar:      "CATTLE_FEATURES",
			Value:       "",
			Usage:       "Declare specific feature values on start up. Example: \"kontainer-driver=true\" - kontainer driver feature will be enabled despite false default value",
			Destination: &config.Features,
		},
	}

	app.Action = func(c *cli.Context) error {
		// enable profiler
		if profileAddress != "" {
			go func() {
				log.Println(http.ListenAndServe(profileAddress, nil))
			}()
		}
		initLogs(c, config)
		return run(c, config)
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func initLogs(c *cli.Context, cfg rancher.Options) {
	switch c.String("log-format") {
	case "simple":
		logrus.SetFormatter(&simplelog.StandardFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debugf("Loglevel set to [%v]", logrus.DebugLevel)
	}
	if cfg.Trace {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.Tracef("Loglevel set to [%v]", logrus.TraceLevel)
	}

	logserver.StartServerWithDefaults()
}

func run(cli *cli.Context, cfg rancher.Options) error {
	logrus.Infof("Rancher version %s is starting", version.FriendlyVersion())
	logrus.Infof("Rancher arguments %+v", cfg)
	ctx := signals.SetupSignalContext()

	if cfg.AddLocal != "true" && cfg.AddLocal != "auto" {
		logrus.Fatal("add-local flag must be set to 'true', see Rancher 2.5.0 release notes for more information")
	}

	embedded, clientConfig, err := k8s.GetConfig(ctx, cfg.K8sMode, kubeConfig)
	if err != nil {
		return err
	}
	cfg.Embedded = embedded

	os.Unsetenv("KUBECONFIG")

	server, err := rancher.New(ctx, clientConfig, &cfg)
	if err != nil {
		return err
	}

	return server.ListenAndServe(ctx)
}
